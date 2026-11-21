package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"veyrogram/api/internal/platform/authn"
	"veyrogram/api/internal/platform/denylist"
	"veyrogram/api/internal/platform/imgw"

	"veyrogram/api/internal/identity/adapters"
	"veyrogram/api/internal/identity/app"
	"veyrogram/api/internal/identity/domain"
	identityhttp "veyrogram/api/internal/identity/http"
	"veyrogram/api/internal/identity/ports"
	"veyrogram/api/internal/platform/clientip"
	"veyrogram/api/internal/platform/config"
	"veyrogram/api/internal/platform/httpx"
	"veyrogram/api/internal/platform/metrics"
	"veyrogram/api/internal/platform/outbox"
	"veyrogram/api/internal/platform/token"
)

// identityMux wires the identity module (feature flag auth.sms_login) and returns a mux serving /v1/auth/*.
func identityMux(cfg config.Config, pool *sql.DB, pm *metrics.Platform, log *slog.Logger, im *imgw.Client) (*http.ServeMux, func(http.Handler) http.Handler, error) {
	ring, err := token.ParseRing(cfg.TokenKeys)
	if err != nil {
		return nil, nil, fmt.Errorf("VG_TOKEN_KEYS: %w", err)
	}
	trusted, err := clientip.ParseTrusted(cfg.TrustedProxies)
	if err != nil {
		return nil, nil, fmt.Errorf("VG_TRUSTED_PROXIES: %w", err)
	}
	var sms ports.SMSSender
	switch cfg.SMSProvider {
	case "http":
		sms = &adapters.HTTPSender{URL: cfg.SMSHTTPURL, Token: cfg.SMSHTTPToken}
	case "smsbao":
		sms = &adapters.SMSBao{User: cfg.SMSBaoUser, Key: cfg.SMSBaoKey, BaseURL: cfg.SMSBaoBaseURL}
	case "log":
		sms = adapters.LogSender{Log: log}
	default:
		return nil, nil, fmt.Errorf("VG_SMS_PROVIDER: unsupported %q", cfg.SMSProvider)
	}
	var deny denylist.List
	if cfg.RedisAddr != "" {
		r, err := denylist.Open(context.Background(), cfg.RedisAddr, cfg.RedisPassword)
		if err != nil {
			log.Warn("redis deny-list not reachable at startup: revocation will degrade to the access-token TTL until it returns", "addr", cfg.RedisAddr)
		}
		deny = r
	} else {
		log.Warn("VG_REDIS_ADDR not set: no deny-list, device revocation takes effect only when access tokens expire (<= 15 min)")
	}
	svc := &app.Service{
		Store: &adapters.MySQLStore{DB: pool}, SMS: sms, Keys: ring, HashKey: []byte(cfg.PhoneHashKey), Policy: domain.DefaultPolicy(),
		Deny: deny, IM: im, IMKey: []byte(cfg.IMTokenKey),
		Observe: func(kind, result string) {
			pm.AuthEvents.Inc(kind, result)
			if result == "rate_limited" || result == "locked" || result == "reuse" {
				log.Warn("auth risk event", "kind", kind, "result", result)
			}
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/", httpx.NotFound)
	auth := authn.Require(authn.Config{Ring: ring, Deny: deny, OnDegraded: func() { pm.DenyDegraded.Inc() }})
	(&identityhttp.Handler{Svc: svc, Trusted: trusted, Log: log, Auth: auth}).Mount(mux)
	return mux, auth, nil
}

// imSyncHandler is the outbox handler for identity.im_sync (worker process).
func imSyncHandler(cfg config.Config, pool *sql.DB, im *imgw.Client) outbox.Handler {
	svc := &app.Service{Store: &adapters.MySQLStore{DB: pool}, IM: im, IMKey: []byte(cfg.IMTokenKey), Policy: domain.DefaultPolicy()}
	return func(ctx context.Context, e outbox.Event) error {
		var p struct{ UID, Class string }
		if err := json.Unmarshal(e.Payload, &p); err != nil || p.UID == "" {
			return fmt.Errorf("identity.im_sync: bad payload")
		}
		return svc.SyncIM(ctx, p.UID, domain.DeviceClass(p.Class))
	}
}

// imNotifier delivers account notices as IM messages from the system account to the user's own person channel.
type imNotifier struct{ im *imgw.Client }

func (n imNotifier) Notify(ctx context.Context, uid, clientMsgNo string, payload []byte) error {
	_, err := n.im.Send(ctx, domain.SystemUID, uid, 1, clientMsgNo, payload)
	return err
}

// deviceNoticeHandler is the outbox handler for identity.device_notice (worker process). The IM payload is the outbox payload plus a message
// type, so clients can render it as a service notice.
func deviceNoticeHandler(im *imgw.Client) outbox.Handler {
	svc := &app.Service{Notify: imNotifier{im}}
	return func(ctx context.Context, e outbox.Event) error {
		var n map[string]any
		if err := json.Unmarshal(e.Payload, &n); err != nil {
			return fmt.Errorf("identity.device_notice: bad payload")
		}
		uid, _ := n["uid"].(string)
		if uid == "" {
			return fmt.Errorf("identity.device_notice: bad payload")
		}
		delete(n, "uid")
		n["type"] = "account_notice"
		msg, _ := json.Marshal(n)
		return svc.SendNotice(ctx, e.ID, uid, msg)
	}
}

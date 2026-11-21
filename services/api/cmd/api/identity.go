package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"

	"veyrogram/api/internal/identity/adapters"
	"veyrogram/api/internal/identity/app"
	"veyrogram/api/internal/identity/domain"
	identityhttp "veyrogram/api/internal/identity/http"
	"veyrogram/api/internal/identity/ports"
	"veyrogram/api/internal/platform/clientip"
	"veyrogram/api/internal/platform/config"
	"veyrogram/api/internal/platform/httpx"
	"veyrogram/api/internal/platform/metrics"
	"veyrogram/api/internal/platform/token"
)

// identityMux wires the identity module (feature flag auth.sms_login) and returns a mux serving /v1/auth/*.
func identityMux(cfg config.Config, pool *sql.DB, pm *metrics.Platform, log *slog.Logger) (*http.ServeMux, error) {
	ring, err := token.ParseRing(cfg.TokenKeys)
	if err != nil {
		return nil, fmt.Errorf("VG_TOKEN_KEYS: %w", err)
	}
	trusted, err := clientip.ParseTrusted(cfg.TrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("VG_TRUSTED_PROXIES: %w", err)
	}
	var sms ports.SMSSender
	switch cfg.SMSProvider {
	case "http":
		sms = &adapters.HTTPSender{URL: cfg.SMSHTTPURL, Token: cfg.SMSHTTPToken}
	case "log":
		sms = adapters.LogSender{Log: log}
	default:
		return nil, fmt.Errorf("VG_SMS_PROVIDER: unsupported %q", cfg.SMSProvider)
	}
	svc := &app.Service{
		Store: &adapters.MySQLStore{DB: pool}, SMS: sms, Keys: ring, HashKey: []byte(cfg.PhoneHashKey), Policy: domain.DefaultPolicy(),
		Observe: func(kind, result string) {
			pm.AuthEvents.Inc(kind, result)
			if result == "rate_limited" || result == "locked" || result == "reuse" {
				log.Warn("auth risk event", "kind", kind, "result", result)
			}
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/", httpx.NotFound)
	(&identityhttp.Handler{Svc: svc, Trusted: trusted, Log: log}).Mount(mux)
	return mux, nil
}

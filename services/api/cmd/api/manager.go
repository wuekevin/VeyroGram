package main

import (
	"database/sql"
	"log/slog"
	"net/http"

	adminadapters "veyrogram/api/internal/admin/adapters"
	adminapp "veyrogram/api/internal/admin/app"
	adminhttp "veyrogram/api/internal/admin/http"
	"veyrogram/api/internal/platform/audit"
	"veyrogram/api/internal/platform/clientip"
	"veyrogram/api/internal/platform/config"
	"veyrogram/api/internal/platform/metrics"
)

func adminService(cfg config.Config, pool *sql.DB) (*adminapp.Service, error) {
	box, err := adminadapters.NewSecretBox(cfg.AdminTOTPKey)
	if err != nil {
		return nil, err
	}
	return &adminapp.Service{Store: &adminadapters.MySQLStore{DB: pool}, Hasher: adminadapters.DefaultArgon2id(), Box: box,
		Audit: &audit.Log{DB: pool}, Policy: adminapp.DefaultPolicy(), Issuer: "VeyroGram"}, nil
}

// managerServer builds the admin plane handler (AD-24): own listener, optional allowlist, own rate limits, no routes shared with the user API.
func managerHandler(cfg config.Config, pool *sql.DB, pm *metrics.Platform, log *slog.Logger) (http.Handler, error) {
	svc, err := adminService(cfg, pool)
	if err != nil {
		return nil, err
	}
	allow, err := clientip.ParseTrusted(cfg.ManagerAllow)
	if err != nil {
		return nil, err
	}
	trusted, err := clientip.ParseTrusted(cfg.TrustedProxies)
	if err != nil {
		return nil, err
	}
	r := adminhttp.NewRouter(svc)
	r.Allow = allow
	r.PublicPerMinute = cfg.ManagerPublicRate
	r.ClientIP = func(req *http.Request) string { return clientip.From(req, trusted) }
	r.OnDenied = func(reason string) {
		pm.AdminDenied.Inc(reason)
		if reason != "rate_limited" {
			log.Warn("admin plane request refused", "reason", reason)
		}
	}
	r.Mount(log)
	return r, nil
}

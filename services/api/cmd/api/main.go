// Command api is the VeyroGram business API (modular monolith edge + composition root).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	msgapp "veyrogram/api/internal/message/app"
	"veyrogram/api/internal/platform/config"
	"veyrogram/api/internal/platform/db"
	"veyrogram/api/internal/platform/health"
	"veyrogram/api/internal/platform/httpx"
	"veyrogram/api/internal/platform/idempotency"
	"veyrogram/api/internal/platform/imgw"
	"veyrogram/api/internal/platform/imwebhook"
	"veyrogram/api/internal/platform/logx"
	"veyrogram/api/internal/platform/metrics"
	"veyrogram/api/internal/platform/migrate"
	"veyrogram/api/internal/platform/outbox"
	"veyrogram/api/migrations"
)

const exitConfig = 78 // EX_CONFIG: invalid configuration

// version is set at build time (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stderr))
}

func run(args []string, getenv func(string) string, stderr *os.File) int {
	if len(args) > 0 {
		switch args[0] {
		case "migrate":
			return runMigrate(args[1:], getenv, stderr)
		case "install":
			return runInstall(args[1:], getenv, os.Stdout, stderr)
		case "probe":
			return runProbe(args[1:], getenv, os.Stdout, stderr)
		case "worker", "audit", "outbox", "db", "sync", "admin":
			return runOps(args, getenv, stderr)
		}
	}
	for _, a := range args {
		switch a {
		case "-version", "--version":
			fmt.Println(version)
			return 0
		case "-check-config", "--check-config":
			// Preflight used by deploy scripts: validate configuration and exit without listening.
			if err := config.Error(violationsOf(getenv)); err != nil {
				fmt.Fprintln(stderr, err)
				return exitConfig
			}
			fmt.Println("config ok")
			return 0
		}
	}
	cfg, violations := config.Load(getenv)
	if err := config.Error(violations); err != nil {
		fmt.Fprintln(stderr, err)
		return exitConfig
	}
	log := logx.New(stderr, slog.LevelInfo).With("version", version)
	reg := metrics.NewRegistry()
	pm := metrics.NewPlatform(reg)

	hc := health.NewRegistry(2 * time.Second) // modules register readiness checks here as they are added

	im, err := imgw.New(cfg.IMAPIURL)
	if err != nil {
		fmt.Fprintln(stderr, "VG_IM_API_URL:", err)
		return exitConfig
	}
	if err := im.AddPeers(cfg.IMAPIPeerURLs...); err != nil {
		fmt.Fprintln(stderr, "VG_IM_API_PEER_URLS:", err)
		return exitConfig
	}
	im.Observe = func(op, result string, d time.Duration) {
		pm.IMGWCalls.Inc(op, result)
		pm.IMGWDuration.Observe(d.Seconds(), op)
		if result == "error" {
			log.Warn("imgw call failed", "op", op)
		}
	}
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/", httpx.NotFound)
	var handler http.Handler = publicMux
	var msgMux, convMux, contactMuxH *http.ServeMux
	var managerH http.Handler // admin plane (own listener); nil unless VG_MANAGER_ENABLED
	var notify func(context.Context, []imwebhook.Notified) error
	var gate *msgapp.Gate      // nil without MySQL: the before_send hook then denies every send (fail-closed)
	var authMux *http.ServeMux // /v1/auth/* answers carry credentials, so it is never run through the idempotency response cache
	if cfg.MySQLDSN != "" {
		pool, err := db.Open(cfg.MySQLDSN)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitConfig
		}
		defer pool.Close()
		hc.Register("mysql", db.Ready(pool))
		metrics.DBPool(reg, pool)
		reg.OnScrape(func() {
			c, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if n, err := outbox.Pending(c, pool); err == nil {
				pm.OutboxLag.Set(float64(n))
			}
		})
		if cfg.ManagerEnabled {
			mh, err := managerHandler(cfg, pool, pm, log)
			if err != nil {
				fmt.Fprintln(stderr, "admin plane:", err)
				return exitConfig
			}
			managerH = mh
		}
		handler = idempotency.Middleware(&idempotency.Store{DB: pool}, nil, nil)(handler)
		if cfg.AuthSMSLogin {
			m, auth, err := identityMux(cfg, pool, pm, log, im)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return exitConfig
			}
			authMux = m
			gate = messageGate(cfg, pool, pm, log)
			notify = notifyHook(pool)
			msgMux = messageMux(pool, log, im, gate, auth)
			convMux = conversationMux(pool, log, im, auth)
			contactMuxH = contactMux(pool, log, auth)
		} else {
			gate = messageGate(cfg, pool, pm, log)
			notify = notifyHook(pool)
		}
	}
	if authMux != nil {
		rest := handler
		top := http.NewServeMux()
		top.Handle("/v1/auth/", authMux)
		top.Handle("/v1/devices", authMux)
		top.Handle("/v1/devices/", authMux)
		top.Handle("/v1/im/", authMux)
		if msgMux != nil {
			top.Handle("/v1/messages", msgMux)
			top.Handle("/v1/messages/", msgMux)
			top.Handle("/v1/sync/", convMux)
			top.Handle("/v1/blocks", contactMuxH)
			top.Handle("/v1/blocks/", contactMuxH)
		}
		top.Handle("/", rest)
		handler = top
	}
	clock := func() float64 { return float64(time.Now().UnixNano()) / 1e9 }
	handler = pm.Middleware(handler, clock)
	public := &http.Server{Addr: cfg.HTTPAddr, Handler: httpx.RequestID(logx.Middleware(log)(handler)), ReadHeaderTimeout: 5 * time.Second}
	// /metrics lives only on the internal listener (loopback/private by config validation) and needs the bearer token (T-17).
	imux := http.NewServeMux()
	imux.Handle("/metrics", reg.Handler(cfg.MetricsToken))
	imux.Handle("/", hc.Mux())
	internal := &http.Server{Addr: cfg.InternalAddr, Handler: httpx.RequestID(imux), ReadHeaderTimeout: 5 * time.Second}

	servers := map[string]*http.Server{"public": public, "internal": internal}
	if managerH != nil {
		servers["manager"] = &http.Server{Addr: cfg.ManagerAddr, Handler: httpx.RequestID(logx.Middleware(log)(managerH)), ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 8 << 10}
	}
	if cfg.IMWebhookSecret != "" {
		allow, err := imwebhook.ParseAllow(cfg.IMWebhookAllow)
		if err != nil {
			fmt.Fprintln(stderr, "VG_IM_WEBHOOK_ALLOW:", err)
			return exitConfig
		}
		var handlers imwebhook.Handlers
		if gate != nil {
			handlers.BeforeSend = beforeSendHook(gate)
		}
		handlers.Notify = notify
		hook := imwebhook.Handler(imwebhook.Config{Secret: cfg.IMWebhookSecret, SigningSecret: cfg.IMWebhookSigning, Allow: allow, Handlers: handlers, OnReject: func(reason string) {
			pm.IMWebhookReject.Inc(reason)
			log.Warn("im webhook rejected", "reason", reason)
		}})
		servers["im-webhook"] = &http.Server{Addr: cfg.IMWebhookAddr, Handler: logx.Middleware(log)(hook), ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 8 << 10}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, len(servers))
	for name, s := range servers {
		go func() {
			log.Info("listening", "listener", name, "addr", s.Addr, "env", string(cfg.Env))
			if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errc <- fmt.Errorf("%s listener: %w", name, err)
			}
		}()
	}
	select {
	case <-ctx.Done():
	case err := <-errc:
		log.Error("server error", "err", err)
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, s := range servers {
		_ = s.Shutdown(shutdown)
	}
	return 0
}

func violationsOf(getenv func(string) string) []config.Violation {
	_, vs := config.Load(getenv)
	return vs
}

// runMigrate implements `veyrogram-api migrate up | down [N] | status | clear-dirty`. Migrations are never run implicitly at startup.
func runMigrate(args []string, getenv func(string) string, stderr *os.File) int {
	cfg, vs := config.Load(getenv)
	if err := config.Error(vs); err != nil {
		fmt.Fprintln(stderr, err)
		return exitConfig
	}
	if cfg.MySQLDSN == "" {
		fmt.Fprintln(stderr, "VG_MYSQL_DSN is required for migrate")
		return exitConfig
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: veyrogram-api migrate up | down [N] | status | clear-dirty")
		return 2
	}
	pool, err := db.Open(cfg.MySQLDSN)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitConfig
	}
	defer pool.Close()
	ms, err := migrate.Load(migrations.FS)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	r := &migrate.Runner{DB: pool, Migrations: ms}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	switch args[0] {
	case "up":
		done, err := r.Up(ctx)
		fmt.Printf("applied: %v\n", done)
		return report(err, stderr)
	case "down":
		n := 1
		if len(args) > 1 {
			if n, err = strconv.Atoi(args[1]); err != nil || n < 1 {
				fmt.Fprintln(stderr, "down N: N must be a positive integer")
				return 2
			}
		}
		done, err := r.Down(ctx, n)
		fmt.Printf("reverted: %v\n", done)
		return report(err, stderr)
	case "status":
		st, err := r.Status(ctx)
		fmt.Printf("applied=%v pending=%v dirty=%v\n", st.Applied, st.Pending, st.Dirty)
		return report(err, stderr)
	case "clear-dirty":
		return report(r.ClearDirty(ctx), stderr)
	}
	fmt.Fprintln(stderr, "unknown migrate command")
	return 2
}

func report(err error, stderr *os.File) int {
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	convadapters "veyrogram/api/internal/conversation/adapters"
	"veyrogram/api/internal/identity/domain"
	"veyrogram/api/internal/platform/audit"
	"veyrogram/api/internal/platform/config"
	"veyrogram/api/internal/platform/db"
	"veyrogram/api/internal/platform/health"
	"veyrogram/api/internal/platform/imgw"
	"veyrogram/api/internal/platform/logx"
	"veyrogram/api/internal/platform/metrics"
	"veyrogram/api/internal/platform/outbox"
)

// runOps implements the operational subcommands:
//
//	worker                      run the outbox worker (SIGTERM stops it after the current batch)
//	audit verify                recompute the audit hash chain
//	audit check-anchors [-days N] [-require-all]   compare the log with the last N days of off-host anchors
//	audit anchor                verify, then write today's chain head to the off-host anchor store
//	outbox replay <id>          put a dead-lettered event back in the queue (and record it in the audit log)
//	db provision [-database D -app-user U -migrator-user M -host H]   create/re-key the app and migrator accounts (admin; passwords from env)
//	db grant-app -database D -user U -host H   (as admin, via VG_MYSQL_ADMIN_DSN) per-table grants for the application account
func runOps(args []string, getenv func(string) string, stderr *os.File) int {
	if args[0] == "db" {
		return runDBGrant(args[1:], getenv, stderr)
	}
	cfg, vs := config.Load(getenv)
	if err := config.Error(vs); err != nil {
		fmt.Fprintln(stderr, err)
		return exitConfig
	}
	if cfg.MySQLDSN == "" {
		fmt.Fprintln(stderr, "VG_MYSQL_DSN is required")
		return exitConfig
	}
	pool, err := db.Open(cfg.MySQLDSN)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitConfig
	}
	defer pool.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log := logx.New(stderr, slog.LevelInfo)
	switch args[0] {
	case "worker":
		reg := metrics.NewRegistry()
		pm := metrics.NewPlatform(reg)
		metrics.DBPool(reg, pool)
		reg.OnScrape(func() {
			c, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if n, err := outbox.Pending(c, pool); err == nil {
				pm.OutboxLag.Set(float64(n))
			}
		})
		hc := health.NewRegistry(2 * time.Second)
		hc.Register("mysql", db.Ready(pool))
		mux := http.NewServeMux()
		mux.Handle("/metrics", reg.Handler(cfg.MetricsToken))
		mux.Handle("/", hc.Mux())
		srv := &http.Server{Addr: cfg.WorkerInternalAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		go func() {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("worker metrics listener failed", "err", err)
			}
		}()
		defer srv.Shutdown(context.Background())
		handlers := map[string]outbox.Handler{} // topics register their handlers here as modules are added
		if cfg.AuthSMSLogin {
			im, err := imgw.New(cfg.IMAPIURL)
			if err != nil {
				fmt.Fprintln(stderr, "VG_IM_API_URL:", err)
				return exitConfig
			}
			handlers[domain.TopicIMSync] = imSyncHandler(cfg, pool, im)
			handlers[domain.TopicDeviceNotice] = deviceNoticeHandler(im)
			handlers[convadapters.TopicStateChanged] = stateNoticeHandler(im)
		}
		w := &outbox.Worker{DB: pool, Handlers: handlers,
			Log: func(msg string, kv ...any) { log.Warn(msg, kv...) },
			OnResult: func(topic, result string) {
				pm.OutboxEvents.Inc(topic, result)
				log.Info("outbox", "topic", topic, "result", result)
			}}
		log.Info("worker started", "version", version, "metrics_addr", cfg.WorkerInternalAddr)
		_ = w.Run(ctx, time.Second)
		return 0
	case "audit":
		l := &audit.Log{DB: pool}
		if len(args) < 2 {
			fmt.Fprintln(stderr, "usage: audit verify | audit anchor | audit check-anchors")
			return 2
		}
		switch args[1] {
		case "verify":
			h, err := l.Verify(ctx)
			if err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
			fmt.Printf("audit chain ok rows=%d head_id=%d head_hash=%s\n", h.Rows, h.ID, h.Hash)
			return 0
		case "check-anchors":
			fs := flag.NewFlagSet("check-anchors", flag.ContinueOnError)
			fs.SetOutput(stderr)
			days := fs.Int("days", 30, "how many days back to compare (today included)")
			requireAll := fs.Bool("require-all", false, "fail when any day has no anchor")
			if err := fs.Parse(args[2:]); err != nil || *days < 1 {
				return 2
			}
			if cfg.AuditAnchorURL == "" {
				fmt.Fprintln(stderr, "VG_AUDIT_ANCHOR_URL is required for audit check-anchors")
				return exitConfig
			}
			rep, err := l.CheckAnchors(ctx, audit.HTTPSink{Base: cfg.AuditAnchorURL, Token: cfg.AuditAnchorToken}, *days)
			if err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
			fmt.Printf("anchors ok checked=%d missing=%d %v\n", rep.Checked, len(rep.Missing), rep.Missing)
			if rep.Checked == 0 || (*requireAll && len(rep.Missing) > 0) {
				fmt.Fprintln(stderr, "error: no anchor was verifiable, or days are missing and -require-all is set")
				return 1
			}
			return 0
		case "anchor":
			if cfg.AuditAnchorURL == "" {
				fmt.Fprintln(stderr, "VG_AUDIT_ANCHOR_URL is required for audit anchor")
				return exitConfig
			}
			a, err := l.AnchorHead(ctx, audit.HTTPSink{Base: cfg.AuditAnchorURL, Token: cfg.AuditAnchorToken})
			if err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
			fmt.Printf("anchored rows=%d head_id=%d head_hash=%s\n", a.Rows, a.HeadID, a.HeadHash)
			return 0
		}
	case "outbox":
		if len(args) == 3 && args[1] == "replay" {
			id, err := strconv.ParseUint(args[2], 10, 64)
			if err != nil {
				fmt.Fprintln(stderr, "replay: dead letter id must be a number")
				return 2
			}
			if err := outbox.Replay(ctx, pool, time.Now(), id); err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
			actor := getenv("USER")
			if actor == "" {
				actor = "operator"
			}
			if err := (&audit.Log{DB: pool}).Append(ctx, audit.Entry{Actor: "cli:" + actor, Action: "outbox.replay", Target: fmt.Sprintf("dead_letter:%d", id)}); err != nil {
				fmt.Fprintln(stderr, "replayed, but the audit entry failed:", err)
				return 1
			}
			fmt.Println("replayed")
			return 0
		}
	}
	fmt.Fprintln(stderr, "unknown command; see `veyrogram-api` docs")
	return 2
}

func runDBProvision(args []string, getenv func(string) string, stderr io.Writer) int {
	fs := flag.NewFlagSet("provision", flag.ContinueOnError)
	fs.SetOutput(stderr)
	database := fs.String("database", "veyrogram", "application database")
	host := fs.String("host", "127.0.0.1", "account host")
	appUser := fs.String("app-user", "vg_app", "application account")
	migUser := fs.String("migrator-user", "vg_migrator", "migrator account")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dsn, appPass, migPass := getenv("VG_MYSQL_ADMIN_DSN"), getenv("VG_PROVISION_APP_PASSWORD"), getenv("VG_PROVISION_MIGRATOR_PASSWORD")
	if dsn == "" || appPass == "" || migPass == "" {
		fmt.Fprintln(stderr, "VG_MYSQL_ADMIN_DSN, VG_PROVISION_APP_PASSWORD and VG_PROVISION_MIGRATOR_PASSWORD are required (passwords are never accepted as arguments)")
		return exitConfig
	}
	admin, err := db.Open(dsn)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitConfig
	}
	defer admin.Close()
	if err := db.Provision(context.Background(), admin, *database, *host, *appUser, appPass, *migUser, migPass); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Printf("provisioned database=%s app=%s migrator=%s host=%s (next: migrate up as the migrator, then db grant-app)\n", *database, *appUser, *migUser, *host)
	return 0
}

func runDBGrant(args []string, getenv func(string) string, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "provision" {
		return runDBProvision(args[1:], getenv, stderr)
	}
	if len(args) == 0 || args[0] != "grant-app" {
		fmt.Fprintln(stderr, "usage: db grant-app -database D -user U -host H   (admin DSN in VG_MYSQL_ADMIN_DSN)")
		return 2
	}
	fs := flag.NewFlagSet("grant-app", flag.ContinueOnError)
	fs.SetOutput(stderr)
	database := fs.String("database", "", "application database")
	user := fs.String("user", "", "application account")
	host := fs.String("host", "127.0.0.1", "account host")
	if err := fs.Parse(args[1:]); err != nil || *database == "" || *user == "" {
		fmt.Fprintln(stderr, "-database and -user are required")
		return 2
	}
	dsn := getenv("VG_MYSQL_ADMIN_DSN")
	if dsn == "" {
		fmt.Fprintln(stderr, "VG_MYSQL_ADMIN_DSN is required (administrator account; never the application account)")
		return exitConfig
	}
	admin, err := db.Open(dsn)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitConfig
	}
	defer admin.Close()
	granted, err := db.GrantApp(context.Background(), admin, *database, *user, *host)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Printf("granted on %d tables (audit_log: SELECT, INSERT only)\n", len(granted))
	return 0
}

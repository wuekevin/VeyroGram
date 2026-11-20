package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"veyrogram/api/internal/platform/audit"
	"veyrogram/api/internal/platform/config"
	"veyrogram/api/internal/platform/db"
	"veyrogram/api/internal/platform/logx"
	"veyrogram/api/internal/platform/outbox"
)

// runOps implements the operational subcommands:
//
//	worker                      run the outbox worker (SIGTERM stops it after the current batch)
//	audit verify                recompute the audit hash chain
//	audit anchor                verify, then write today's chain head to the off-host anchor store
//	outbox replay <id>          put a dead-lettered event back in the queue (and record it in the audit log)
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
		w := &outbox.Worker{DB: pool, Handlers: map[string]outbox.Handler{}, // topics register their handlers here as modules are added
			Log:      func(msg string, kv ...any) { log.Warn(msg, kv...) },
			OnResult: func(topic, result string) { log.Info("outbox", "topic", topic, "result", result) }}
		log.Info("worker started", "version", version)
		_ = w.Run(ctx, time.Second)
		return 0
	case "audit":
		l := &audit.Log{DB: pool}
		if len(args) < 2 {
			fmt.Fprintln(stderr, "usage: audit verify | audit anchor")
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

func runDBGrant(args []string, getenv func(string) string, stderr io.Writer) int {
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

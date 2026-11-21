package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"veyrogram/api/internal/admin/domain"
	"veyrogram/api/internal/platform/config"
	"veyrogram/api/internal/platform/db"
	"veyrogram/api/internal/platform/migrate"
	"veyrogram/api/internal/platform/preflight"
	"veyrogram/api/migrations"
)

const (
	exitBlocked          = 3 // preflight found a blocker
	exitAlreadyInstalled = 4 // the database says this installation is already initialised
)

// runInstall implements the install wizard:
//
//	install preflight [-install] [-fresh] [-json] [-user U] [-ports host:port,...]   check the host; exit 3 if anything blocks
//	install run -username U [-password-stdin] [-fresh]                               configuration check, preflight, migrations, first administrator
//
// `install run` is safe to run again: every step before the administrator is idempotent, and once the database records the installation as
// initialised the wizard refuses (exit 4) whatever files exist on disk. Deleting install.lock changes nothing; that file is informational only.
func runInstall(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: install preflight [-install] [-fresh] [-json] | install run -username U [-password-stdin]")
		return 2
	}
	switch args[0] {
	case "preflight":
		return installPreflight(args[1:], getenv, stdout, stderr)
	case "run":
		return installRun(args[1:], getenv, stdout, stderr)
	}
	fmt.Fprintln(stderr, "unknown install command")
	return 2
}

// preflightOptions derives what to check from the same VG_* variables the service reads, so the check and the service cannot disagree.
func preflightOptions(cfg config.Config, getenv func(string) string, install, fresh bool, user string, extra []string) preflight.Options {
	o := preflight.Options{
		Strict:       cfg.Strict(),
		Install:      install,
		Fresh:        fresh,
		MySQLDSN:     firstNonEmpty(getenv("VG_MYSQL_ADMIN_DSN"), cfg.MySQLDSN),
		RedisAddr:    cfg.RedisAddr,
		RedisPass:    cfg.RedisPassword,
		RequireRedis: cfg.AuthSMSLogin,
		Ports: []preflight.Port{{Name: "api", Addr: cfg.HTTPAddr}, {Name: "internal", Addr: cfg.InternalAddr},
			{Name: "worker-internal", Addr: cfg.WorkerInternalAddr}, {Name: "im-webhook", Addr: cfg.IMWebhookAddr}},
	}
	if cfg.ManagerEnabled {
		o.Ports = append(o.Ports, preflight.Port{Name: "manager", Addr: cfg.ManagerAddr})
	}
	for _, a := range extra {
		if a = strings.TrimSpace(a); a != "" {
			o.Ports = append(o.Ports, preflight.Port{Name: "extra:" + a, Addr: a})
		}
	}
	if install {
		o.ServiceUser = user
		o.EnvFile = firstNonEmpty(getenv("VG_ENV_FILE"), "/etc/veyrogram/api.env")
		o.DiskPaths = []string{"/opt/veyrogram"}
	}
	return o
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// configFindings turns configuration violations into blocking findings: a host cannot be called ready with a configuration the service refuses to start on.
func configFindings(vs []config.Violation) []preflight.Finding {
	var out []preflight.Finding
	for _, v := range vs {
		out = append(out, preflight.Finding{Check: "config:" + v.Key, Sev: preflight.Block, Detail: v.Reason, Fix: "correct " + v.Key + " in the environment file"})
	}
	return out
}

func installPreflight(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("preflight", flag.ContinueOnError)
	fs.SetOutput(stderr)
	install := fs.Bool("install", false, "also check what the installer itself needs: root, systemd, the service account, the environment file")
	fresh := fs.Bool("fresh", false, "nothing of ours is running yet: a port already in use is a conflict")
	asJSON := fs.Bool("json", false, "machine-readable output")
	user := fs.String("user", "veyrogram", "service account (with -install)")
	ports := fs.String("ports", "", "extra host:port listeners to check, comma separated (the IM gateway, for instance)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, vs := config.Load(getenv)
	findings := configFindings(vs)
	var extra []string
	if *ports != "" {
		extra = strings.Split(*ports, ",")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	findings = append(findings, preflight.Run(ctx, preflight.OSHost{}, preflightOptions(cfg, getenv, *install, *fresh, *user, extra))...)
	printFindings(stdout, findings, *asJSON)
	if b := preflight.Blockers(findings); len(b) > 0 {
		fmt.Fprintf(stderr, "preflight: %d blocking problem(s); nothing was changed\n", len(b))
		return exitBlocked
	}
	return 0
}

func printFindings(w io.Writer, fs []preflight.Finding, asJSON bool) {
	if asJSON {
		_ = json.NewEncoder(w).Encode(fs)
		return
	}
	tag := map[preflight.Severity]string{preflight.OK: "  ok ", preflight.Warn: "warn ", preflight.Block: "BLOCK"}
	for _, f := range fs {
		fmt.Fprintf(w, "[%s] %-22s %s\n", tag[f.Sev], f.Check, f.Detail)
		if f.Sev != preflight.OK && f.Fix != "" {
			fmt.Fprintf(w, "        -> %s\n", f.Fix)
		}
	}
}

func installRun(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	username := fs.String("username", "", "login name of the first administrator")
	stdin := fs.Bool("password-stdin", false, "read the administrator password from standard input")
	fresh := fs.Bool("fresh", false, "nothing of ours is running yet: a port already in use is a conflict")
	ports := fs.String("ports", "", "extra host:port listeners to check, comma separated")
	if err := fs.Parse(args); err != nil || *username == "" {
		fmt.Fprintln(stderr, "-username is required")
		return 2
	}
	step := func(n int, name string) { fmt.Fprintf(stdout, "[%d/6] %s\n", n, name) }

	// 1. Configuration. Nothing is touched until the configuration the service will start with is valid and complete.
	step(1, "configuration")
	cfg, vs := config.Load(getenv)
	if err := config.Error(vs); err != nil {
		fmt.Fprintln(stderr, err)
		return exitConfig
	}
	if cfg.MySQLDSN == "" {
		fmt.Fprintln(stderr, "VG_MYSQL_DSN is required")
		return exitConfig
	}
	if len(cfg.AdminTOTPKey) < 32 {
		fmt.Fprintln(stderr, "VG_ADMIN_TOTP_KEY (at least 32 characters) is required: it encrypts the administrator's TOTP secret")
		return exitConfig
	}

	// 2. The administrator's credentials are checked before anything is written, so a weak password leaves the system exactly as it was.
	step(2, "administrator credentials")
	password, err := bootstrapPassword(getenv, *stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if !domain.ValidUsername(*username) {
		fmt.Fprintln(stderr, "refused: the username must be 3-64 characters of letters, digits, dot, dash or underscore")
		return 1
	}
	var weak *domain.WeakPasswordError
	if err := domain.ValidatePassword(*username, password); errors.As(err, &weak) {
		fmt.Fprintln(stderr, "refused: the password does not meet the policy:")
		for _, p := range weak.Problems {
			fmt.Fprintln(stderr, "  -", p)
		}
		return 1
	} else if err != nil {
		fmt.Fprintln(stderr, "refused:", err)
		return 1
	}

	// 3. Host and servers.
	step(3, "host preflight")
	var extra []string
	if *ports != "" {
		extra = strings.Split(*ports, ",")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	findings := preflight.Run(ctx, preflight.OSHost{}, preflightOptions(cfg, getenv, false, *fresh, "", extra))
	for _, f := range findings {
		if f.Sev != preflight.OK {
			printFindings(stdout, []preflight.Finding{f}, false)
		}
	}
	if b := preflight.Blockers(findings); len(b) > 0 {
		fmt.Fprintf(stderr, "preflight: %d blocking problem(s); nothing was changed\n", len(b))
		return exitBlocked
	}

	pool, err := db.Open(cfg.MySQLDSN)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitConfig
	}
	defer pool.Close()

	// 4. Already installed? The database decides, never a file.
	step(4, "installation state")
	done, err := installed(ctx, pool)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if done {
		fmt.Fprintln(stderr, "refused: this installation is already initialised. The decision is stored in the database; removing install.lock does not re-open it.")
		return exitAlreadyInstalled
	}

	// 5. Schema. Idempotent: re-running applies only what is missing. Each migration has a down step (`migrate down`).
	step(5, "database migrations")
	ms, err := migrate.Load(migrations.FS)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	applied, err := (&migrate.Runner{DB: pool, Migrations: ms}).Up(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err, "(fix the cause and run the wizard again; it resumes)")
		return 1
	}
	fmt.Fprintf(stdout, "      applied: %v\n", applied)

	// 6. The first administrator, once.
	step(6, "first administrator")
	if code := bootstrapAdmin(cfg, pool, *username, password, stderr); code != 0 {
		return code
	}
	writeInstallMarker(getenv("VG_INSTALL_LOCK"), stdout)
	fmt.Fprintln(stdout, "installation complete. Next: start the services, then sign in at the admin site with the password and the authenticator code above.")
	return 0
}

// installed reports whether the database already records the initialisation. A missing table means a database that was never migrated.
func installed(ctx context.Context, pool *sql.DB) (bool, error) {
	var n int
	err := pool.QueryRowContext(ctx, "SELECT COUNT(*) FROM system_state WHERE k='initialized'").Scan(&n)
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1146 { // ER_NO_SUCH_TABLE
		return false, nil
	}
	return n > 0, err
}

// writeInstallMarker leaves a human-readable note of when the install happened. It is informational only; nothing reads it to decide anything.
func writeInstallMarker(path string, w io.Writer) {
	if path == "" {
		return
	}
	note := map[string]string{"installed_at": time.Now().UTC().Format(time.RFC3339), "version": version, "note": "informational only; the installation state is in the database and this file is never read to authorise anything"}
	b, _ := json.MarshalIndent(note, "", "  ")
	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
		if err = os.WriteFile(tmp, append(b, '\n'), 0o644); err == nil {
			err = os.Rename(tmp, path)
		}
		if err != nil {
			fmt.Fprintln(w, "      note: could not write", path, "-", err)
		}
	}
}

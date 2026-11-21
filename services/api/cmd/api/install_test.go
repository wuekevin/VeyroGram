package main

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"veyrogram/api/internal/platform/testdb"
)

const strongPW = "Correct-Horse-Battery-9!"

func installEnv(dsn string, extra ...string) []string {
	return append([]string{"VG_ENV=development", "VG_MYSQL_DSN=" + dsn, "VG_ADMIN_TOTP_KEY=" + adminTOTPKey}, extra...)
}

// Story 1.21 AC1: the preflight checks the real servers and lists blockers; a wrong database or a busy port stops it, without printing secrets.
func TestInstallPreflightAgainstRealServers(t *testing.T) {
	_, dsn := testdb.NewWithDSN(t)
	bin := build(t)

	so, se, code := runBin(t, bin, installEnv(dsn), "install", "preflight")
	if code != 0 {
		t.Fatalf("preflight on a working development host: %d\n%s\n%s", code, so, se)
	}
	if !strings.Contains(so, "mysql-version") || !strings.Contains(so, "8.") {
		t.Errorf("preflight should report the real MySQL version:\n%s", so)
	}

	// JSON output is machine-readable and has the same findings.
	so, _, _ = runBin(t, bin, installEnv(dsn), "install", "preflight", "-json")
	var fs []map[string]any
	if err := json.Unmarshal([]byte(so), &fs); err != nil || len(fs) == 0 {
		t.Fatalf("-json output: %v\n%s", err, so)
	}

	// Redis, when the test environment has one: the real version is read, and an open server is only a warning in development.
	if addr := os.Getenv("VG_TEST_REDIS_ADDR"); addr != "" {
		so, _, code = runBin(t, bin, installEnv(dsn, "VG_REDIS_ADDR="+addr), "install", "preflight")
		if code != 0 || !strings.Contains(so, "redis-version") {
			t.Errorf("redis check: %d\n%s", code, so)
		}
	}

	// A MySQL that cannot be reached blocks (exit 3), and the password in the DSN never reaches the output.
	closed := freeAddr(t)
	badDSN := "root:Sup3r-secret-pw@tcp(" + closed + ")/x"
	so, se, code = runBin(t, bin, installEnv(badDSN), "install", "preflight")
	if code != exitBlocked || !strings.Contains(so, "BLOCK") {
		t.Fatalf("unreachable MySQL must block with %d, got %d\n%s\n%s", exitBlocked, code, so, se)
	}
	if strings.Contains(so+se, "Sup3r-secret-pw") {
		t.Error("the database password leaked into the preflight output")
	}

	// -fresh: a port that is already in use is a conflict.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	so, _, code = runBin(t, bin, installEnv(dsn, "VG_HTTP_ADDR="+l.Addr().String()), "install", "preflight", "-fresh")
	if code != exitBlocked || !strings.Contains(so, "port:api") {
		t.Errorf("a busy port must block a fresh install: %d\n%s", code, so)
	}
	if _, _, code = runBin(t, bin, installEnv(dsn, "VG_HTTP_ADDR="+l.Addr().String()), "install", "preflight"); code != 0 {
		t.Errorf("without -fresh a bound port is our own service and must not block: %d", code)
	}

	// A configuration the service would refuse is a blocker too.
	so, _, code = runBin(t, bin, []string{"VG_ENV=production", "VG_MYSQL_DSN=" + dsn, "VG_SMS_FIXED_CODE=123456"}, "install", "preflight")
	if code != exitBlocked || !strings.Contains(so, "config:VG_SMS_FIXED_CODE") {
		t.Errorf("invalid configuration must block: %d\n%s", code, so)
	}
}

// Story 1.21 AC3 and the security negatives: weak configuration and weak passwords are refused before anything is written; a successful install
// cannot be repeated, even after install.lock is deleted.
func TestInstallWizardRefusesWeakInputsAndReinitialisation(t *testing.T) {
	d, dsn := testdb.NewWithDSN(t)
	bin := build(t)
	lock := filepath.Join(t.TempDir(), "state", "install.lock")
	env := installEnv(dsn, "VG_ADMIN_BOOTSTRAP_PASSWORD="+strongPW, "VG_INSTALL_LOCK="+lock)
	tables := func() int {
		var n int
		if err := d.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Weak configuration: refused, nothing created.
	_, se, code := runBin(t, bin, []string{"VG_ENV=production", "VG_MYSQL_DSN=" + dsn, "VG_ADMIN_TOTP_KEY=" + adminTOTPKey, "VG_SMS_FIXED_CODE=123456", "VG_ADMIN_BOOTSTRAP_PASSWORD=" + strongPW}, "install", "run", "-username", "owner")
	if code != exitConfig || !strings.Contains(se, "VG_SMS_FIXED_CODE") {
		t.Fatalf("weak configuration: %d %q", code, se)
	}
	// Missing authenticator-encryption key.
	if _, se, code = runBin(t, bin, []string{"VG_ENV=development", "VG_MYSQL_DSN=" + dsn, "VG_ADMIN_BOOTSTRAP_PASSWORD=" + strongPW}, "install", "run", "-username", "owner"); code != exitConfig || !strings.Contains(se, "VG_ADMIN_TOTP_KEY") {
		t.Fatalf("missing TOTP key: %d %q", code, se)
	}
	// Weak passwords: refused with every problem, and the database is still empty.
	for _, pw := range []string{"", "short", "password1234", "owner-owner-owner"} {
		_, se, code = runBin(t, bin, installEnv(dsn, "VG_ADMIN_BOOTSTRAP_PASSWORD="+pw), "install", "run", "-username", "owner")
		if code != 1 || !strings.Contains(se, "policy") {
			t.Errorf("password %q must be refused by the policy: %d %q", pw, code, se)
		}
	}
	if _, se, code = runBin(t, bin, installEnv(dsn, "VG_ADMIN_BOOTSTRAP_PASSWORD="+strongPW), "install", "run", "-username", "a b"); code != 1 {
		t.Errorf("bad username must be refused: %d %q", code, se)
	}
	if n := tables(); n != 0 {
		t.Fatalf("refused inputs must leave the database untouched, found %d tables", n)
	}

	// The real thing.
	so, se, code := runBin(t, bin, env, "install", "run", "-username", "owner")
	if code != 0 {
		t.Fatalf("install: %d\n%s\n%s", code, so, se)
	}
	for _, want := range []string{"[1/6]", "[6/6]", "applied:", "secret:", "installation complete"} {
		if !strings.Contains(so, want) {
			t.Errorf("output lacks %q:\n%s", want, so)
		}
	}
	if strings.Contains(so+se, strongPW) {
		t.Error("the administrator password was echoed")
	}
	b, err := os.ReadFile(lock)
	if err != nil || !strings.Contains(string(b), "informational only") {
		t.Fatalf("install marker: %v %q", err, b)
	}
	var admins int
	if err := d.QueryRow("SELECT COUNT(*) FROM admin_user").Scan(&admins); err != nil || admins != 1 {
		t.Fatalf("admins after install: %d %v", admins, err)
	}

	// Run again: refused, with the dedicated exit code, and nothing changes.
	if _, se, code = runBin(t, bin, env, "install", "run", "-username", "second"); code != exitAlreadyInstalled || !strings.Contains(se, "already initialised") {
		t.Fatalf("second run must be refused: %d %q", code, se)
	}
	// Delete the lock file and run again: still refused, because the database decides.
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if _, _, code = runBin(t, bin, env, "install", "run", "-username", "second"); code != exitAlreadyInstalled {
		t.Fatalf("with install.lock deleted the wizard must still refuse: %d", code)
	}
	if _, err := os.Stat(lock); err == nil {
		t.Error("a refused run must not recreate the marker")
	}
	if err := d.QueryRow("SELECT COUNT(*) FROM admin_user").Scan(&admins); err != nil || admins != 1 {
		t.Fatalf("admins after refused re-runs: %d %v", admins, err)
	}
}

// An interrupted install resumes: migrations are idempotent, so running the wizard after a partial run (schema present, no administrator) completes it.
func TestInstallWizardResumesAfterPartialRun(t *testing.T) {
	d, dsn := testdb.NewWithDSN(t)
	bin := build(t)
	if _, se, code := runBin(t, bin, installEnv(dsn), "migrate", "up"); code != 0 {
		t.Fatalf("migrate: %d %s", code, se)
	}
	so, se, code := runBin(t, bin, installEnv(dsn, "VG_ADMIN_BOOTSTRAP_PASSWORD="+strongPW), "install", "run", "-username", "owner")
	if code != 0 {
		t.Fatalf("wizard after a partial install: %d\n%s\n%s", code, so, se)
	}
	var n int
	if err := d.QueryRow("SELECT COUNT(*) FROM system_state WHERE k='initialized'").Scan(&n); err != nil || n != 1 {
		t.Fatalf("initialised marker: %d %v", n, err)
	}
}

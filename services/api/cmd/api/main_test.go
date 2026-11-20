package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"veyrogram/api/internal/platform/audit"
	"veyrogram/api/internal/platform/outbox"
	"veyrogram/api/internal/platform/testdb"
)

func build(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "api")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func baseEnv(extra ...string) []string {
	return append([]string{"PATH=" + os.Getenv("PATH")}, extra...)
}

// Security negative (story 1.1 AC1): a production start with unsafe values must exit non-zero and name every offender.
func TestStartupRefusedWithInsecureProductionConfig(t *testing.T) {
	bin := build(t)
	cmd := exec.Command(bin)
	cmd.Env = baseEnv("VG_ENV=production", "VG_SMS_FIXED_CODE=123456", "VG_ADMIN_PASSWORD=admin1234567", "VG_TOKEN_SIGNING_KEY=short", "VG_CORS_ALLOWED_ORIGINS=*", "VG_DEBUG=true")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != exitConfig {
		t.Fatalf("expected exit code %d, got err=%v", exitConfig, err)
	}
	for _, k := range []string{"VG_SMS_FIXED_CODE", "VG_ADMIN_PASSWORD", "VG_TOKEN_SIGNING_KEY", "VG_CORS_ALLOWED_ORIGINS", "VG_DEBUG"} {
		if !strings.Contains(stderr.String(), k) {
			t.Errorf("stderr must list %s, got:\n%s", k, stderr.String())
		}
	}
}

func TestUnsetEnvRefusesToStartWithoutRequiredSecrets(t *testing.T) {
	bin := build(t)
	cmd := exec.Command(bin)
	cmd.Env = baseEnv() // no VG_ENV: fail closed as production, missing signing key
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil || !strings.Contains(stderr.String(), "VG_TOKEN_SIGNING_KEY") {
		t.Fatalf("must refuse to start, err=%v stderr=%s", err, stderr.String())
	}
}

// Story 1.1 AC2/AC3: health endpoints live only on the internal listener; the public listener must not serve them.
func TestHealthOnlyOnInternalListener(t *testing.T) {
	bin := build(t)
	pub, internal := freeAddr(t), freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	cmd.Env = baseEnv("VG_ENV=development", "VG_HTTP_ADDR="+pub, "VG_INTERNAL_ADDR="+internal)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); _ = cmd.Wait() }()

	get := func(addr, path string) (int, error) {
		c := http.Client{Timeout: time.Second}
		resp, err := c.Get("http://" + addr + path)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		return resp.StatusCode, nil
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if code, err := get(internal, "/healthz"); err == nil && code == 200 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("internal listener did not come up")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if code, _ := get(internal, "/readyz"); code != 200 {
		t.Fatalf("readyz with no failing checks must be 200, got %d", code)
	}
	for _, p := range []string{"/healthz", "/readyz"} {
		if code, err := get(pub, p); err != nil || code != 404 {
			t.Fatalf("public listener must not serve %s (want 404), got %d err=%v", p, code, err)
		}
	}
}

func TestCheckConfigPreflight(t *testing.T) {
	bin := build(t)
	ok := exec.Command(bin, "-check-config")
	ok.Env = baseEnv("VG_ENV=development")
	if out, err := ok.CombinedOutput(); err != nil || !strings.Contains(string(out), "config ok") {
		t.Fatalf("valid config must pass preflight, err=%v out=%s", err, out)
	}
	bad := exec.Command(bin, "-check-config")
	bad.Env = baseEnv("VG_ENV=production", "VG_SMS_FIXED_CODE=123456")
	var ee *exec.ExitError
	if err := bad.Run(); !errors.As(err, &ee) || ee.ExitCode() != exitConfig {
		t.Fatalf("invalid config must fail preflight with %d, got %v", exitConfig, err)
	}
	ver := exec.Command(bin, "-version")
	if out, err := ver.Output(); err != nil || strings.TrimSpace(string(out)) == "" {
		t.Fatalf("-version must print a version, err=%v", err)
	}
}

// ---- Story 1.3: migrate subcommand, MySQL readiness (real MySQL; skipped without VG_TEST_MYSQL_DSN) ----

func runBin(t *testing.T, bin string, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = baseEnv(env...)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatal(err)
		}
		code = ee.ExitCode()
	}
	return so.String(), se.String(), code
}

func TestMigrateRequiresDSNAndRefusesUnsafeConfig(t *testing.T) {
	bin := build(t)
	if _, se, code := runBin(t, bin, []string{"VG_ENV=development"}, "migrate", "status"); code != exitConfig || !strings.Contains(se, "VG_MYSQL_DSN") {
		t.Fatalf("migrate without DSN must exit %d naming VG_MYSQL_DSN, got %d %q", exitConfig, code, se)
	}
	if _, _, code := runBin(t, bin, []string{"VG_ENV=production", "VG_SMS_FIXED_CODE=123456"}, "migrate", "status"); code != exitConfig {
		t.Fatalf("migrate must apply the same strict config validation, got %d", code)
	}
}

func TestMigrateUpStatusDownAgainstRealMySQL(t *testing.T) {
	_, dsn := testdb.NewWithDSN(t)
	bin := build(t)
	env := []string{"VG_ENV=development", "VG_MYSQL_DSN=" + dsn}
	if so, se, code := runBin(t, bin, env, "migrate", "up"); code != 0 || !strings.Contains(so, "applied: [1 2]") {
		t.Fatalf("up failed: %d %q %q", code, so, se)
	}
	if so, _, code := runBin(t, bin, env, "migrate", "status"); code != 0 || !strings.Contains(so, "applied=[1 2] pending=[] dirty=false") {
		t.Fatalf("status after up: %d %q", code, so)
	}
	if so, _, code := runBin(t, bin, env, "migrate", "up"); code != 0 || !strings.Contains(so, "applied: []") {
		t.Fatalf("second up must be a no-op: %d %q", code, so)
	}
	if so, _, code := runBin(t, bin, env, "migrate", "down"); code != 0 || !strings.Contains(so, "reverted: [2]") {
		t.Fatalf("down failed: %d %q", code, so)
	}
	if _, _, code := runBin(t, bin, env, "migrate", "down", "0"); code != 2 {
		t.Fatalf("down 0 must be a usage error, got %d", code)
	}
}

func TestReadyzReflectsMySQLAvailability(t *testing.T) {
	_, dsn := testdb.NewWithDSN(t)
	bin := build(t)
	for _, tc := range []struct {
		name, dsn string
		want      int
	}{{"reachable", dsn, 200}, {"unreachable", "u:p@tcp(127.0.0.1:1)/x", 503}} {
		t.Run(tc.name, func(t *testing.T) {
			pub, internal := freeAddr(t), freeAddr(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cmd := exec.CommandContext(ctx, bin)
			cmd.Env = baseEnv("VG_ENV=development", "VG_HTTP_ADDR="+pub, "VG_INTERNAL_ADDR="+internal, "VG_MYSQL_DSN="+tc.dsn)
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { cancel(); _ = cmd.Wait() }()
			deadline := time.Now().Add(6 * time.Second)
			var code int
			for time.Now().Before(deadline) {
				if resp, err := (&http.Client{Timeout: 3 * time.Second}).Get("http://" + internal + "/readyz"); err == nil {
					code = resp.StatusCode
					resp.Body.Close()
					if code == tc.want {
						return
					}
				}
				time.Sleep(100 * time.Millisecond)
			}
			t.Fatalf("readyz never reached %d (last %d)", tc.want, code)
		})
	}
}

// ---- Story 1.5: observability exposure and log redaction (black-box) ----

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

type proc struct {
	pub, internal string
	stderr        *syncBuf
}

func start(t *testing.T, env ...string) *proc {
	t.Helper()
	bin := build(t)
	p := &proc{pub: freeAddr(t), internal: freeAddr(t), stderr: &syncBuf{}}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin)
	cmd.Env = baseEnv(append([]string{"VG_ENV=development", "VG_HTTP_ADDR=" + p.pub, "VG_INTERNAL_ADDR=" + p.internal}, env...)...)
	cmd.Stderr = p.stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = cmd.Wait() })
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get("http://" + p.internal + "/healthz"); err == nil {
			resp.Body.Close()
			return p
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("server did not start")
	return nil
}

func get(t *testing.T, url, auth string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestMetricsAndDebugEndpointsAreNotPublic(t *testing.T) {
	const tok = "metrics-token-0123456789abcdef"
	p := start(t, "VG_METRICS_TOKEN="+tok)
	for _, path := range []string{"/metrics", "/debug/pprof/", "/debug/vars", "/healthz", "/readyz", "/internal/metrics"} {
		if code, body := get(t, "http://"+p.pub+path, "Bearer "+tok); code == 200 || strings.Contains(body, "vg_http_requests_total") {
			t.Errorf("public listener must not expose %s (got %d)", path, code)
		}
	}
	if code, _ := get(t, "http://"+p.internal+"/metrics", ""); code != 401 {
		t.Errorf("internal /metrics without token must be 401, got %d", code)
	}
	if code, _ := get(t, "http://"+p.internal+"/debug/pprof/", "Bearer "+tok); code == 200 {
		t.Errorf("pprof must not be exposed even internally, got %d", code)
	}
	code, body := get(t, "http://"+p.internal+"/metrics", "Bearer "+tok)
	if code != 200 || !strings.Contains(body, "vg_message_delivery_latency_seconds") || !strings.Contains(body, "vg_http_requests_total") {
		t.Fatalf("authenticated /metrics must expose the platform metrics, got %d", code)
	}
}

func TestMetricsDisabledWithoutToken(t *testing.T) {
	p := start(t)
	if code, _ := get(t, "http://"+p.internal+"/metrics", "Bearer anything"); code != 404 {
		t.Fatalf("without VG_METRICS_TOKEN /metrics must be disabled (404), got %d", code)
	}
}

func TestWeakMetricsTokenIsRejected(t *testing.T) {
	bin := build(t)
	if _, se, code := runBin(t, bin, []string{"VG_ENV=production", "VG_METRICS_TOKEN=short"}, "-check-config"); code != exitConfig || !strings.Contains(se, "VG_METRICS_TOKEN") {
		t.Fatalf("weak metrics token must fail config (got %d %q)", code, se)
	}
}

func TestRequestLogsAreJSONWithTraceAndNoInjectedSecrets(t *testing.T) {
	p := start(t)
	req, _ := http.NewRequest("GET", "http://"+p.pub+"/v1/anything?token=QUERY-SECRET-123&phone=%2B8613800138000", nil)
	req.Header.Set("Authorization", "Bearer HEADER-SECRET-TOKEN-123")
	req.Header.Set("X-Request-Id", "feedfacecafe0042")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	time.Sleep(200 * time.Millisecond)
	out := p.stderr.String()
	if !strings.Contains(out, `"trace_id":"feedfacecafe0042"`) || !strings.Contains(out, `"msg":"http_request"`) {
		t.Fatalf("expected a JSON access log with the trace id, got:\n%s", out)
	}
	for _, leak := range []string{"QUERY-SECRET", "HEADER-SECRET", "13800138000"} {
		if strings.Contains(out, leak) {
			t.Errorf("log leaks %q", leak)
		}
	}
}

// ---- Story 1.6: IM callback listener exposure (black-box) ----

func post(t *testing.T, url, ct, body string) int {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestIMCallbackListenerRejectsWrongSecretAndFailsClosed(t *testing.T) {
	const secret = "callback-secret-0123456789abcdef0123456789"
	hook := freeAddr(t)
	p := start(t, "VG_IM_WEBHOOK_SECRET="+secret, "VG_IM_WEBHOOK_ADDR="+hook, "VG_METRICS_TOKEN=metrics-token-0123456789abcdef")
	body := `{"from_uid":"u1","channel_id":"u2@u1","channel_type":1,"client_msg_no":"c1","payload":"aGk="}`
	if c := post(t, "http://"+hook+"/im/webhook/wrong?event=msg.before_send", "application/json", body); c != 403 {
		t.Fatalf("wrong secret must be 403, got %d", c)
	}
	if c := post(t, "http://"+hook+"/im/webhook/?event=msg.before_send", "application/json", body); c != 403 {
		t.Fatalf("missing secret must be 403, got %d", c)
	}
	resp, err := http.Post("http://"+hook+"/im/webhook/"+secret+"?event=msg.before_send", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(b), `"allow":false`) {
		t.Fatalf("with no policy installed the gate must deny (fail-closed), got %d %s", resp.StatusCode, b)
	}
	// the callback path must not exist on the public or the health/metrics listeners
	for _, base := range []string{"http://" + p.pub, "http://" + p.internal} {
		if c := post(t, base+"/im/webhook/"+secret+"?event=msg.before_send", "application/json", body); c == 200 {
			t.Errorf("%s must not serve the IM callback", base)
		}
	}
	// rejections are counted for alerting
	_, m := get(t, "http://"+p.internal+"/metrics", "Bearer metrics-token-0123456789abcdef")
	if !strings.Contains(m, `vg_im_webhook_rejected_total{reason="bad_secret"} 2`) {
		t.Fatalf("rejections must be exported as a metric:\n%s", m)
	}
}

func TestIMCallbackListenerDisabledWithoutSecretAndConfigGuards(t *testing.T) {
	hook := freeAddr(t)
	start(t, "VG_IM_WEBHOOK_ADDR="+hook)
	if resp, err := (&http.Client{Timeout: time.Second}).Get("http://" + hook + "/"); err == nil {
		resp.Body.Close()
		t.Fatal("without VG_IM_WEBHOOK_SECRET no callback listener may be started")
	}
	bin := build(t)
	for _, env := range [][]string{
		{"VG_ENV=production", "VG_IM_WEBHOOK_SECRET=short"},
		{"VG_ENV=production", "VG_IM_WEBHOOK_ADDR=0.0.0.0:9102"},
		{"VG_ENV=production", "VG_IM_API_URL=http://8.8.8.8:5001"},
	} {
		if _, se, code := runBin(t, bin, env, "-check-config"); code != exitConfig || !strings.Contains(se, "VG_IM_") {
			t.Errorf("%v must fail config validation, got %d %q", env, code, se)
		}
	}
	if _, _, code := runBin(t, bin, []string{"VG_ENV=development", "VG_IM_API_URL=http://8.8.8.8:5001"}, "-version"); code != 0 {
		t.Error("sanity: -version needs no config")
	}
}

// ---- Story 1.4: worker and audit/outbox operations against the real binary ----

func migrated(t *testing.T) (*sql.DB, string, string) {
	t.Helper()
	d, dsn := testdb.NewWithDSN(t)
	bin := build(t)
	if _, se, code := runBin(t, bin, []string{"VG_ENV=development", "VG_MYSQL_DSN=" + dsn}, "migrate", "up"); code != 0 {
		t.Fatalf("migrate: %d %s", code, se)
	}
	return d, dsn, bin
}

func TestAuditVerifyAndAnchorCommands(t *testing.T) {
	d, dsn, bin := migrated(t)
	env := []string{"VG_ENV=development", "VG_MYSQL_DSN=" + dsn}
	for i := 0; i < 3; i++ {
		if err := (&audit.Log{DB: d}).Append(context.Background(), audit.Entry{Actor: "a", Action: "x", Target: "t"}); err != nil {
			t.Fatal(err)
		}
	}
	if so, se, code := runBin(t, bin, env, "audit", "verify"); code != 0 || !strings.Contains(so, "audit chain ok rows=3") {
		t.Fatalf("verify: %d %q %q", code, so, se)
	}
	var put, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		put, auth = r.Method+" "+r.URL.Path, r.Header.Get("Authorization")
	}))
	defer srv.Close()
	if so, se, code := runBin(t, bin, append(env, "VG_AUDIT_ANCHOR_URL="+srv.URL+"/b", "VG_AUDIT_ANCHOR_TOKEN=tok"), "audit", "anchor"); code != 0 || !strings.Contains(so, "anchored rows=3") {
		t.Fatalf("anchor: %d %q %q", code, so, se)
	}
	if !strings.HasPrefix(put, "PUT /b/audit-anchor/") || auth != "Bearer tok" {
		t.Fatalf("anchor request: %q %q", put, auth)
	}
	if _, _, code := runBin(t, bin, env, "audit", "anchor"); code != exitConfig {
		t.Fatalf("anchor without a store must be a config error, got %d", code)
	}
	if _, _, code := runBin(t, bin, []string{"VG_ENV=production", "VG_AUDIT_ANCHOR_URL=http://insecure.example.com"}, "-check-config"); code != exitConfig {
		t.Fatal("production must refuse a non-https anchor store")
	}
	d.Exec("UPDATE audit_log SET actor='mallory' WHERE id=2")
	if _, se, code := runBin(t, bin, env, "audit", "verify"); code != 1 || !strings.Contains(se, "verification failed") {
		t.Fatalf("tampering must make verify fail: %d %q", code, se)
	}
}

func TestWorkerProcessesEventsAndReplayIsAudited(t *testing.T) {
	d, dsn, bin := migrated(t)
	env := []string{"VG_ENV=development", "VG_MYSQL_DSN=" + dsn}
	// an event for a topic nobody handles dead-letters once attempts are exhausted; fast-forward to the final attempt
	id, err := outbox.Enqueue(context.Background(), d, time.Now(), 1, "unhandled.topic", "k", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	d.Exec("UPDATE outbox SET attempts=4 WHERE event_id=?", id)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin, "worker")
	cmd.Env = baseEnv(env...)
	buf := &syncBuf{}
	cmd.Stderr = buf
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); _ = cmd.Wait() }()
	var dl uint64
	for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if d.QueryRow("SELECT id FROM dead_letter WHERE event_id=?", id).Scan(&dl) == nil {
			break
		}
	}
	if dl == 0 {
		t.Fatalf("the worker process must dead-letter the exhausted event; log:\n%s", buf.String())
	}
	if _, se, code := runBin(t, bin, env, "outbox", "replay", fmt.Sprint(dl)); code != 0 {
		t.Fatalf("replay: %d %s", code, se)
	}
	var n int
	d.QueryRow("SELECT COUNT(*) FROM audit_log WHERE action='outbox.replay' AND target=?", fmt.Sprintf("dead_letter:%d", dl)).Scan(&n)
	if n != 1 {
		t.Fatalf("a replay must leave an audit entry, found %d", n)
	}
	if _, _, code := runBin(t, bin, env, "outbox", "replay", fmt.Sprint(dl)); code != 1 {
		t.Fatal("replaying twice must fail")
	}
	if _, _, code := runBin(t, bin, env, "outbox", "replay", "abc"); code != 2 {
		t.Fatal("bad id must be a usage error")
	}
}

func TestOutboxBacklogIsExportedAsMetric(t *testing.T) {
	d, dsn, _ := migrated(t)
	for i := 0; i < 3; i++ {
		outbox.Enqueue(context.Background(), d, time.Now(), 1, "t", "k", []byte("x"))
	}
	p := start(t, "VG_MYSQL_DSN="+dsn, "VG_METRICS_TOKEN=metrics-token-0123456789abcdef")
	_, m := get(t, "http://"+p.internal+"/metrics", "Bearer metrics-token-0123456789abcdef")
	if !strings.Contains(m, "vg_outbox_pending 3") {
		t.Fatalf("vg_outbox_pending must reflect the queue:\n%s", m)
	}
}

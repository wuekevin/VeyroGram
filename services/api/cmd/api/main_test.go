package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"veyrogram/api/internal/platform/audit"
	"veyrogram/api/internal/platform/denylist"
	"veyrogram/api/internal/platform/outbox"
	"veyrogram/api/internal/platform/testdb"
	"veyrogram/api/internal/platform/testim"
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
	if so, se, code := runBin(t, bin, env, "migrate", "up"); code != 0 || !strings.Contains(so, "applied: [1 2 3 4]") {
		t.Fatalf("up failed: %d %q %q", code, so, se)
	}
	if so, _, code := runBin(t, bin, env, "migrate", "status"); code != 0 || !strings.Contains(so, "applied=[1 2 3 4] pending=[] dirty=false") {
		t.Fatalf("status after up: %d %q", code, so)
	}
	if so, _, code := runBin(t, bin, env, "migrate", "up"); code != 0 || !strings.Contains(so, "applied: []") {
		t.Fatalf("second up must be a no-op: %d %q", code, so)
	}
	if so, _, code := runBin(t, bin, env, "migrate", "down"); code != 0 || !strings.Contains(so, "reverted: [4]") {
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
	workerAddr := freeAddr(t)
	const token = "worker-metrics-token-0123456789"
	env := []string{"VG_ENV=development", "VG_MYSQL_DSN=" + dsn, "VG_WORKER_INTERNAL_ADDR=" + workerAddr, "VG_METRICS_TOKEN=" + token}
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
	// the worker process exports its own metrics on its own internal listener, behind the bearer token
	if code, _ := get(t, "http://"+workerAddr+"/metrics", ""); code == 200 {
		t.Fatal("worker /metrics must require the bearer token")
	}
	code, body := get(t, "http://"+workerAddr+"/metrics", "Bearer "+token)
	if code != 200 || !strings.Contains(body, `vg_outbox_events_total{topic="unhandled.topic",result="dead"} 1`) {
		t.Fatalf("worker metrics must count the dead-lettered event: %d\n%s", code, body)
	}
	if code, _ := get(t, "http://"+workerAddr+"/readyz", ""); code != 200 {
		t.Fatalf("worker /readyz should be ready with MySQL up, got %d", code)
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

func TestAuditCheckAnchorsCommandAgainstAnAnchorStore(t *testing.T) {
	d, dsn, bin := migrated(t)
	store := map[string][]byte{}
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			store[r.URL.Path] = b
		case http.MethodGet:
			if b, ok := store[r.URL.Path]; ok {
				w.Write(b)
				return
			}
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	env := []string{"VG_ENV=development", "VG_MYSQL_DSN=" + dsn, "VG_AUDIT_ANCHOR_URL=" + srv.URL + "/b"}
	for i := 0; i < 4; i++ {
		if err := (&audit.Log{DB: d}).Append(context.Background(), audit.Entry{Actor: "a", Action: "x", Target: "t"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, code := runBin(t, bin, env, "audit", "check-anchors"); code != 1 {
		t.Fatalf("with no anchors at all the check must fail (nothing was verified), got %d", code)
	}
	if _, se, code := runBin(t, bin, env, "audit", "anchor"); code != 0 {
		t.Fatal(se)
	}
	if so, se, code := runBin(t, bin, env, "audit", "check-anchors", "-days", "3"); code != 0 || !strings.Contains(so, "checked=1 missing=2") {
		t.Fatalf("check: %d %q %q", code, so, se)
	}
	if _, _, code := runBin(t, bin, env, "audit", "check-anchors", "-days", "3", "-require-all"); code != 1 {
		t.Fatal("-require-all must fail when days are missing")
	}
	d.Exec("DELETE FROM audit_log WHERE id > 2") // tail truncation: the chain still verifies by itself
	if _, se, code := runBin(t, bin, env, "audit", "verify"); code != 0 {
		t.Fatalf("premise: a truncated chain verifies alone: %q", se)
	}
	if _, se, code := runBin(t, bin, env, "audit", "check-anchors"); code != 1 || !strings.Contains(se, "anchor") {
		t.Fatalf("tail truncation must be detected against the off-host anchor: %d %q", code, se)
	}
}

// ---- Story 1.12: phone/SMS auth against the real binary ----

const authSeed = "AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyA"

func authEnv(dsn string, extra ...string) []string {
	return append([]string{"VG_MYSQL_DSN=" + dsn, "VG_AUTH_SMS_LOGIN=true", "VG_SMS_PROVIDER=log", "VG_TOKEN_KEYS=k1:" + authSeed,
		"VG_PHONE_HASH_KEY=phone-hash-key-0123456789abcdef-strong", "VG_IM_TOKEN_KEY=im-token-key-0123456789abcdef-strong-x"}, extra...)
}

func jpost(t *testing.T, url, body string, hdr ...string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return resp.StatusCode, m
}

func TestAuthRegisterLoginRefreshOverRealHTTP(t *testing.T) {
	d, dsn, _ := migrated(t)
	p := start(t, authEnv(dsn)...)
	base := "http://" + p.pub
	const dev = `"device_id":"device-aaaaaaaaaaaaaaaa"`
	if code, _ := jpost(t, base+"/v1/auth/sms/code", `{"phone":"+8613800138000",`+dev+`}`); code != 202 {
		t.Fatalf("send code: %d", code)
	}
	m := regexp.MustCompile(`验证码 (\d{6})`).FindStringSubmatch(p.stderr.String())
	if m == nil {
		t.Fatalf("dev log provider did not print a code:\n%s", p.stderr.String())
	}
	if strings.Contains(p.stderr.String(), "13800138000") {
		t.Fatal("the full phone number must never reach the logs")
	}
	login := `{"phone":"+8613800138000","code":"` + m[1] + `","device":{"id":"device-aaaaaaaaaaaaaaaa","class":"app","name":"Pixel"}}`
	// an Idempotency-Key on a credential-bearing endpoint must not make the server store the tokens
	code, body := jpost(t, base+"/v1/auth/login", login, "Idempotency-Key", "idem-key-0123456789")
	if code != 200 || body["access_token"] == nil || body["refresh_token"] == nil {
		t.Fatalf("login: %d %v", code, body)
	}
	var n int
	if err := d.QueryRow("SELECT COUNT(*) FROM idempotency_key").Scan(&n); err != nil || n != 0 {
		t.Fatalf("auth responses must not enter the idempotency cache (rows=%d err=%v)", n, err)
	}
	code, body2 := jpost(t, base+"/v1/auth/refresh", `{"refresh_token":"`+body["refresh_token"].(string)+`"}`)
	if code != 200 || body2["refresh_token"] == body["refresh_token"] {
		t.Fatalf("refresh: %d %v", code, body2)
	}
	if code, _ := jpost(t, base+"/v1/auth/nope", `{}`); code != 404 {
		t.Fatalf("unknown auth path: %d", code)
	}
	if c, mt := get(t, "http://"+p.internal+"/metrics", ""); c == 200 && strings.Contains(mt, "vg_auth_events_total") {
		t.Fatal("metrics must need the bearer token")
	}
}

func TestAuthRoutesAbsentWhenFlagOffAndStartupGuards(t *testing.T) {
	_, dsn, bin := migrated(t)
	p := start(t, "VG_MYSQL_DSN="+dsn)
	if code, _ := jpost(t, "http://"+p.pub+"/v1/auth/login", `{}`); code != 404 {
		t.Fatalf("auth.sms_login off: routes must not exist, got %d", code)
	}
	prod := []string{"VG_ENV=production", "VG_MYSQL_DSN=vg_app:s3cret-pass@tcp(127.0.0.1:3306)/x", "VG_TOKEN_SIGNING_KEY=0123456789abcdef0123456789abcdef-strong",
		"VG_AUTH_SMS_LOGIN=true", "VG_SMS_PROVIDER=log", "VG_TOKEN_KEYS=k1:" + authSeed, "VG_PHONE_HASH_KEY=phone-hash-key-0123456789abcdef-strong", "VG_IM_TOKEN_KEY=im-token-key-0123456789abcdef-strong-x"}
	if so, se, code := runBin(t, bin, prod, "-check-config"); code == 0 || !strings.Contains(se+so, "VG_SMS_PROVIDER") {
		t.Fatalf("production with the log SMS provider must be refused: %d %s%s", code, so, se)
	}
}

// ---- Story 1.13: logout/revocation through the real binary and a real Redis deny-list ----

func TestLogoutRevokesAccessTokenViaRedisDenyList(t *testing.T) {
	redisAddr := os.Getenv("VG_TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("VG_TEST_REDIS_ADDR not set")
	}
	_, dsn, _ := migrated(t)
	// fresh databases restart device ids at 1, so entries left by an earlier run in the shared test Redis would deny them
	clean := func() {
		if r, err := denylist.Open(context.Background(), redisAddr, ""); err == nil {
			defer r.Close()
			r.C.Del(context.Background(), "vg:deny:dev:1:1", "vg:deny:dev:1:2")
		}
	}
	clean()
	t.Cleanup(clean)
	p := start(t, authEnv(dsn, "VG_REDIS_ADDR="+redisAddr, "VG_METRICS_TOKEN=metrics-token-0123456789abcdef")...)
	base := "http://" + p.pub
	jpost(t, base+"/v1/auth/sms/code", `{"phone":"+8613800138001","device_id":"device-aaaaaaaaaaaaaaaa"}`)
	m := regexp.MustCompile(`验证码 (\d{6})`).FindStringSubmatch(p.stderr.String())
	if m == nil {
		t.Fatal("no code")
	}
	_, body := jpost(t, base+"/v1/auth/login", `{"phone":"+8613800138001","code":"`+m[1]+`","device":{"id":"device-aaaaaaaaaaaaaaaa","class":"app"}}`)
	access, _ := body["access_token"].(string)
	if access == "" {
		t.Fatalf("login: %v", body)
	}
	if c, b := get(t, base+"/v1/devices", "Bearer "+access); c != 200 || !strings.Contains(b, `"current":true`) {
		t.Fatalf("devices: %d %s", c, b)
	}
	if c, _ := get(t, base+"/v1/devices", ""); c != 401 {
		t.Fatalf("no token: %d", c)
	}
	if c, _ := jpost(t, base+"/v1/auth/logout", `{}`, "Authorization", "Bearer "+access); c != 204 {
		t.Fatalf("logout: %d", c)
	}
	if c, _ := get(t, base+"/v1/devices", "Bearer "+access); c != 401 {
		t.Fatalf("a logged-out access token must be refused at once, got %d", c)
	}
	// the IM is not running in this test: the revocation is queued for the worker, not lost
	if code, mt := get(t, "http://"+p.internal+"/metrics", "Bearer metrics-token-0123456789abcdef"); code != 200 || !strings.Contains(mt, `vg_auth_events_total{kind="device",result="revoked"} 1`) {
		t.Fatalf("revocation must be counted: %d\n%s", code, mt)
	}
}

func TestStaleRedisAddressDegradesButServes(t *testing.T) {
	_, dsn, _ := migrated(t)
	p := start(t, authEnv(dsn, "VG_REDIS_ADDR=127.0.0.1:1", "VG_METRICS_TOKEN=metrics-token-0123456789abcdef")...)
	base := "http://" + p.pub
	jpost(t, base+"/v1/auth/sms/code", `{"phone":"+8613800138002","device_id":"device-aaaaaaaaaaaaaaaa"}`)
	m := regexp.MustCompile(`验证码 (\d{6})`).FindStringSubmatch(p.stderr.String())
	_, body := jpost(t, base+"/v1/auth/login", `{"phone":"+8613800138002","code":"`+m[1]+`","device":{"id":"device-aaaaaaaaaaaaaaaa","class":"app"}}`)
	access, _ := body["access_token"].(string)
	if c, _ := get(t, base+"/v1/devices", "Bearer "+access); c != 200 {
		t.Fatalf("with Redis down valid tokens are still served: %d", c)
	}
	if _, mt := get(t, "http://"+p.internal+"/metrics", "Bearer metrics-token-0123456789abcdef"); !strings.Contains(mt, "vg_auth_denylist_degraded_total 1") {
		t.Fatalf("degradation must be visible in metrics:\n%s", mt)
	}
}

// Real API + real worker + real (patched) WuKongIM: the IM token is registered, and logout's queued IM sync is carried out against the real IM.
func TestIMTokenAndLogoutSyncAgainstRealWuKongIM(t *testing.T) {
	redisAddr := os.Getenv("VG_TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("VG_TEST_REDIS_ADDR not set")
	}
	im := testim.Start(t, "") // skips when VG_TEST_WUKONGIM_BIN is not set
	d, dsn, bin := migrated(t)
	extra := []string{"VG_REDIS_ADDR=" + redisAddr, "VG_IM_API_URL=" + im.APIURL, "VG_METRICS_TOKEN=metrics-token-0123456789abcdef"}
	if r, err := denylist.Open(context.Background(), redisAddr, ""); err == nil {
		r.C.Del(context.Background(), "vg:deny:dev:1:1", "vg:deny:dev:1:2")
		r.Close()
	}
	p := start(t, authEnv(dsn, extra...)...)
	base := "http://" + p.pub
	jpost(t, base+"/v1/auth/sms/code", `{"phone":"+8613800138003","device_id":"device-aaaaaaaaaaaaaaaa"}`)
	m := regexp.MustCompile(`验证码 (\d{6})`).FindStringSubmatch(p.stderr.String())
	_, body := jpost(t, base+"/v1/auth/login", `{"phone":"+8613800138003","code":"`+m[1]+`","device":{"id":"device-aaaaaaaaaaaaaaaa","class":"web"}}`)
	access, _ := body["access_token"].(string)
	c, imBody := get(t, base+"/v1/im/token", "Bearer "+access)
	if c != 200 || !strings.Contains(imBody, `"device_flag":1`) || !strings.Contains(imBody, `"token":"`) {
		t.Fatalf("the real IM must accept the token registration: %d %s", c, imBody)
	}
	if c, _ := jpost(t, base+"/v1/auth/logout", `{}`, "Authorization", "Bearer "+access); c != 204 {
		t.Fatalf("logout: %d", c)
	}
	buf := startWorker(t, bin, dsn, extra)
	var status string
	for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if d.QueryRow("SELECT status FROM outbox WHERE topic='identity.im_sync'").Scan(&status) == nil && status == "done" {
			break
		}
	}
	if status != "done" {
		t.Fatalf("the worker must complete the IM sync against the real IM (status=%q); log:\n%s", status, buf.String())
	}
	var epoch int
	if err := d.QueryRow("SELECT epoch FROM im_epoch").Scan(&epoch); err != nil || epoch != 1 {
		t.Fatalf("the class epoch must have been bumped once: %d %v", epoch, err)
	}
}

// startWorker runs the real worker binary until the test ends and returns its log.
func startWorker(t *testing.T, bin, dsn string, extra []string) *syncBuf {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin, "worker")
	cmd.Env = baseEnv(append(authEnv(dsn, extra...), "VG_ENV=development", "VG_WORKER_INTERNAL_ADDR="+freeAddr(t))...)
	buf := &syncBuf{}
	cmd.Stderr = buf
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = cmd.Wait() })
	return buf
}

func eventually(t *testing.T, what string, within time.Duration, f func() bool) {
	t.Helper()
	for deadline := time.Now().Add(within); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if f() {
			return
		}
	}
	t.Fatalf("timed out waiting for: %s", what)
}

// The 1.13 end-to-end proof with real clients: a revoked device's session is cut on the wire, a surviving same-class device reconnects with a
// freshly fetched IM token (the old one is refused), a device of another class is told about it by a system notice, and logout-all ends everything.
func TestRevocationKickReconnectAndNoticeOverRealWebSocket(t *testing.T) {
	redisAddr := os.Getenv("VG_TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("VG_TEST_REDIS_ADDR not set")
	}
	im := testim.Start(t, "")
	d, dsn, bin := migrated(t)
	extra := []string{"VG_REDIS_ADDR=" + redisAddr, "VG_IM_API_URL=" + im.APIURL, "VG_METRICS_TOKEN=metrics-token-0123456789abcdef"}
	if r, err := denylist.Open(context.Background(), redisAddr, ""); err == nil {
		r.C.Del(context.Background(), "vg:deny:dev:1:1", "vg:deny:dev:1:2", "vg:deny:dev:1:3")
		r.Close()
	}
	p := start(t, authEnv(dsn, extra...)...)
	wlog := startWorker(t, bin, dsn, extra)
	base := "http://" + p.pub
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d", im.Ports["ws"])
	phone := "+8613800138009"

	login := func(device, class string) (access, refresh, devID string) {
		t.Helper()
		_, _ = d.Exec("UPDATE login_event SET created_at = created_at - INTERVAL 5 MINUTE") // step over the 60 s resend cooldown
		before := len(regexp.MustCompile(`验证码 (\d{6})`).FindAllStringSubmatch(p.stderr.String(), -1))
		jpost(t, base+"/v1/auth/sms/code", `{"phone":"`+phone+`","device_id":"`+device+`"}`)
		var code string
		eventually(t, "sms code in log", 3*time.Second, func() bool {
			all := regexp.MustCompile(`验证码 (\d{6})`).FindAllStringSubmatch(p.stderr.String(), -1)
			if len(all) > before {
				code = all[len(all)-1][1]
				return true
			}
			return false
		})
		c, body := jpost(t, base+"/v1/auth/login", `{"phone":"`+phone+`","code":"`+code+`","device":{"id":"`+device+`","class":"`+class+`"}}`)
		if c != 200 {
			t.Fatalf("login %s: %d %v", device, c, body)
		}
		access, _ = body["access_token"].(string)
		refresh, _ = body["refresh_token"].(string)
		devID = fmt.Sprint(body["device_id"])
		return
	}
	imToken := func(access string) (uid, tok string, code int) {
		c, b := get(t, base+"/v1/im/token", "Bearer "+access)
		var o struct {
			UID   string `json:"uid"`
			Token string `json:"token"`
		}
		_ = json.Unmarshal([]byte(b), &o)
		return o.UID, o.Token, c
	}

	accA, _, _ := login("device-aaaaaaaaaaaaaaaa", "web")
	_, _, devB := login("device-bbbbbbbbbbbbbbbb", "web")
	accC, _, _ := login("device-cccccccccccccccc", "app")

	uid, tokA, c := imToken(accA)
	if c != 200 || tokA == "" {
		t.Fatalf("im token A: %d", c)
	}
	_, tokC, _ := imToken(accC)

	wsA := dialWS(t, wsURL)
	if !wsA.connect(uid, tokA, "im-a", 1) {
		t.Fatal("web device A must be able to connect with its IM token")
	}
	wsC := dialWS(t, wsURL)
	if !wsC.connect(uid, tokC, "im-c", 0) {
		t.Fatal("app device C must be able to connect with its IM token")
	}

	// A revokes B (same class). The worker bumps the web epoch, kicks the web sessions and re-registers the new token.
	req, _ := http.NewRequest("DELETE", base+"/v1/devices/"+devB, nil)
	req.Header.Set("Authorization", "Bearer "+accA)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 204 {
		t.Fatalf("revoke B: %v %v", err, resp)
	}
	resp.Body.Close()

	if !wsA.kicked(20 * time.Second) {
		t.Fatal("A's web session must be cut by the revocation of a same-class device")
	}
	from, notice, ok := wsC.recvNotice(20 * time.Second)
	if !ok {
		var st, last string
		_ = d.QueryRow("SELECT status, COALESCE(last_error,'') FROM outbox WHERE topic='identity.device_notice'").Scan(&st, &last)
		t.Logf("notice outbox status=%q last_error=%q\nworker log:\n%s", st, last, wlog.String())
	}
	if !ok || from != "vg_system" || notice["type"] != "account_notice" || notice["kind"] != "device_revoked" || notice["device_id"] != devB {
		t.Fatalf("the app device must receive the system notice: ok=%v from=%q notice=%v", ok, from, notice)
	}

	// The kicked client reconnects: the old token is refused, a freshly fetched one works.
	if retry := dialWS(t, wsURL); retry.connect(uid, tokA, "im-a", 1) {
		t.Fatal("the pre-revocation IM token must no longer be accepted")
	}
	_, tokA2, c := imToken(accA)
	if c != 200 || tokA2 == tokA {
		t.Fatalf("a new IM token is expected after the epoch bump: %d same=%v", c, tokA2 == tokA)
	}
	wsA2 := dialWS(t, wsURL)
	if !wsA2.connect(uid, tokA2, "im-a", 1) {
		t.Fatal("A must reconnect with the freshly fetched token")
	}

	// Logout everywhere: sessions of every class end, tokens are dead, nothing reconnects.
	if c, b := jpost(t, base+"/v1/auth/logout-all", `{}`, "Authorization", "Bearer "+accA); c != 200 || b["revoked_devices"] != float64(2) {
		t.Fatalf("logout-all: %d %v (A and C were the live devices)", c, b)
	}
	if !wsA2.kicked(20*time.Second) || !wsC.kicked(20*time.Second) {
		t.Fatal("logout-all must cut every IM session")
	}
	for name, acc := range map[string]string{"A": accA, "C": accC} {
		if c, _ := get(t, base+"/v1/devices", "Bearer "+acc); c != 401 {
			t.Fatalf("access token %s must be dead after logout-all: %d", name, c)
		}
	}
	if retry := dialWS(t, wsURL); retry.connect(uid, tokA2, "im-a", 1) {
		t.Fatal("no IM token issued before logout-all may connect afterwards")
	}
	if retry := dialWS(t, wsURL); retry.connect(uid, tokC, "im-c", 0) {
		t.Fatal("the app device's IM token must be dead too")
	}
	var live int
	if err := d.QueryRow("SELECT COUNT(*) FROM device WHERE revoked_at IS NULL").Scan(&live); err != nil || live != 0 {
		t.Fatalf("no live device may remain: %d %v", live, err)
	}
}

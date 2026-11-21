package main

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Story 1.21 ops test: the health probe reports each part, exits non-zero when any is down, and writes metrics a stopped probe cannot fake.
func TestProbeReportsEachTargetAndExitCode(t *testing.T) {
	apiStatus := 200
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/readyz" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(apiStatus)
	}))
	defer api.Close()
	im := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			return
		}
		http.NotFound(w, r)
	}))
	defer im.Close()
	wl, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer wl.Close()
	canaryOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer canaryOK.Close()

	env := map[string]string{"VG_ENV": "development", "VG_INTERNAL_ADDR": strings.TrimPrefix(api.URL, "http://"), "VG_WORKER_INTERNAL_ADDR": wl.Addr().String(), "VG_IM_API_URL": im.URL}
	getenv := func(k string) string { return env[k] }
	dir := t.TempDir()
	tf := filepath.Join(dir, "veyrogram_probe.prom")
	run := func() (string, int) {
		var out, errb bytes.Buffer
		code := runProbe([]string{"-textfile", tf}, getenv, &out, &errb)
		return out.String(), code
	}

	out, code := run()
	if code != 0 || !strings.Contains(out, "ok     api") || !strings.Contains(out, "ok     worker") || !strings.Contains(out, "ok     im") {
		t.Fatalf("all healthy: %d\n%s", code, out)
	}
	b, err := os.ReadFile(tf)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`veyrogram_probe_up{target="api"} 1`, `veyrogram_probe_up{target="worker"} 1`, `veyrogram_probe_up{target="im"} 1`, "veyrogram_probe_timestamp_seconds "} {
		if !strings.Contains(string(b), want) {
			t.Errorf("textfile lacks %q:\n%s", want, b)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".probe.*")); len(left) != 0 {
		t.Errorf("temporary files left behind: %v", left)
	}

	// readiness failing (say MySQL is unreachable behind /readyz) is DOWN even though the process answers
	apiStatus = 503
	out, code = run()
	b, _ = os.ReadFile(tf)
	if code != 1 || !strings.Contains(out, "DOWN   api") || !strings.Contains(string(b), `veyrogram_probe_up{target="api"} 0`) || !strings.Contains(string(b), `veyrogram_probe_up{target="im"} 1`) {
		t.Fatalf("503 from /readyz must mark only api down: %d\n%s\n%s", code, out, b)
	}
	apiStatus = 200

	// worker listener gone
	wl.Close()
	if out, code = run(); code != 1 || !strings.Contains(out, "DOWN   worker") {
		t.Fatalf("closed worker port: %d\n%s", code, out)
	}

	// optional canary target
	env["VG_PROBE_CANARY_URL"] = canaryOK.URL
	wl2, _ := net.Listen("tcp", wl.Addr().String())
	if wl2 != nil {
		defer wl2.Close()
	}
	if out, _ = run(); !strings.Contains(out, "canary") {
		t.Errorf("canary target missing when configured:\n%s", out)
	}
}

func TestProbeWithNothingRunningFailsAndWritesNoSecrets(t *testing.T) {
	bin := build(t)
	closed := freeAddr(t)
	tf := filepath.Join(t.TempDir(), "p.prom")
	so, _, code := runBin(t, bin, []string{"VG_ENV=development", "VG_INTERNAL_ADDR=" + closed, "VG_WORKER_INTERNAL_ADDR=" + freeAddr(t), "VG_IM_API_URL=http://" + freeAddr(t),
		"VG_METRICS_TOKEN=metrics-token-0123456789abcdef", "VG_MYSQL_DSN=u:Sup3r-secret-pw@tcp(127.0.0.1:1)/d"}, "probe", "-textfile", tf)
	if code != 1 || strings.Count(so, "DOWN") != 3 {
		t.Fatalf("nothing running: %d\n%s", code, so)
	}
	b, _ := os.ReadFile(tf)
	if strings.Contains(so+string(b), "Sup3r-secret-pw") || strings.Contains(so+string(b), "metrics-token") {
		t.Error("the probe printed a secret")
	}
	if strings.Count(string(b), "} 0") != 3 {
		t.Errorf("textfile should show three targets down:\n%s", b)
	}
}

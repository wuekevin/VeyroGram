// Package testim starts a real WuKongIM (the pinned build) for contract and integration tests. Set VG_TEST_WUKONGIM_BIN to the binary built by
// tools/build_wukongim.sh; without it the tests are skipped, never faked. Every listener binds loopback on a free port and uses a temp data dir.
package testim

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// IM is one running instance.
type IM struct {
	APIURL string
	Ports  map[string]int // api, manager, cluster, tcp, ws
	t      testing.TB
	bin    string
	cfg    string
	cmd    *exec.Cmd
	cancel context.CancelFunc
	logs   *lockedBuf
	done   chan struct{}
}

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *lockedBuf) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

func freePort(t testing.TB) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func findExample() (string, error) {
	dir, _ := os.Getwd()
	for i := 0; i < 8; i++ {
		p := filepath.Join(dir, "upstream", "WuKongIM", "wukongim.toml.example")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		dir = filepath.Dir(dir)
	}
	return "", fmt.Errorf("upstream/WuKongIM/wukongim.toml.example not found above the test directory")
}

// setKey replaces `key = ...` inside [section] (first occurrence); it fails the test if the key is absent so config drift is noticed.
func setKey(t testing.TB, conf, section, key, value string) string {
	t.Helper()
	lines := strings.Split(conf, "\n")
	in := false
	re := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(key) + `\s*=`)
	for i, l := range lines {
		if s := strings.TrimSpace(l); strings.HasPrefix(s, "[") {
			in = s == "["+section+"]"
			continue
		}
		if in && re.MatchString(l) {
			lines[i] = key + " = " + value
			return strings.Join(lines, "\n")
		}
	}
	t.Fatalf("testim: key %s.%s not found in the example config (upstream layout changed?)", section, key)
	return ""
}

// Start launches WuKongIM with before_send disabled (pass beforeSendURL to enable the synchronous webhook).
func Start(t testing.TB, beforeSendURL string) *IM {
	t.Helper()
	bin := os.Getenv("VG_TEST_WUKONGIM_BIN")
	if bin == "" {
		t.Skip("VG_TEST_WUKONGIM_BIN not set: skipping WuKongIM contract test (build it with tools/build_wukongim.sh)")
	}
	ex, err := findExample()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(ex)
	dir := t.TempDir()
	conf := strings.ReplaceAll(string(raw), "./data/wukongim-single-node-data", filepath.Join(dir, "data"))
	api, cl, mg, tcp, ws := freePort(t), freePort(t), freePort(t), freePort(t), freePort(t)
	conf = strings.ReplaceAll(conf, "127.0.0.1:7001", fmt.Sprintf("127.0.0.1:%d", cl))
	conf = setKey(t, conf, "api", "listen_addr", fmt.Sprintf(`"127.0.0.1:%d"`, api))
	conf = setKey(t, conf, "api", "external_tcp_addr", fmt.Sprintf(`"127.0.0.1:%d"`, tcp))
	conf = setKey(t, conf, "api", "external_ws_addr", fmt.Sprintf(`"ws://127.0.0.1:%d"`, ws))
	conf = setKey(t, conf, "manager", "listen_addr", fmt.Sprintf(`"127.0.0.1:%d"`, mg))
	conf = setKey(t, conf, "manager", "jwt_secret", `"test-only-manager-secret-0123456789abcdef"`)
	conf = setKey(t, conf, "cluster", "join_token", `"test-only-join-token-0123456789abcdef"`)
	conf = setKey(t, conf, "gateway", "listeners", fmt.Sprintf(`[{ name = "tcp-wkproto", network = "tcp", address = "127.0.0.1:%d", transport = "gnet", protocol = "wkproto" }, { name = "ws-gateway", network = "websocket", address = "127.0.0.1:%d", transport = "gnet", protocol = "wsmux" }]`, tcp, ws))
	conf = setKey(t, conf, "plugin", "enable", "false")
	if beforeSendURL != "" {
		conf = setKey(t, conf, "webhook.before_send", "enabled", "true")
		conf = setKey(t, conf, "webhook.before_send", "http_addr", fmt.Sprintf("%q", beforeSendURL))
	}
	im := &IM{Ports: map[string]int{"api": api, "manager": mg, "cluster": cl, "tcp": tcp, "ws": ws}, APIURL: fmt.Sprintf("http://127.0.0.1:%d", api), t: t, bin: bin, cfg: filepath.Join(dir, "wukongim.toml"), logs: &lockedBuf{}}
	if err := os.WriteFile(im.cfg, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	im.start()
	t.Cleanup(func() {
		im.Stop()
		if t.Failed() {
			t.Logf("WuKongIM log tail:\n%s", tail(im.logs.String(), 3000))
		}
	})
	return im
}

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

func (im *IM) start() {
	im.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, im.bin, "-config", im.cfg)
	cmd.Stdout, cmd.Stderr = im.logs, im.logs
	cmd.Dir = filepath.Dir(im.cfg)
	if err := cmd.Start(); err != nil {
		cancel()
		im.t.Fatal(err)
	}
	im.cmd, im.cancel, im.done = cmd, cancel, make(chan struct{})
	go func() { _ = cmd.Wait(); close(im.done) }()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-im.done:
			im.t.Fatalf("WuKongIM exited during startup:\n%s", tail(im.logs.String(), 3000))
		default:
		}
		if resp, err := http.Get(im.APIURL + "/healthz"); err == nil {
			ok := resp.StatusCode == 200
			resp.Body.Close()
			if ok {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	im.t.Fatalf("WuKongIM not healthy within 60s:\n%s", tail(im.logs.String(), 3000))
}

// Stop terminates the process (SIGKILL via context) and waits for it.
func (im *IM) Stop() {
	if im.cancel == nil {
		return
	}
	im.cancel()
	<-im.done
	im.cancel = nil
}

// Restart stops and starts the IM again with the same config and data directory (same API address).
func (im *IM) Restart() {
	im.t.Helper()
	im.Stop()
	im.start()
}

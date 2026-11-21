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
	data   string
}

// DataDir is the directory the IM stores its state in.
func (im *IM) DataDir() string { return im.data }

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

// addKey inserts `key = value` directly under [section] for keys that exist only in patched builds (see patches/wukongim).
func addKey(t testing.TB, conf, section, key, value string) string {
	t.Helper()
	hdr := "[" + section + "]"
	i := strings.Index(conf, "\n"+hdr+"\n")
	if i < 0 {
		t.Fatalf("testim: section %s not found", hdr)
	}
	i += len(hdr) + 2
	return conf[:i] + key + " = " + value + "\n" + conf[i:]
}

// Start launches WuKongIM with before_send disabled (pass beforeSendURL to enable the synchronous webhook).
func Start(t testing.TB, beforeSendURL string) *IM {
	t.Helper()
	return StartSigned(t, beforeSendURL, "")
}

// StartSigned is Start plus webhook.signing_secret, which exists only in the VeyroGram-patched build (tools/build_wukongim.sh).
func StartSigned(t testing.TB, beforeSendURL, signingSecret string) *IM {
	t.Helper()
	return StartOpts(t, Options{BeforeSendURL: beforeSendURL, SigningSecret: signingSecret})
}

// Options configures a test IM. NotifyURL, when set, receives the asynchronous webhook (msg.notify only).
type Options struct {
	BeforeSendURL string
	SigningSecret string
	NotifyURL     string
	DataDir       string // use this existing data directory instead of a fresh one (restore rehearsals)
}

// StartOpts is the general launcher.
func StartOpts(t testing.TB, o Options) *IM {
	t.Helper()
	beforeSendURL, signingSecret := o.BeforeSendURL, o.SigningSecret
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
	dataPath := filepath.Join(dir, "data")
	if o.DataDir != "" {
		dataPath = o.DataDir
	}
	conf := strings.ReplaceAll(string(raw), "./data/wukongim-single-node-data", dataPath)
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
	if o.NotifyURL != "" {
		conf = setKey(t, conf, "webhook", "http_addr", fmt.Sprintf("%q", o.NotifyURL))
		conf = setKey(t, conf, "webhook", "focus_events", `["msg.notify"]`)
		conf = setKey(t, conf, "webhook", "msg_notify_batch_max_wait", `"50ms"`)
	}
	if signingSecret != "" {
		conf = addKey(t, conf, "webhook", "signing_secret", fmt.Sprintf("%q", signingSecret))
	}
	if beforeSendURL != "" {
		conf = setKey(t, conf, "webhook.before_send", "enabled", "true")
		conf = setKey(t, conf, "webhook.before_send", "http_addr", fmt.Sprintf("%q", beforeSendURL))
	}
	im := &IM{Ports: map[string]int{"api": api, "manager": mg, "cluster": cl, "tcp": tcp, "ws": ws}, APIURL: fmt.Sprintf("http://127.0.0.1:%d", api), t: t, bin: bin, cfg: filepath.Join(dir, "wukongim.toml"), logs: &lockedBuf{}, data: dataPath}
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
	if err := im.startErr(); err != nil {
		im.t.Fatal(err)
	}
}

// startErr launches the process and waits until it answers /healthz. It does not touch the testing.T, so a cluster can start its nodes concurrently
// (a three-node cluster only becomes healthy once a quorum is up).
func (im *IM) startErr() error {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, im.bin, "-config", im.cfg)
	cmd.Stdout, cmd.Stderr = im.logs, im.logs
	cmd.Dir = filepath.Dir(im.cfg)
	if err := cmd.Start(); err != nil {
		cancel()
		return err
	}
	im.cmd, im.cancel, im.done = cmd, cancel, make(chan struct{})
	go func() { _ = cmd.Wait(); close(im.done) }()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-im.done:
			return fmt.Errorf("WuKongIM exited during startup:\n%s", tail(im.logs.String(), 3000))
		default:
		}
		if resp, err := http.Get(im.APIURL + "/healthz"); err == nil {
			ok := resp.StatusCode == 200
			resp.Body.Close()
			if ok {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("WuKongIM not healthy within 90s:\n%s", tail(im.logs.String(), 3000))
}

// StartCluster launches an n-node WuKongIM cluster on loopback (replica count n, all nodes in one test process tree) with the same webhook options as
// StartOpts. Node i serves its own API/WS ports; the returned slice is ordered by node id (1..n).
func StartCluster(t testing.TB, n int, o Options) []*IM {
	t.Helper()
	bin := os.Getenv("VG_TEST_WUKONGIM_BIN")
	if bin == "" {
		t.Skip("VG_TEST_WUKONGIM_BIN not set: skipping WuKongIM cluster test (build it with tools/build_wukongim.sh)")
	}
	ex, err := findExample()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(ex)
	cl := make([]int, n)
	var nodes []string
	for i := range cl {
		cl[i] = freePort(t)
		nodes = append(nodes, fmt.Sprintf("{ id = %d, addr = \"127.0.0.1:%d\" }", i+1, cl[i]))
	}
	var ims []*IM
	for i := 0; i < n; i++ {
		dir := t.TempDir()
		conf := string(raw)
		conf = setKey(t, conf, "node", "id", fmt.Sprint(i+1))
		conf = setKey(t, conf, "node", "data_dir", fmt.Sprintf("%q", filepath.Join(dir, "data")))
		conf = strings.ReplaceAll(conf, "127.0.0.1:7001", fmt.Sprintf("127.0.0.1:%d", cl[i]))
		conf = setKey(t, conf, "cluster", "id", `"wukongim-test-cluster"`)
		conf = setKey(t, conf, "cluster", "nodes", "["+strings.Join(nodes, ", ")+"]")
		conf = setKey(t, conf, "cluster", "slot_replica_n", fmt.Sprint(n))
		api, mg, tcp, ws := freePort(t), freePort(t), freePort(t), freePort(t)
		conf = setKey(t, conf, "api", "listen_addr", fmt.Sprintf(`"127.0.0.1:%d"`, api))
		conf = setKey(t, conf, "api", "external_tcp_addr", fmt.Sprintf(`"127.0.0.1:%d"`, tcp))
		conf = setKey(t, conf, "api", "external_ws_addr", fmt.Sprintf(`"ws://127.0.0.1:%d"`, ws))
		conf = setKey(t, conf, "manager", "listen_addr", fmt.Sprintf(`"127.0.0.1:%d"`, mg))
		conf = setKey(t, conf, "manager", "jwt_secret", `"test-only-manager-secret-0123456789abcdef"`)
		conf = setKey(t, conf, "cluster", "join_token", `"test-only-join-token-0123456789abcdef"`)
		conf = setKey(t, conf, "gateway", "listeners", fmt.Sprintf(`[{ name = "tcp-wkproto", network = "tcp", address = "127.0.0.1:%d", transport = "gnet", protocol = "wkproto" }, { name = "ws-gateway", network = "websocket", address = "127.0.0.1:%d", transport = "gnet", protocol = "wsmux" }]`, tcp, ws))
		conf = setKey(t, conf, "plugin", "enable", "false")
		if o.NotifyURL != "" {
			conf = setKey(t, conf, "webhook", "http_addr", fmt.Sprintf("%q", o.NotifyURL))
			conf = setKey(t, conf, "webhook", "focus_events", `["msg.notify"]`)
		}
		if o.BeforeSendURL != "" {
			conf = setKey(t, conf, "webhook.before_send", "enabled", "true")
			conf = setKey(t, conf, "webhook.before_send", "http_addr", fmt.Sprintf("%q", o.BeforeSendURL))
		}
		im := &IM{Ports: map[string]int{"api": api, "manager": mg, "cluster": cl[i], "tcp": tcp, "ws": ws}, APIURL: fmt.Sprintf("http://127.0.0.1:%d", api), t: t, bin: bin, cfg: filepath.Join(dir, "wukongim.toml"), logs: &lockedBuf{}}
		if err := os.WriteFile(im.cfg, []byte(conf), 0o600); err != nil {
			t.Fatal(err)
		}
		ims = append(ims, im)
	}
	t.Cleanup(func() {
		for _, im := range ims {
			im.Stop()
		}
		if t.Failed() {
			for i, im := range ims {
				t.Logf("WuKongIM node %d log tail:\n%s", i+1, tail(im.logs.String(), 2000))
			}
		}
	})
	errs := make(chan error, n)
	for _, im := range ims {
		go func() { errs <- im.startErr() }()
	}
	for range ims {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	return ims
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

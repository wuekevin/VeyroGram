package testim_test

// Interop contract test (Story 1.11 AC2): the EXACT IM SDK the Web client ships (wukongimjssdk, version pinned in tests/imsdk/web/package.json)
// against the REAL WuKongIM built from the pinned SHA plus our patch queue. It proves the SDK and the server still speak the same protocol:
// handshake with a token we issued, a bad token is refused, a message sent by one SDK client reaches the other SDK client.
// Needs: VG_TEST_WUKONGIM_BIN (tools/build_wukongim.sh), node >= 22 on PATH, and `npm ci` done in tests/imsdk/web. Skipped, never faked, otherwise.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"veyrogram/api/internal/platform/imgw"
	"veyrogram/api/internal/platform/testim"
)

func sdkDir(t *testing.T) string {
	t.Helper()
	if os.Getenv("VG_TEST_WUKONGIM_BIN") == "" {
		t.Skip("VG_TEST_WUKONGIM_BIN unset")
	}
	skip := t.Skip
	if os.Getenv("VG_REQUIRE_IMSDK") == "1" { // CI: a missing prerequisite is a failure, not a silent skip
		skip = t.Fatal
	}
	if _, err := exec.LookPath("node"); err != nil {
		skip("node not installed")
	}
	dir, _ := filepath.Abs("../../../../../tests/imsdk/web")
	if _, err := os.Stat(filepath.Join(dir, "node_modules", "wukongimjssdk")); err != nil {
		skip("run `npm ci --ignore-scripts` in tests/imsdk/web first")
	}
	return dir
}

type event map[string]any

// node starts one SDK client process and streams its JSON lines.
func node(t *testing.T, dir string, args ...string) (*exec.Cmd, <-chan event) {
	t.Helper()
	cmd := exec.Command("node", append([]string{"client.mjs"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	ch := make(chan event, 32)
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			var e event
			if json.Unmarshal(sc.Bytes(), &e) == nil {
				ch <- e
			}
		}
	}()
	return cmd, ch
}

func waitFor(t *testing.T, ch <-chan event, name string) event {
	t.Helper()
	deadline := time.After(25 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				t.Fatalf("client exited before event %q", name)
			}
			if e["event"] == name {
				return e
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %q", name)
		}
	}
}

func setup(t *testing.T) (dir, ws string) {
	dir = sdkDir(t)
	im := testim.Start(t, "")
	c, err := imgw.New(im.APIURL)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, uid := range []string{"u1", "u2"} {
		// The SDK's own device type is not assumed: issue the token for every device flag (0 app, 1 web, 2 desktop).
		for flag := uint8(0); flag <= 2; flag++ {
			if err := c.UpdateToken(ctx, uid, "tok-"+uid, flag, 0); err != nil {
				t.Fatalf("update token: %v", err)
			}
		}
	}
	return dir, fmt.Sprintf("ws://127.0.0.1:%d", im.Ports["ws"])
}

func TestWebSDKConnectsWithIssuedTokenAndRefusesABadOne(t *testing.T) {
	dir, ws := setup(t)
	cmd, ch := node(t, dir, ws, "u1", "tok-u1", "connect")
	waitFor(t, ch, "connected")
	if err := cmd.Wait(); err != nil {
		t.Fatalf("a valid token must connect cleanly: %v", err)
	}
	bad, bch := node(t, dir, ws, "u1", "wrong-token", "connect")
	rej := waitFor(t, bch, "rejected")
	if rej["reasonCode"] == nil {
		t.Fatalf("a wrong token must be refused with a reason code, got %v", rej)
	}
	if err := bad.Wait(); err == nil {
		t.Fatal("the client with a wrong token must not report success")
	}
}

func TestWebSDKMessageReachesAnotherSDKClient(t *testing.T) {
	dir, ws := setup(t)
	_, recv := node(t, dir, ws, "u2", "tok-u2", "recv")
	waitFor(t, recv, "connected")
	snd, send := node(t, dir, ws, "u1", "tok-u1", "send", "u2", "hello from the web sdk")
	ack := waitFor(t, send, "ack")
	if ack["reasonCode"] != float64(1) {
		t.Fatalf("send must be acknowledged with reason code 1 (success), got %v", ack)
	}
	if err := snd.Wait(); err != nil {
		t.Fatalf("sender: %v", err)
	}
	m := waitFor(t, recv, "message")
	if m["from"] != "u1" || m["text"] != "hello from the web sdk" {
		t.Fatalf("receiver got the wrong message: %v", m)
	}
}

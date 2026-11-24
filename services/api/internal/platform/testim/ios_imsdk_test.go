package testim_test

// Interop contract test (Story 1.11 AC2, iOS): the EXACT IM SDK the iOS client ships (WuKongIMSDK, the source pinned by apps/ios/PIN) running in
// the iOS simulator against the REAL WuKongIM built from the pinned SHA plus our patch queue. The simulator shares the host network, so the probe
// app connects to 127.0.0.1. Crosses SDKs like the Android test: Web -> iOS and iOS -> Web.
// Needs: VG_TEST_WUKONGIM_BIN, VG_IOS_SIM (simulator UDID) with the probe installed (tests/imsdk/ios/run.sh does all of it), node + tests/imsdk/web deps.
// Skipped otherwise; VG_REQUIRE_IMSDK_IOS=1 turns a missing prerequisite into a failure.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"veyrogram/api/internal/platform/testim"
)

const iosProbeBundle = "app.veyrogram.improbe"

func iosSetup(t *testing.T) (sim, nodeDir string, im *testim.IM) {
	t.Helper()
	nodeDir = sdkDir(t)
	skip := t.Skip
	if os.Getenv("VG_REQUIRE_IMSDK_IOS") == "1" {
		skip = t.Fatal
	}
	sim = os.Getenv("VG_IOS_SIM")
	if sim == "" {
		skip("VG_IOS_SIM unset: run tests/imsdk/ios/run.sh")
	}
	if _, err := exec.LookPath("xcrun"); err != nil {
		skip("xcrun not found")
	}
	if out, err := exec.Command("xcrun", "simctl", "get_app_container", sim, iosProbeBundle).CombinedOutput(); err != nil {
		skip("probe app not installed in the simulator: " + strings.TrimSpace(string(out)))
	}
	return sim, nodeDir, testim.Start(t, "")
}

// iosProbe launches the probe app and streams its JSON event lines (the app exits itself when done).
func iosProbe(t *testing.T, sim string, args ...string) <-chan event {
	t.Helper()
	a := append([]string{"simctl", "launch", "--console", "--terminate-running-process", sim, iosProbeBundle}, args...)
	cmd := exec.Command("xcrun", a...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = exec.Command("xcrun", "simctl", "terminate", sim, iosProbeBundle).Run()
	})
	ch := make(chan event, 64)
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(out)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var e event
			if l := sc.Text(); strings.HasPrefix(l, "{") && json.Unmarshal([]byte(l), &e) == nil {
				ch <- e
			}
		}
	}()
	return ch
}

func iosIssue(t *testing.T, im *testim.IM, uids ...string) { t.Helper(); issue(t, im, uids...) }

func iosWaitFor(t *testing.T, ch <-chan event, name string, d time.Duration) event {
	t.Helper()
	deadline := time.After(d)
	var seen []event
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				t.Fatalf("the iOS probe exited before event %q; saw %v", name, seen)
			}
			seen = append(seen, e)
			if e["event"] == name {
				return e
			}
			if e["event"] == "error" {
				t.Fatalf("the iOS probe reported an error: %v", e)
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %q; saw %v", name, seen)
		}
	}
}

func TestIOSSDKConnectsWithIssuedTokenAndRefusesABadOne(t *testing.T) {
	sim, _, im := iosSetup(t)
	iosIssue(t, im, "i1")
	addr := fmt.Sprintf("127.0.0.1:%d", im.Ports["tcp"])
	iosWaitFor(t, iosProbe(t, sim, addr, "i1", "tok-i1", "connect"), "connected", 40*time.Second)
	bad := iosProbe(t, sim, addr, "i1", "wrong-token", "bad")
	var seen []event
	for {
		select {
		case e, ok := <-bad:
			if !ok {
				t.Fatal("the iOS probe exited without reporting")
			}
			seen = append(seen, e)
			if e["event"] == "connected" {
				t.Fatalf("a wrong token must never connect: %v", e)
			}
			if e["event"] == "not_connected" {
				// Unlike the Android SDK, the iOS SDK reports the refusal explicitly: status Disconnected (4) with reason WK_REASON_AUTHFAIL (2).
				explicit := false
				for _, s := range seen {
					explicit = explicit || (s["status"] == float64(4) && s["reasonCode"] == float64(2))
				}
				if !explicit {
					t.Fatalf("the iOS SDK must report the refusal as Disconnected/AUTHFAIL, saw %v", seen)
				}
				if e["connected"] != false {
					t.Fatalf("a wrong token must never connect: %v", e)
				}
				return
			}
		case <-time.After(60 * time.Second):
			t.Fatal("timed out waiting for the verdict")
		}
	}
}

func TestWebSDKMessageReachesIOSSDK(t *testing.T) {
	sim, nodeDir, im := iosSetup(t)
	iosIssue(t, im, "i1", "u1")
	recv := iosProbe(t, sim, fmt.Sprintf("127.0.0.1:%d", im.Ports["tcp"]), "i1", "tok-i1", "recv")
	iosWaitFor(t, recv, "connected", 40*time.Second)
	time.Sleep(2 * time.Second)
	ws := fmt.Sprintf("ws://127.0.0.1:%d", im.Ports["ws"])
	snd, send := node(t, nodeDir, ws, "u1", "tok-u1", "send", "i1", "hello ios from the web sdk")
	if ack := waitFor(t, send, "ack"); ack["reasonCode"] != float64(1) {
		t.Fatalf("send must be acknowledged, got %v", ack)
	}
	_ = snd.Wait()
	m := iosWaitFor(t, recv, "message", 40*time.Second)
	if m["from"] != "u1" || m["text"] != "hello ios from the web sdk" {
		t.Fatalf("the iOS SDK got the wrong message: %v", m)
	}
}

func TestIOSSDKMessageReachesWebSDK(t *testing.T) {
	sim, nodeDir, im := iosSetup(t)
	iosIssue(t, im, "i1", "u2")
	ws := fmt.Sprintf("ws://127.0.0.1:%d", im.Ports["ws"])
	_, recv := node(t, nodeDir, ws, "u2", "tok-u2", "recv")
	waitFor(t, recv, "connected")
	ack := iosWaitFor(t, iosProbe(t, sim, fmt.Sprintf("127.0.0.1:%d", im.Ports["tcp"]), "i1", "tok-i1", "send", "u2", "hello web from the ios sdk"), "ack", 40*time.Second)
	if ack["reasonCode"] != float64(1) {
		t.Fatalf("the iOS SDK send must be acknowledged: %v", ack)
	}
	m := waitFor(t, recv, "message")
	if m["from"] != "i1" || m["text"] != "hello web from the ios sdk" {
		t.Fatalf("the Web SDK got the wrong message: %v", m)
	}
}

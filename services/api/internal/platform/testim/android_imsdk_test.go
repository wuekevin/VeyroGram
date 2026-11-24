package testim_test

// Interop contract test (Story 1.11 AC2, Android): the EXACT IM SDK the Android client ships (WuKongIMAndroidSDK, version pinned in
// tests/imsdk/android/app/build.gradle) running on an emulator/device against the REAL WuKongIM built from the pinned SHA plus our patch queue.
// The probe app reaches the host WuKongIM through `adb reverse`. Besides Android<->Android-free checks it crosses SDKs: a Web SDK client
// sends to the Android SDK client and the other way round.
// Needs: VG_TEST_WUKONGIM_BIN, `adb` with ONE device/emulator online, and the probe APKs built and installed (tests/imsdk/android/run.sh does all of it).
// Skipped otherwise; VG_REQUIRE_IMSDK_ANDROID=1 turns a missing prerequisite into a failure.

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"veyrogram/api/internal/platform/imgw"
	"veyrogram/api/internal/platform/testim"
)

const probeRunner = "app.veyrogram.improbe.test/androidx.test.runner.AndroidJUnitRunner"

func adbPath() string {
	if p := os.Getenv("VG_ADB"); p != "" {
		return p
	}
	if p, err := exec.LookPath("adb"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	for _, c := range []string{os.Getenv("ANDROID_HOME"), filepath.Join(home, "Library/Android/sdk")} {
		if c != "" {
			if _, err := os.Stat(filepath.Join(c, "platform-tools/adb")); err == nil {
				return filepath.Join(c, "platform-tools/adb")
			}
		}
	}
	return ""
}

func androidSetup(t *testing.T) (adb, nodeDir string, im *testim.IM) {
	t.Helper()
	nodeDir = sdkDir(t) // also checks VG_TEST_WUKONGIM_BIN and node
	skip := t.Skip
	if os.Getenv("VG_REQUIRE_IMSDK_ANDROID") == "1" {
		skip = t.Fatal
	}
	adb = adbPath()
	if adb == "" {
		skip("adb not found")
	}
	out, _ := exec.Command(adb, "devices").Output()
	n := 0
	for _, l := range strings.Split(string(out), "\n")[1:] {
		if strings.HasSuffix(strings.TrimSpace(l), "device") {
			n++
		}
	}
	if n != 1 {
		skip(fmt.Sprintf("need exactly one adb device online, found %d", n))
	}
	if o, _ := exec.Command(adb, "shell", "pm", "list", "instrumentation").Output(); !strings.Contains(string(o), probeRunner) {
		skip("probe app not installed: run tests/imsdk/android/run.sh")
	}
	im = testim.Start(t, "")
	return adb, nodeDir, im
}

type probeResult struct {
	events []map[string]string
	ok     bool
	raw    string
}

func (r probeResult) get(event, key string) string {
	for _, e := range r.events {
		if e["event"] == event {
			return e[key]
		}
	}
	return ""
}
func (r probeResult) has(event string) bool {
	for _, e := range r.events {
		if e["event"] == event {
			return true
		}
	}
	return false
}

// parseInstrument turns `am instrument -r` output into events and the final verdict.
func parseInstrument(out string) probeResult {
	r := probeResult{raw: out}
	cur := map[string]string{}
	flush := func() {
		if len(cur) > 0 {
			r.events = append(r.events, cur)
			cur = map[string]string{}
		}
	}
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		l := sc.Text()
		switch {
		case strings.HasPrefix(l, "INSTRUMENTATION_STATUS: "):
			kv := strings.SplitN(strings.TrimPrefix(l, "INSTRUMENTATION_STATUS: "), "=", 2)
			if len(kv) == 2 && kv[0] != "numtests" && kv[0] != "current" && kv[0] != "id" && kv[0] != "class" && kv[0] != "test" && kv[0] != "stack" {
				cur[kv[0]] = kv[1]
			}
		case strings.HasPrefix(l, "INSTRUMENTATION_STATUS_CODE:"):
			flush()
		}
	}
	r.ok = strings.Contains(out, "OK (1 test)")
	return r
}

func startProbe(t *testing.T, adb string, port int, args ...string) <-chan probeResult {
	t.Helper()
	_ = exec.Command(adb, "reverse", fmt.Sprintf("tcp:%d", port), fmt.Sprintf("tcp:%d", port)).Run()
	t.Cleanup(func() { _ = exec.Command(adb, "reverse", "--remove", fmt.Sprintf("tcp:%d", port)).Run() })
	// adb joins the arguments into one device-shell command line, so every value is single-quoted (message text contains spaces).
	a := []string{"shell", "am", "instrument", "--user", "0", "-w", "-r", "-e", "addr", fmt.Sprintf("127.0.0.1:%d", port)}
	for _, v := range args {
		a = append(a, "'"+strings.ReplaceAll(v, "'", `'\''`)+"'")
	}
	if os.Getenv("VG_ANDROID_PROBE_DEBUG") == "1" { // SDK debug log in logcat (tag WKLogger*)
		a = append(a, "-e", "debug", "1")
	}
	a = append(a, probeRunner)
	ch := make(chan probeResult, 1)
	go func() {
		out, _ := exec.Command(adb, a...).CombinedOutput()
		ch <- parseInstrument(string(out))
	}()
	return ch
}

func waitProbe(t *testing.T, ch <-chan probeResult) probeResult {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(90 * time.Second):
		t.Fatal("the Android probe did not finish in time")
		return probeResult{}
	}
}

func issue(t *testing.T, im *testim.IM, uids ...string) {
	t.Helper()
	c, err := imgw.New(im.APIURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, uid := range uids {
		for flag := uint8(0); flag <= 2; flag++ {
			if err := c.UpdateToken(context.Background(), uid, "tok-"+uid, flag, 0); err != nil {
				t.Fatalf("update token: %v", err)
			}
		}
	}
}

func TestAndroidSDKConnectsWithIssuedTokenAndRefusesABadOne(t *testing.T) {
	adb, _, im := androidSetup(t)
	issue(t, im, "a1")
	port := im.Ports["tcp"]
	good := waitProbe(t, startProbe(t, adb, port, "-e", "uid", "a1", "-e", "token", "tok-a1", "-e", "mode", "connect"))
	if !good.ok || !good.has("connected") {
		t.Fatalf("the Android SDK must connect with the issued token; got ok=%v events=%v\n%s", good.ok, good.events, good.raw)
	}
	bad := waitProbe(t, startProbe(t, adb, port, "-e", "uid", "a1", "-e", "token", "wrong-token", "-e", "mode", "bad"))
	if !bad.ok || !bad.has("not_connected") || bad.has("connected") {
		t.Fatalf("a wrong token must never connect; got ok=%v events=%v\n%s", bad.ok, bad.events, bad.raw)
	}
}

func TestWebSDKMessageReachesAndroidSDK(t *testing.T) {
	adb, nodeDir, im := androidSetup(t)
	issue(t, im, "a1", "u1")
	recv := startProbe(t, adb, im.Ports["tcp"], "-e", "uid", "a1", "-e", "token", "tok-a1", "-e", "mode", "recv")
	time.Sleep(6 * time.Second) // let the probe connect before the Web client sends (offline delivery is not what this test is about)
	ws := fmt.Sprintf("ws://127.0.0.1:%d", im.Ports["ws"])
	snd, send := node(t, nodeDir, ws, "u1", "tok-u1", "send", "a1", "hello android from the web sdk")
	ack := waitFor(t, send, "ack")
	if ack["reasonCode"] != float64(1) {
		t.Fatalf("send must be acknowledged, got %v", ack)
	}
	_ = snd.Wait()
	r := waitProbe(t, recv)
	if !r.ok || r.get("message", "from") != "u1" || r.get("message", "text") != "hello android from the web sdk" {
		t.Fatalf("the Android SDK got the wrong message: ok=%v events=%v\n%s", r.ok, r.events, r.raw)
	}
}

func TestAndroidSDKMessageReachesWebSDK(t *testing.T) {
	adb, nodeDir, im := androidSetup(t)
	issue(t, im, "a1", "u2")
	ws := fmt.Sprintf("ws://127.0.0.1:%d", im.Ports["ws"])
	_, recv := node(t, nodeDir, ws, "u2", "tok-u2", "recv")
	waitFor(t, recv, "connected")
	r := waitProbe(t, startProbe(t, adb, im.Ports["tcp"], "-e", "uid", "a1", "-e", "token", "tok-a1", "-e", "mode", "send", "-e", "peer", "u2", "-e", "text", "hello web from the android sdk"))
	if !r.ok || !r.has("ack") {
		t.Fatalf("the Android SDK send must be acknowledged: ok=%v events=%v\n%s", r.ok, r.events, r.raw)
	}
	m := waitFor(t, recv, "message")
	if m["from"] != "a1" || m["text"] != "hello web from the android sdk" {
		t.Fatalf("the Web SDK got the wrong message: %v", m)
	}
}

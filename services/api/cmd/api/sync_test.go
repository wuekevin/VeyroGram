package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	convdomain "veyrogram/api/internal/conversation/domain"
	identitydomain "veyrogram/api/internal/identity/domain"
	"veyrogram/api/internal/platform/denylist"
	"veyrogram/api/internal/platform/testim"
)

func TestSyncSystemUIDMatchesIdentity(t *testing.T) {
	if convdomain.SystemUID != identitydomain.SystemUID {
		t.Fatalf("conversation %q vs identity %q", convdomain.SystemUID, identitydomain.SystemUID)
	}
}

// jget calls a GET endpoint and decodes the JSON answer.
func jget(t *testing.T, url, access string) (int, map[string]any) {
	t.Helper()
	c, b := get(t, url, "Bearer "+access)
	var m map[string]any
	_ = json.Unmarshal([]byte(b), &m)
	return c, m
}

func jsend(t *testing.T, method, url, access, body string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var m map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&m)
	return resp.StatusCode, m
}

func seqsOf(m map[string]any, key string) []uint64 {
	var out []uint64
	for _, x := range m[key].([]any) {
		out = append(out, uint64(x.(map[string]any)["message_seq"].(float64)))
	}
	return out
}

// The 1.16 end-to-end proof: a brand-new device catches up from scratch, unread counts agree across a user's devices, a device that was offline gets
// exactly the missed messages in order, drafts follow the user across devices (with a push nudge), and nobody can read anybody else's state.
func TestSyncCatchUpUnreadDraftsAndIsolationOverRealIM(t *testing.T) {
	redisAddr := os.Getenv("VG_TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("VG_TEST_REDIS_ADDR not set")
	}
	hookAddr := freeAddr(t)
	im := testim.Start(t, "http://"+hookAddr+"/im/webhook/"+msgHookSecret)
	d, dsn, bin := migrated(t)
	extra := []string{"VG_REDIS_ADDR=" + redisAddr, "VG_IM_API_URL=" + im.APIURL, "VG_METRICS_TOKEN=metrics-token-0123456789abcdef",
		"VG_IM_WEBHOOK_SECRET=" + msgHookSecret, "VG_IM_WEBHOOK_ADDR=" + hookAddr, "VG_MESSAGE_RATE_LIMIT=100000"}
	if r, err := denylist.Open(context.Background(), redisAddr, ""); err == nil {
		r.C.Del(context.Background(), "vg:deny:dev:1:1", "vg:deny:dev:1:2", "vg:deny:dev:1:3", "vg:deny:dev:1:4")
		r.Close()
	}
	p := start(t, authEnv(dsn, extra...)...)
	startWorker(t, bin, dsn, extra)
	base := "http://" + p.pub
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d", im.Ports["ws"])

	login := func(phone, device string) (access, uid string) {
		t.Helper()
		_, _ = d.Exec("UPDATE login_event SET created_at = created_at - INTERVAL 5 MINUTE")
		before := len(regexp.MustCompile(`验证码 (\d{6})`).FindAllStringSubmatch(p.stderr.String(), -1))
		jpost(t, base+"/v1/auth/sms/code", `{"phone":"`+phone+`","device_id":"`+device+`"}`)
		var code string
		eventually(t, "sms code", 3*time.Second, func() bool {
			all := regexp.MustCompile(`验证码 (\d{6})`).FindAllStringSubmatch(p.stderr.String(), -1)
			if len(all) > before {
				code = all[len(all)-1][1]
				return true
			}
			return false
		})
		c, body := jpost(t, base+"/v1/auth/login", `{"phone":"`+phone+`","code":"`+code+`","device":{"id":"`+device+`","class":"app"}}`)
		if c != 200 {
			t.Fatalf("login: %d %v", c, body)
		}
		access, _ = body["access_token"].(string)
		return access, ""
	}
	creds := func(access string) (uid, tok string) {
		c, m := jget(t, base+"/v1/im/token", access)
		if c != 200 {
			t.Fatalf("im token: %d", c)
		}
		return m["uid"].(string), m["token"].(string)
	}
	accA, _ := login("+8613800138201", "device-aaaaaaaaaaaaaaaa")
	accB1, _ := login("+8613800138202", "device-bbbbbbbbbbbbbbbb")
	accC, _ := login("+8613800138203", "device-cccccccccccccccc")
	uidA, _ := creds(accA)
	uidB, tokB := creds(accB1)
	uidC, _ := creds(accC)
	send := func(acc, to, text string) uint64 {
		t.Helper()
		b := fmt.Sprintf(`{"to_uid":%q,"client_msg_no":%q,"payload":{"type":1,"content":%q}}`, to, ulid(), text)
		c, r := jpost(t, base+"/v1/messages", b, "Authorization", "Bearer "+acc)
		if c != 200 {
			t.Fatalf("send: %d %v", c, r)
		}
		return uint64(r["message_seq"].(float64))
	}

	// 1. A sends 7 messages while B has no device online and has never synced.
	for i := 1; i <= 7; i++ {
		send(accA, uidB, fmt.Sprintf("m%d", i))
	}

	// 2. B signs in on a brand-new device: the conversation list shows the conversation with the right unread count and the newest message(s).
	accB2, _ := login("+8613800138202", "device-dddddddddddddddd")
	var convLat, msgLat []time.Duration
	t0 := time.Now()
	c, conv := jget(t, base+"/v1/sync/conversations?msg_count=2", accB2)
	convLat = append(convLat, time.Since(t0))
	items, _ := conv["items"].([]any)
	if c != 200 || len(items) != 1 {
		t.Fatalf("new device must see exactly the one conversation: %d %v", c, conv)
	}
	it := items[0].(map[string]any)
	if it["peer"] != uidA || it["unread"] != float64(7) || it["last_msg_seq"] != float64(7) {
		t.Fatalf("conversation row: %v", it)
	}
	if r := seqsOf(it, "recents"); len(r) != 2 || r[0] != 6 || r[1] != 7 {
		t.Fatalf("two newest messages, oldest first: %v", r)
	}
	// unread parity: the user's other device (B1) sees the same numbers
	_, conv1 := jget(t, base+"/v1/sync/conversations", accB1)
	if u := conv1["items"].([]any)[0].(map[string]any)["unread"]; u != float64(7) {
		t.Fatalf("unread must agree across devices: %v", u)
	}

	// 3. Catch up on the whole history in pages of 3: every message exactly once, in order, with a correct has_more.
	var got []uint64
	after, pages := uint64(0), 0
	for {
		t0 = time.Now()
		c, m := jget(t, fmt.Sprintf("%s/v1/sync/messages?peer=%s&after_seq=%d&limit=3", base, uidA, after), accB2)
		msgLat = append(msgLat, time.Since(t0))
		if c != 200 {
			t.Fatalf("messages: %d %v", c, m)
		}
		got = append(got, seqsOf(m, "messages")...)
		after = uint64(m["next_seq"].(float64))
		pages++
		if m["has_more"] != true {
			break
		}
		if pages > 10 {
			t.Fatal("paging does not terminate")
		}
	}
	if fmt.Sprint(got) != "[1 2 3 4 5 6 7]" || pages != 3 {
		t.Fatalf("history must arrive complete, ordered, without repeats: %v in %d pages", got, pages)
	}
	// the payload comes back as the JSON object that was sent
	_, one := jget(t, fmt.Sprintf("%s/v1/sync/messages?peer=%s&after_seq=6&limit=5", base, uidA), accB2)
	pl := one["messages"].([]any)[0].(map[string]any)["payload"].(map[string]any)
	if pl["type"] != float64(1) || pl["content"] != "m7" {
		t.Fatalf("payload: %v", pl)
	}

	// 4. B reads the conversation on one device; every device sees unread 0.
	if c, _ := jsend(t, "POST", base+"/v1/sync/read", accB2, fmt.Sprintf(`{"peer":%q}`, uidA)); c != 204 {
		t.Fatalf("read: %d", c)
	}
	for name, acc := range map[string]string{"B1": accB1, "B2": accB2} {
		_, m := jget(t, base+"/v1/sync/conversations", acc)
		if u := m["items"].([]any)[0].(map[string]any)["unread"]; u != float64(0) {
			t.Fatalf("%s must see unread 0 after the read mark: %v", name, u)
		}
	}

	// 5. Offline catch-up: B's device connects, goes away, A keeps writing, B comes back and syncs from the last seq it holds.
	wsB := dialWS(t, wsURL)
	if !wsB.connect(uidB, tokB, "im-b", 0) {
		t.Fatal("B must connect")
	}
	_ = wsB.conn.Close()                                 // network drop
	if w := os.Getenv("VG_TEST_OFFLINE_WAIT"); w != "" { // set to 10m to run the story's real "offline for 10 minutes" scenario (kept out of CI for its duration)
		d, err := time.ParseDuration(w)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("device B stays offline for %v", d)
		time.Sleep(d)
	}
	for i := 8; i <= 12; i++ {
		send(accA, uidB, fmt.Sprintf("m%d", i))
	}
	wsB2 := dialWS(t, wsURL)
	if !wsB2.connect(uidB, tokB, "im-b", 0) {
		t.Fatal("B must reconnect")
	}
	_, m := jget(t, fmt.Sprintf("%s/v1/sync/messages?peer=%s&after_seq=7&limit=50", base, uidA), accB2)
	if fmt.Sprint(seqsOf(m, "messages")) != "[8 9 10 11 12]" || m["has_more"] != false {
		t.Fatalf("exactly the missed messages, once, in order: %v", m)
	}
	_, conv = jget(t, base+"/v1/sync/conversations", accB2)
	if u := conv["items"].([]any)[0].(map[string]any)["unread"]; u != float64(5) {
		t.Fatalf("unread counts only what arrived after the read mark: %v", u)
	}

	// 6. Drafts follow the user: B1 types, B2 (connected) is nudged and fetches the same text; clearing it propagates as a tombstone.
	c, dv := jsend(t, "PUT", base+"/v1/sync/drafts", accB1, fmt.Sprintf(`{"peer":%q,"content":"half a thought"}`, uidA))
	if c != 200 || dv["version"] != float64(1) {
		t.Fatalf("put draft: %d %v", c, dv)
	}
	notified := false
	deadline := time.Now().Add(5 * time.Second) // the story's 5-second target, on a quiet machine
	for time.Now().Before(deadline) && !notified {
		if _, n, ok := wsB2.recvNotice(time.Until(deadline)); ok && n["type"] == "sync_notice" && n["kind"] == "draft" {
			notified = true
			if strings.Contains(fmt.Sprint(n), "half a thought") {
				t.Fatal("the push notice must carry pointers only, never draft text")
			}
		}
	}
	if !notified {
		t.Fatal("B's other device must be nudged within 5 s")
	}
	_, st := jget(t, base+"/v1/sync/state?since=0", accB2)
	si := st["items"].([]any)
	if len(si) != 1 || si[0].(map[string]any)["content"] != "half a thought" || si[0].(map[string]any)["peer"] != uidA {
		t.Fatalf("B2 must see the same draft: %v", st)
	}
	jsend(t, "PUT", base+"/v1/sync/conversation-settings", accB1, fmt.Sprintf(`{"peer":%q,"pinned":true,"muted":false}`, uidA))
	jsend(t, "PUT", base+"/v1/sync/drafts", accB1, fmt.Sprintf(`{"peer":%q,"content":""}`, uidA))
	_, st = jget(t, base+"/v1/sync/state?since=1", accB2) // only what changed after version 1, in version order
	si = st["items"].([]any)
	if len(si) != 2 || si[0].(map[string]any)["kind"] != "setting" || si[0].(map[string]any)["pinned"] != true ||
		si[1].(map[string]any)["kind"] != "draft" || si[1].(map[string]any)["deleted"] != true || st["next_since"] != float64(3) {
		t.Fatalf("incremental state: %v", st)
	}
	// concurrent writers of one user never get the same version
	vers := make(chan float64, 20)
	for i := 0; i < 20; i++ {
		go func(i int) {
			_, r := jsend(t, "PUT", base+"/v1/sync/drafts", accB1, fmt.Sprintf(`{"peer":%q,"content":"d%d"}`, uidA, i))
			vers <- r["version"].(float64)
		}(i)
	}
	seen := map[float64]bool{}
	for i := 0; i < 20; i++ {
		seen[<-vers] = true
	}
	if len(seen) != 20 {
		t.Fatalf("versions must be unique under concurrency: %d distinct", len(seen))
	}

	// 6b. Conflict detection: a device that edits from a stale version does not clobber the newer draft; it gets the current one back to merge.
	_, cur := jsend(t, "PUT", base+"/v1/sync/drafts", accB1, fmt.Sprintf(`{"peer":%q,"content":"newest from B1"}`, uidA))
	curV := cur["version"].(float64)
	c, cf := jsend(t, "PUT", base+"/v1/sync/drafts", accB2, fmt.Sprintf(`{"peer":%q,"content":"stale edit","base_version":%d}`, uidA, uint64(curV)-1))
	if c != 409 || cf["code"] != "draft_conflict" || cf["current"].(map[string]any)["content"] != "newest from B1" || cf["current"].(map[string]any)["version"] != curV {
		t.Fatalf("a stale edit must be refused with the current draft: %d %v", c, cf)
	}
	if c, ok := jsend(t, "PUT", base+"/v1/sync/drafts", accB2, fmt.Sprintf(`{"peer":%q,"content":"merged","base_version":%d}`, uidA, uint64(curV))); c != 200 || ok["version"].(float64) <= curV {
		t.Fatalf("an edit based on the current version is accepted: %d %v", c, ok)
	}
	// tombstone retention: a cleared draft older than the window is purged by the CLI; a fresh one and live drafts stay
	jsend(t, "PUT", base+"/v1/sync/drafts", accB1, fmt.Sprintf(`{"peer":%q,"content":""}`, uidA))
	if _, err := d.Exec("UPDATE draft SET updated_at = updated_at - INTERVAL 40 DAY WHERE deleted=1"); err != nil {
		t.Fatal(err)
	}
	jsend(t, "PUT", base+"/v1/sync/drafts", accB1, fmt.Sprintf(`{"peer":%q,"content":"live again"}`, uidC))
	jsend(t, "PUT", base+"/v1/sync/drafts", accB1, fmt.Sprintf(`{"peer":%q,"content":""}`, uidC)) // fresh tombstone
	out, serr, code := runBin(t, bin, baseEnv(append(authEnv(dsn, extra...), "VG_ENV=development")...), "sync", "purge-tombstones", "-days", "30")
	var left int
	_ = d.QueryRow("SELECT COUNT(*) FROM draft WHERE deleted=1").Scan(&left)
	if code != 0 || !strings.Contains(out, "purged 1 draft tombstones") || left != 1 {
		t.Fatalf("purge: code=%d out=%q err=%q tombstones left=%d (want the fresh one only)", code, out, serr, left)
	}

	// 7. Isolation: C sees nothing of A<->B, cannot name a foreign conversation, and has no state of B's.
	_, cc := jget(t, base+"/v1/sync/conversations", accC)
	if len(cc["items"].([]any)) != 0 {
		t.Fatalf("an uninvolved user must see no conversations: %v", cc)
	}
	for _, peer := range []string{uidA, uidB} {
		_, cm := jget(t, fmt.Sprintf("%s/v1/sync/messages?peer=%s&after_seq=0&limit=50", base, peer), accC)
		if len(cm["messages"].([]any)) != 0 {
			t.Fatalf("C must not read anything through peer=%s: %v", peer, cm)
		}
	}
	if _, cs := jget(t, base+"/v1/sync/state?since=0", accC); len(cs["items"].([]any)) != 0 {
		t.Fatalf("C has no state of B's: %v", cs)
	}
	for name, path := range map[string]string{
		"composite channel id": "/v1/sync/messages?peer=" + uidA + "@" + uidB,
		"system conversation":  "/v1/sync/messages?peer=vg_system",
		"bad cursor":           "/v1/sync/conversations?cursor=nope",
		"negative seq":         "/v1/sync/messages?peer=" + uidA + "&after_seq=-1",
	} {
		if c, _ := jget(t, base+path, accC); c != 400 {
			t.Errorf("%s: %d, want 400", name, c)
		}
	}
	if c, _ := get(t, base+"/v1/sync/conversations", ""); c != 401 {
		t.Errorf("unauthenticated: %d", c)
	}
	_ = uidC
	// the account-notice conversation (vg_system) never shows up as a conversation
	_, ba := jget(t, base+"/v1/sync/conversations?limit=100", accB1)
	for _, x := range ba["items"].([]any) {
		if x.(map[string]any)["peer"] == "vg_system" {
			t.Fatal("service notices are not conversations")
		}
	}

	// 8. latency evidence for the page reads (recorded, not a pass/fail gate: [ASSUMPTION] targets need real hardware under load)
	for i := 0; i < 30; i++ {
		t0 = time.Now()
		jget(t, base+"/v1/sync/conversations?msg_count=2", accB2)
		convLat = append(convLat, time.Since(t0))
		t0 = time.Now()
		jget(t, fmt.Sprintf("%s/v1/sync/messages?peer=%s&after_seq=0&limit=10", base, uidA), accB2)
		msgLat = append(msgLat, time.Since(t0))
	}
	pct := func(l []time.Duration) string {
		sort.Slice(l, func(i, j int) bool { return l[i] < l[j] })
		return fmt.Sprintf("n=%d p50=%v p95=%v max=%v", len(l), l[len(l)/2], l[len(l)*95/100], l[len(l)-1])
	}
	t.Logf("sync latency (12 messages, one machine): conversations %s | messages %s", pct(convLat), pct(msgLat))
}

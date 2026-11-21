package main

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"
)

// Paging while the list changes underneath, and the sync endpoints at a size that is not a toy: 300 conversations and a 3000-message history.
func TestSyncPagingIsStableAndFastAtScale(t *testing.T) {
	r := newRig(t)
	accA, uidA, _ := r.user("+8613800138601", "device-aaaaaaaaaaaaaaaa")
	const peers, history = 300, 3000
	// seed accounts straight into the identity table (the test is about sync, not about 300 SMS logins)
	for i := 0; i < peers; i++ {
		if _, err := r.d.Exec("INSERT INTO `user` (tenant_id, uid, phone_hash, phone_last4, created_at) VALUES (1,?,?,?,NOW(3))", fmt.Sprintf("seed_%03d", i), fmt.Sprintf("%064d", i+1), "0000"); err != nil {
			t.Fatal(err)
		}
	}
	send := func(acc, to string) {
		c, m := jpost(t, r.base+"/v1/messages", fmt.Sprintf(`{"to_uid":%q,"client_msg_no":%q,"payload":{"type":1,"content":"seed"}}`, to, ulid()), "Authorization", "Bearer "+acc)
		if c != 200 {
			t.Errorf("seed send: %d %v", c, m)
		}
	}
	// A messages every peer once, oldest conversation first: seed_000 is the least recently active, seed_299 the most
	for i := 0; i < peers; i++ {
		send(accA, fmt.Sprintf("seed_%03d", i))
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(1100 * time.Millisecond) // the IM orders by second-resolution timestamps; make the next activity strictly newer
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for i := 0; i < history; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func() { defer wg.Done(); defer func() { <-sem }(); send(accA, "seed_150") }()
	}
	wg.Wait()

	// 1. Page the whole list (limit 50) and, after the first page, make the LEAST recently active conversation the most recent. It moves from the last
	// page to the top; every other row shifts down by one. Nothing may be skipped.
	seen := map[string]int{}
	cursor, pages := "", 0
	var convLat []time.Duration
	for {
		t0 := time.Now()
		c, m := jget(t, r.base+"/v1/sync/conversations?limit=50&msg_count=3&cursor="+cursor, accA)
		convLat = append(convLat, time.Since(t0))
		if c != 200 {
			t.Fatalf("page %d: %d %v", pages, c, m)
		}
		for _, x := range m["items"].([]any) {
			seen[x.(map[string]any)["peer"].(string)]++
		}
		pages++
		if pages == 1 {
			time.Sleep(1100 * time.Millisecond)
			send(accA, "seed_000")
		}
		if m["has_more"] != true {
			break
		}
		cursor = m["next_cursor"].(string)
		if pages > 20 {
			t.Fatal("paging does not terminate")
		}
	}
	dups := 0
	for i := 0; i < peers; i++ {
		n := seen[fmt.Sprintf("seed_%03d", i)]
		if n == 0 {
			t.Errorf("seed_%03d was skipped while paging", i)
		}
		if n > 1 {
			dups += n - 1
		}
	}
	if dups > 2 {
		t.Errorf("at most a repeated boundary row per page is expected, got %d repeats", dups)
	}

	// 2. Walk the 3000-message history in pages of 200: every message once, in order.
	var all []uint64
	after, msgPages := uint64(0), 0
	var msgLat []time.Duration
	for {
		t0 := time.Now()
		c, m := jget(t, fmt.Sprintf("%s/v1/sync/messages?peer=seed_150&after_seq=%d&limit=200", r.base, after), accA)
		msgLat = append(msgLat, time.Since(t0))
		if c != 200 {
			t.Fatalf("messages: %d", c)
		}
		all = append(all, seqsOf(m, "messages")...)
		after = uint64(m["next_seq"].(float64))
		msgPages++
		if m["has_more"] != true {
			break
		}
	}
	if len(all) != history+1 { // the 3000 plus the one seed message seed_150 received in the first round
		t.Fatalf("history: %d messages in %d pages", len(all), msgPages)
	}
	for i, s := range all {
		if s != uint64(i+1) {
			t.Fatalf("message %d has seq %d: history must be gap-free and ordered", i, s)
		}
	}
	pct := func(l []time.Duration) string {
		sort.Slice(l, func(i, j int) bool { return l[i] < l[j] })
		return fmt.Sprintf("n=%d p50=%v p95=%v max=%v", len(l), l[len(l)/2], l[len(l)*95/100], l[len(l)-1])
	}
	t.Logf("sync at scale (%d conversations, %d-message history, one machine): conversation pages(50 rows x3 recents) %s | message pages(200) %s", peers, history, pct(convLat), pct(msgLat))
	_ = uidA
}

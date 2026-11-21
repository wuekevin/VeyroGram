package main

import (
	"fmt"
	"strings"
	"testing"
)

// Block list end to end: the blocked account cannot write to the blocker (and is told nothing more than "recipient unavailable"), the blocker still can,
// unblocking restores delivery, and the list API behaves.
func TestBlockListThroughRealIMAndAPI(t *testing.T) {
	r := newRig(t)
	accA, uidA, tokA := r.user("+8613800138501", "device-aaaaaaaaaaaaaaaa")
	accB, uidB, tokB := r.user("+8613800138502", "device-bbbbbbbbbbbbbbbb")
	wsA, wsB := r.ws(uidA, tokA, "im-a"), r.ws(uidB, tokB, "im-b")
	api := func(acc, from, to string) int {
		c, _ := jpost(t, r.base+"/v1/messages", fmt.Sprintf(`{"to_uid":%q,"client_msg_no":%q,"payload":{"type":1,"content":"x"}}`, to, ulid()), "Authorization", "Bearer "+acc)
		return c
	}
	if a := wsA.send(uidB, ulid(), text("before the block")); a.Reason != 1 {
		t.Fatalf("baseline send: %+v", a)
	}

	// B blocks A. Idempotent; the list shows A.
	for i := 0; i < 2; i++ {
		if c, _ := jsend(t, "PUT", r.base+"/v1/blocks/"+uidA, accB, ""); c != 204 {
			t.Fatalf("block #%d: %d", i, c)
		}
	}
	if _, l := jget(t, r.base+"/v1/blocks", accB); len(l["items"].([]any)) != 1 || l["items"].([]any)[0].(map[string]any)["uid"] != uidA {
		t.Fatalf("list: %v", l)
	}
	// the blocked sender is refused on both paths, with the same answer as for a missing account
	if a := wsA.send(uidB, ulid(), text("after the block")); a.Reason != 130 {
		t.Fatalf("a blocked sender must get 130 over the IM: %+v", a)
	}
	if c := api(accA, uidA, uidB); c != 404 {
		t.Fatalf("a blocked sender must get 404 recipient_unavailable from the API: %d", c)
	}
	// the blocker can still write to the blocked account; nobody else is affected
	if a := wsB.send(uidA, ulid(), text("I can still write")); a.Reason != 1 {
		t.Fatalf("the blocker keeps the ability to send: %+v", a)
	}
	accC, uidC, _ := r.user("+8613800138503", "device-cccccccccccccccc")
	if c := api(accC, uidC, uidB); c != 200 {
		t.Fatalf("a third party is not blocked: %d", c)
	}
	// block validation and isolation of lists
	for name, tc := range map[string]struct {
		acc, uid string
		want     int
	}{
		"yourself":        {accB, uidB, 400},
		"unknown":         {accB, "nobody_here", 404},
		"bad uid":         {accB, "a@b", 404}, // not a valid path-safe uid for the route semantics: unknown account
		"unauthenticated": {"", uidA, 401},
	} {
		if c, _ := jsend(t, "PUT", r.base+"/v1/blocks/"+tc.uid, tc.acc, ""); c != tc.want && !(name == "bad uid" && c == 400) {
			t.Errorf("%s: %d, want %d", name, c, tc.want)
		}
	}
	if _, l := jget(t, r.base+"/v1/blocks", accA); len(l["items"].([]any)) != 0 {
		t.Fatalf("A's list must be empty (lists are per user): %v", l)
	}

	// unblock restores delivery
	if c, _ := jsend(t, "DELETE", r.base+"/v1/blocks/"+uidA, accB, ""); c != 204 {
		t.Fatalf("unblock: %d", c)
	}
	if c, _ := jsend(t, "DELETE", r.base+"/v1/blocks/"+uidA, accB, ""); c != 204 {
		t.Fatalf("unblock is idempotent: %d", c)
	}
	if a := wsA.send(uidB, ulid(), text("after the unblock")); a.Reason != 1 {
		t.Fatalf("delivery restored: %+v", a)
	}
	_, m := get(t, "http://"+r.p.internal+"/metrics", "Bearer metrics-token-0123456789abcdef")
	if !strings.Contains(m, `vg_message_gate_total{rule="block",result="deny"}`) {
		t.Errorf("block denials must be visible in metrics")
	}
}

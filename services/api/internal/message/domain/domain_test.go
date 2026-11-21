package domain

import (
	"strings"
	"testing"
)

func TestPeer(t *testing.T) {
	for _, c := range []struct {
		from, ch, peer string
		ok             bool
	}{
		{"alice", "bob", "bob", true},       // bare id (server-side callers)
		{"alice", "alice@bob", "bob", true}, // composite id handed to the hook, either order
		{"alice", "bob@alice", "bob", true},
		{"alice", "alice@alice", "alice", true}, // saved-messages style self conversation
		{"mallory", "alice@bob", "", false},     // not a participant
		{"alice", "a@b@c", "", false},
		{"alice", "", "", false},
		{"alice", "bob@", "", false},
		{"alice", "bo b", "", false},
		{"", "bob", "", false},
	} {
		if p, ok := Peer(c.from, c.ch); ok != c.ok || p != c.peer {
			t.Errorf("Peer(%q,%q) = %q,%v want %q,%v", c.from, c.ch, p, ok, c.peer, c.ok)
		}
	}
}

func TestCheckPayload(t *testing.T) {
	big := `{"type":1,"content":"` + strings.Repeat("a", MaxPayloadBytes) + `"}`
	manyRunes := `{"type":1,"content":"` + strings.Repeat("好", MaxTextRunes+1) + `"}`
	for _, c := range []struct {
		name, in string
		want     uint8
	}{
		{"text", `{"type":1,"content":"hi"}`, 0},
		{"text with 4000 CJK runes", `{"type":1,"content":"` + strings.Repeat("好", MaxTextRunes) + `"}`, 0},
		{"too many runes", manyRunes, ReasonInvalidPayload},
		{"unknown type passes", `{"type":4242,"schema_version":9,"x":[1]}`, 0},
		{"no type", `{"content":"hi"}`, ReasonInvalidPayload},
		{"type zero", `{"type":0}`, ReasonInvalidPayload},
		{"type is a string", `{"type":"1","content":"hi"}`, ReasonInvalidPayload},
		{"blank text", `{"type":1,"content":"  \n"}`, ReasonInvalidPayload},
		{"text without content", `{"type":1}`, ReasonInvalidPayload},
		{"not an object", `[1,2]`, ReasonInvalidPayload},
		{"null", `null`, ReasonInvalidPayload},
		{"garbage", `hello`, ReasonInvalidPayload},
		{"empty", ``, ReasonInvalidPayload},
		{"too large", big, ReasonPayloadTooLarge},
	} {
		if got := CheckPayload([]byte(c.in)); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}

func TestULIDAndUID(t *testing.T) {
	for s, ok := range map[string]bool{
		"01HZX3J9K2M4N6P8Q0R2S4T6V8": true,
		"7ZZZZZZZZZZZZZZZZZZZZZZZZZ": true,
		"81HZX3J9K2M4N6P8Q0R2S4T6V8": false, // timestamp overflow
		"01hzx3j9k2m4n6p8q0r2s4t6v8": false, // lower case
		"01HZX3J9K2M4N6P8Q0R2S4T6VI": false, // I is not Crockford
		"01HZX3J9K2M4N6P8Q0R2S4T6V":  false, // short
		"":                           false,
	} {
		if ValidULID(s) != ok {
			t.Errorf("ValidULID(%q) != %v", s, ok)
		}
	}
	if !ValidUID("u_abc-123") || ValidUID("a b") || ValidUID(strings.Repeat("a", 41)) || ValidUID("a@b") {
		t.Error("uid alphabet/length rules")
	}
}

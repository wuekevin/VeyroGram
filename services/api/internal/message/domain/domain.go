// Package domain holds the message module's rules: what a sendable one-to-one message is and the reason codes that explain a refusal.
package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

// Business reason codes (WuKongIM reserves 128-255 for business use; the IM relays them to the sender's SENDACK unchanged).
const (
	ReasonUnsupportedChannel   uint8 = 128
	ReasonSenderUnavailable    uint8 = 129
	ReasonRecipientUnavailable uint8 = 130
	ReasonInvalidPayload       uint8 = 131
	ReasonPayloadTooLarge      uint8 = 132
	ReasonRateLimited          uint8 = 133
	ReasonUnavailable          uint8 = 200 // the policy could not be evaluated in time: fail closed
)

// ChannelPerson is WuKongIM's one-to-one channel type.
const ChannelPerson uint8 = 1

// SystemUID sends account notices (identity.device_notice). It must equal identity's domain.SystemUID; a test in cmd/api pins that.
const SystemUID = "vg_system"

const (
	MaxPayloadBytes = 16 << 10
	MaxTextRunes    = 4000
	TypeText        = 1
)

// Send is one message about to be accepted by the IM.
type Send struct {
	From        string
	ChannelID   string
	ChannelType uint8
	ClientMsgNo string
	Payload     []byte
}

type Verdict struct {
	Allow  bool
	Reason uint8
}

var (
	Allow = Verdict{Allow: true}
	// ErrInvalidInput is returned for malformed API input (the caller's fault, 400).
	ErrInvalidInput = errors.New("message: invalid input")
	// ErrIMUnavailable means the IM could not be reached; the send had no effect and is safe to retry with the same client_msg_no.
	ErrIMUnavailable = errors.New("message: im unavailable")
)

// Rejected is a refusal by the policy gate or by the IM itself (send ban, blacklist, ...).
type Rejected struct {
	Reason uint8
	Native bool // true when the IM refused on its own rules rather than our gate
}

func (e *Rejected) Error() string { return "message: rejected, reason code " + itoa(int(e.Reason)) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for ; n > 0; n /= 10 {
		i--
		b[i] = byte('0' + n%10)
	}
	return string(b[i:])
}

// ValidUID reports whether s looks like an account uid (the same alphabet the identity module issues).
func ValidUID(s string) bool {
	if s == "" || len(s) > 40 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// ValidULID accepts a 26-character Crockford base32 ULID (the client_msg_no the clients generate, AD-4).
func ValidULID(s string) bool {
	if len(s) != 26 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' && c != 'I' && c != 'L' && c != 'O' && c != 'U') {
			return false
		}
	}
	return s[0] <= '7' // a ULID's 48-bit timestamp keeps the first character within 0-7
}

// Peer resolves the other participant of a person channel. WuKongIM hands hooks the composite "a@b" id while server-side callers pass the bare
// peer uid, so both are accepted; the sender must be one of the two participants (a user can never post into someone else's conversation).
func Peer(from, channelID string) (string, bool) {
	if !strings.Contains(channelID, "@") {
		if !ValidUID(channelID) || !ValidUID(from) {
			return "", false
		}
		return channelID, true
	}
	parts := strings.Split(channelID, "@")
	if len(parts) != 2 || !ValidUID(parts[0]) || !ValidUID(parts[1]) || !ValidUID(from) {
		return "", false
	}
	switch from {
	case parts[0]:
		return parts[1], true
	case parts[1]:
		return parts[0], true
	}
	return "", false
}

// CheckPayload validates the message body: a JSON object with an integer `type`. Text (type 1) needs a non-empty `content`. Unknown types pass
// untouched (AD-4: unknown types degrade on the client, they are never rejected by the server). It returns 0 when the payload is acceptable.
func CheckPayload(b []byte) uint8 {
	if len(b) > MaxPayloadBytes {
		return ReasonPayloadTooLarge
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		return ReasonInvalidPayload
	}
	var typ int
	if json.Unmarshal(m["type"], &typ) != nil || typ <= 0 {
		return ReasonInvalidPayload
	}
	if typ == TypeText {
		var content string
		if json.Unmarshal(m["content"], &content) != nil || strings.TrimSpace(content) == "" || utf8.RuneCountInString(content) > MaxTextRunes {
			return ReasonInvalidPayload
		}
	}
	return 0
}

// PersonChannel is the canonical storage id of the conversation between two accounts: the two uids, ordered, joined by "@". The IM's own composite id
// is not ordered the same way, so every writer of message_ref normalises through this function and the unique key sees one conversation.
func PersonChannel(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "@" + b
}

// NotifiedPerson extracts the canonical conversation id from a composite IM person-channel id; ok is false for anything that is not a two-party
// conversation including the sender.
func NotifiedPerson(from, channelID string) (string, bool) {
	peer, ok := Peer(from, channelID)
	if !ok {
		return "", false
	}
	return PersonChannel(from, peer), true
}

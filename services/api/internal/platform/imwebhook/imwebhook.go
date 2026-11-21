// Package imwebhook receives WuKongIM callbacks on the internal listener (ARCHITECTURE AD-2). Stock WuKongIM signs nothing, so the handler enforces:
// an unguessable path secret (constant-time compare), a source-address allowlist, POST + JSON only, a body size cap, and strict decoding.
// With SigningSecret set (requires the VeyroGram-patched WuKongIM, patches/wukongim/0001) it also requires a valid HMAC-SHA256 signature
// and a fresh timestamp, so a caller that learns the path secret still cannot forge or replay callbacks.
// Anything else is 403/4xx and reported through OnReject so it can raise an alert.
package imwebhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

const (
	MaxBody = 64 << 10 // before_send and unknown events
	// MaxNotifyBody bounds a msg.notify batch (the IM batches up to 100 messages of up to 16 KiB each; a rejected batch would be retried and eventually lost).
	MaxNotifyBody = 4 << 20
)

// Notified is one committed message from a msg.notify batch. ChannelID is the IM's composite person-channel id ("a@b"). Payload is decoded from base64.
type Notified struct {
	MessageID   int64  `json:"message_id"`
	MessageSeq  uint64 `json:"message_seq"`
	ClientMsgNo string `json:"client_msg_no"`
	FromUID     string `json:"from_uid"`
	ChannelID   string `json:"channel_id"`
	ChannelType uint8  `json:"channel_type"`
	Timestamp   int64  `json:"timestamp"`
	Payload     []byte `json:"payload"`
}

// BeforeSend is the synchronous send gate (AD-21). Allow=false rejects with ReasonCode (business codes 128–255).
type BeforeSend struct {
	FromUID     string `json:"from_uid"`
	ChannelID   string `json:"channel_id"`
	ChannelType uint8  `json:"channel_type"`
	ClientMsgNo string `json:"client_msg_no"`
	Payload     []byte `json:"payload"`
	NoPersist   bool   `json:"no_persist"`
	SyncOnce    bool   `json:"sync_once"`
}

// Decision is the wire answer WuKongIM expects. Payload nil means "keep the original".
type Decision struct {
	Allow      bool   `json:"allow"`
	Payload    []byte `json:"payload,omitempty"`
	ReasonCode uint8  `json:"reason_code,omitempty"`
}

// DenyUnavailable is the reason code used when no policy is installed or it fails (fail-closed).
const DenyUnavailable uint8 = 200

// Handlers are supplied by later stories (message, risk). A nil BeforeSend denies every send: fail-closed.
type Handlers struct {
	BeforeSend func(context.Context, BeforeSend) (Decision, error)
	// Notify consumes committed messages (msg.notify). An error answers 500 so the IM retries the batch; nil means events are acknowledged and ignored.
	Notify func(context.Context, []Notified) error
}

// Config configures the handler.
type Config struct {
	Secret string
	// SigningSecret, when set, makes a valid X-WK-Signature/X-WK-Timestamp mandatory (reject reason "bad_signature").
	SigningSecret string
	MaxSkew       time.Duration // allowed clock difference for the timestamp; default 5 minutes
	Now           func() time.Time
	Allow         []netip.Prefix // allowed source addresses; empty means loopback only
	Handlers      Handlers
	OnReject      func(reason string) // "bad_secret" | "source" | "method" | "content_type" | "body" ...
}

// Signature headers set by the patched WuKongIM (pkg/webhooksig): HMAC-SHA256(secret, timestamp + "." + body), hex, prefixed "v1=".
const (
	HeaderTimestamp = "X-WK-Timestamp"
	HeaderSignature = "X-WK-Signature"
)

func validSignature(secret, timestamp, signature string, body []byte, now time.Time, skew time.Duration) bool {
	sec, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || secret == "" {
		return false
	}
	if d := now.Sub(time.Unix(sec, 0)); d > skew || d < -skew {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hmac.Equal([]byte("v1="+hex.EncodeToString(mac.Sum(nil))), []byte(signature))
}

var loopback = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")}

// ParseAllow parses a comma-separated CIDR/IP list.
func ParseAllow(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		if pf, err := netip.ParsePrefix(p); err == nil {
			out = append(out, pf)
		} else if a, err := netip.ParseAddr(p); err == nil {
			out = append(out, netip.PrefixFrom(a, a.BitLen()))
		} else {
			return nil, errors.New("imwebhook: bad allowlist entry " + p)
		}
	}
	return out, nil
}

// Handler serves POST /im/webhook/{secret}?event=<name>. With an empty secret every request is 404 (feature disabled, never open).
func Handler(c Config) http.Handler {
	allow := c.Allow
	if len(allow) == 0 {
		allow = loopback
	}
	reject := func(w http.ResponseWriter, status int, reason string) {
		if c.OnReject != nil {
			c.OnReject(reason)
		}
		http.Error(w, http.StatusText(status), status)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.Secret == "" {
			http.NotFound(w, r)
			return
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		addr, perr := netip.ParseAddr(host)
		ok := false
		if err == nil && perr == nil {
			addr = addr.Unmap()
			for _, p := range allow {
				if p.Contains(addr) {
					ok = true
					break
				}
			}
		}
		if !ok {
			reject(w, http.StatusForbidden, "source")
			return
		}
		const prefix = "/im/webhook/"
		secret, found := strings.CutPrefix(r.URL.Path, prefix)
		if !found || subtle.ConstantTimeCompare([]byte(secret), []byte(c.Secret)) != 1 {
			reject(w, http.StatusForbidden, "bad_secret")
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			reject(w, http.StatusMethodNotAllowed, "method")
			return
		}
		if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
			reject(w, http.StatusUnsupportedMediaType, "content_type")
			return
		}
		bodyMax := MaxBody
		if r.URL.Query().Get("event") == "msg.notify" {
			bodyMax = MaxNotifyBody
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, int64(bodyMax)+1))
		if err != nil || len(body) > bodyMax {
			reject(w, http.StatusRequestEntityTooLarge, "body")
			return
		}
		if c.SigningSecret != "" {
			now, skew := time.Now(), c.MaxSkew
			if c.Now != nil {
				now = c.Now()
			}
			if skew <= 0 {
				skew = 5 * time.Minute
			}
			if !validSignature(c.SigningSecret, r.Header.Get(HeaderTimestamp), r.Header.Get(HeaderSignature), body, now, skew) {
				reject(w, http.StatusForbidden, "bad_signature")
				return
			}
		}
		switch r.URL.Query().Get("event") {
		case "msg.before_send":
			var in BeforeSend
			if err := json.Unmarshal(body, &in); err != nil || in.FromUID == "" || in.ChannelID == "" {
				reject(w, http.StatusBadRequest, "malformed")
				return
			}
			d := Decision{Allow: false, ReasonCode: DenyUnavailable}
			if c.Handlers.BeforeSend != nil {
				if got, err := c.Handlers.BeforeSend(r.Context(), in); err == nil {
					d = got
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(d)
		case "msg.notify":
			if c.Handlers.Notify == nil {
				w.WriteHeader(http.StatusOK)
				return
			}
			var batch []Notified
			if err := json.Unmarshal(body, &batch); err != nil {
				reject(w, http.StatusBadRequest, "malformed")
				return
			}
			if err := c.Handlers.Notify(r.Context(), batch); err != nil {
				http.Error(w, "retry", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			// other notifications (msg.notify, user.onlinestatus, ...) are acknowledged and ignored until a story consumes them
			w.WriteHeader(http.StatusOK)
		}
	})
}

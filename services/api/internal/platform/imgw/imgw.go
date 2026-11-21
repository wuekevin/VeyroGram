// Package imgw is the only client of the WuKongIM HTTP API (ARCHITECTURE AD-2). The IM API never leaves loopback/private networks; calls are
// retried on transport errors and 5xx (all wrapped operations are idempotent) and every attempt outcome is reported to an Observer.
package imgw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"veyrogram/api/internal/platform/logx"
)

// Observer receives one event per call (after retries): op, result ("ok"|"error"|"retried_ok"), duration.
type Observer func(op, result string, d time.Duration)

// Client talks to the IM API.
type Client struct {
	base     *url.URL
	HTTP     *http.Client
	Retries  int           // extra attempts after the first (default 2)
	Backoff  time.Duration // base delay, doubled each retry (default 100ms)
	Observe  Observer
	Sleep    func(time.Duration) // test hook
	AllowAny bool                // tests only: skip the private-network check
	peers    []*Client           // the other nodes of an IM cluster (see AddPeers)
}

// ErrNotPrivate is returned when the IM API URL points at a public host.
var ErrNotPrivate = errors.New("imgw: the IM API must be a loopback or private address (AD-2)")

// APIError is a non-2xx answer from the IM.
type APIError struct {
	Op     string
	Status int
	Msg    string
}

func (e *APIError) Error() string { return fmt.Sprintf("imgw %s: HTTP %d: %s", e.Op, e.Status, e.Msg) }

// New validates baseURL and returns a client.
func New(baseURL string) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("imgw: bad base URL %q", baseURL)
	}
	if !privateHost(u.Hostname()) {
		return nil, ErrNotPrivate
	}
	return &Client{base: u, HTTP: &http.Client{Timeout: 5 * time.Second}}, nil
}

// AddPeers registers the API URLs of the other nodes of an IM cluster. A connection lives on the node that accepted it and /user/device_quit only closes
// connections owned by the node that receives the call (verified against a real three-node cluster: TestClusterDeviceQuitReachesASessionOwnedByAnotherNode),
// so DeviceQuit is sent to every node. Everything else (tokens, sends, sync) is cluster-wide state and goes to the primary URL only.
func (c *Client) AddPeers(urls ...string) error {
	for _, u := range urls {
		p, err := New(u)
		if err != nil {
			return err
		}
		p.Retries, p.Backoff, p.Observe, p.Sleep, p.AllowAny = c.Retries, c.Backoff, c.Observe, c.Sleep, c.AllowAny
		p.HTTP = c.HTTP
		c.peers = append(c.peers, p)
	}
	return nil
}

func privateHost(h string) bool {
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && !ip.IsUnspecified() && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}

func (c *Client) do(ctx context.Context, op, method, path string, in any, out any) error {
	start := time.Now()
	var body []byte
	if in != nil {
		var err error
		if body, err = json.Marshal(in); err != nil {
			return err
		}
	}
	retries, backoff, sleep := c.Retries, c.Backoff, c.Sleep
	if retries == 0 {
		retries = 2
	}
	if retries < 0 {
		retries = 0
	}
	if backoff == 0 {
		backoff = 100 * time.Millisecond
	}
	if sleep == nil {
		sleep = time.Sleep
	}
	var last error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			sleep(backoff << (attempt - 1))
		}
		if err := ctx.Err(); err != nil {
			last = err
			break
		}
		last = c.once(ctx, op, method, path, body, out)
		if last == nil {
			res := "ok"
			if attempt > 0 {
				res = "retried_ok"
			}
			c.report(op, res, start)
			return nil
		}
		var ae *APIError
		if errors.As(last, &ae) && ae.Status < 500 {
			break // client errors are not retried
		}
	}
	c.report(op, "error", start)
	return last
}

func (c *Client) report(op, result string, start time.Time) {
	if c.Observe != nil {
		c.Observe(op, result, time.Since(start))
	}
}

func (c *Client) once(ctx context.Context, op, method, path string, body []byte, out any) error {
	u := *c.base
	u.Path = strings.TrimRight(u.Path, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if id := logx.Trace(ctx); id != "" {
		req.Header.Set("X-Request-Id", id)
		if tp := traceparent(id); tp != "" {
			req.Header.Set("traceparent", tp)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("imgw %s: %w", op, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		msg := strings.TrimSpace(string(raw))
		var m struct {
			Msg string `json:"msg"`
		}
		if json.Unmarshal(raw, &m) == nil && m.Msg != "" {
			msg = m.Msg
		}
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return &APIError{Op: op, Status: resp.StatusCode, Msg: msg}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("imgw %s: bad response: %w", op, err)
		}
	}
	return nil
}

// Health reports whether the IM answers its health endpoint.
func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, "health", http.MethodGet, "/healthz", nil, nil)
}

// UpdateToken registers the IM connection token for (uid, deviceFlag) (AD-9: tokens are per user and device flag).
func (c *Client) UpdateToken(ctx context.Context, uid, token string, deviceFlag, deviceLevel uint8) error {
	if uid == "" || token == "" {
		return errors.New("imgw: uid and token are required")
	}
	return c.do(ctx, "update_token", http.MethodPost, "/user/token", map[string]any{"uid": uid, "token": token, "device_flag": deviceFlag, "device_level": deviceLevel}, nil)
}

// DeviceQuit kicks the connections of uid for deviceFlag (255 = all devices; owner-local use only, see AD-9).
func (c *Client) DeviceQuit(ctx context.Context, uid string, deviceFlag uint8) error {
	if uid == "" {
		return errors.New("imgw: uid is required")
	}
	// Every node must acknowledge: a node that cannot be reached may still hold the session, and the caller (the outbox worker) retries until all do.
	var errs []error
	for _, n := range append([]*Client{c}, c.peers...) {
		if err := n.do(ctx, "device_quit", http.MethodPost, "/user/device_quit", map[string]any{"uid": uid, "device_flag": deviceFlag}, nil); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// SetUserSendBan mutes (ban=true) or unmutes a user with the IM's native enforcement (AD-21).
func (c *Client) SetUserSendBan(ctx context.Context, uid string, ban bool) error {
	if uid == "" {
		return errors.New("imgw: uid is required")
	}
	v := int64(0)
	if ban {
		v = 1
	}
	return c.do(ctx, "send_ban", http.MethodPost, "/user/send_ban", map[string]any{"uid": uid, "send_ban": v}, nil)
}

// SendResult is the IM acknowledgement of a server-side send.
type SendResult struct {
	ClientMsgNo string `json:"client_msg_no"`
	MessageID   int64  `json:"message_id"`
	MessageSeq  uint64 `json:"message_seq"`
	Reason      uint8  `json:"reason"`
}

// ReasonSuccess is the IM's SENDACK success code. Any other value means the message was NOT accepted even though the HTTP status is 200
// (verified against the pinned WuKongIM: a muted sender gets HTTP 200 with reason 25).
const ReasonSuccess uint8 = 1

// SendRejected is returned when the IM answered 200 but refused the message.
type SendRejected struct{ Reason uint8 }

func (e *SendRejected) Error() string {
	return fmt.Sprintf("imgw send: message rejected by IM, reason code %d", e.Reason)
}

// Send delivers a message as fromUID. clientMsgNo is mandatory: WuKongIM de-duplicates on it, which is what makes retries safe.
func (c *Client) Send(ctx context.Context, fromUID, channelID string, channelType uint8, clientMsgNo string, payload []byte) (SendResult, error) {
	var r SendResult
	if clientMsgNo == "" {
		return r, errors.New("imgw: client_msg_no is required so retries are idempotent")
	}
	err := c.do(ctx, "send", http.MethodPost, "/message/send", map[string]any{
		"from_uid": fromUID, "channel_id": channelID, "channel_type": channelType, "client_msg_no": clientMsgNo, "payload": payload,
	}, &r)
	if err == nil && r.Reason != ReasonSuccess {
		return r, &SendRejected{Reason: r.Reason}
	}
	return r, err
}

// traceparent derives a W3C traceparent from a hex trace id (left-padded to 32 hex digits). Non-hex ids yield "".
func traceparent(id string) string {
	id = strings.ReplaceAll(strings.ToLower(id), "-", "")
	if len(id) == 0 || len(id) > 32 {
		return ""
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return ""
		}
	}
	id = strings.Repeat("0", 32-len(id)) + id
	if strings.Trim(id, "0") == "" {
		return ""
	}
	return "00-" + id + "-0000000000000001-01"
}

// SyncedMessage is one message as the IM returns it from its sync endpoints. Payload is the raw message body (base64 on the wire, decoded here).
type SyncedMessage struct {
	MessageID   int64  `json:"message_id"`
	MessageSeq  uint64 `json:"message_seq"`
	ClientMsgNo string `json:"client_msg_no"`
	FromUID     string `json:"from_uid"`
	ChannelID   string `json:"channel_id"`
	ChannelType uint8  `json:"channel_type"`
	Timestamp   int32  `json:"timestamp"`
	Payload     []byte `json:"payload"`
}

// SyncedConversation is one row of the IM's conversation list for a user. Unread and ReadToMessageSeq are the IM's per-user truth, so every device of
// the user sees the same numbers.
type SyncedConversation struct {
	ChannelID        string          `json:"channel_id"`
	ChannelType      uint8           `json:"channel_type"`
	Unread           uint64          `json:"unread"`
	Timestamp        int64           `json:"timestamp"`
	LastMessageSeq   uint64          `json:"last_msg_seq"`
	LastClientMsgNo  string          `json:"last_client_msg_no"`
	ReadToMessageSeq uint64          `json:"readed_to_msg_seq"`
	Recents          []SyncedMessage `json:"recents"`
}

// ConversationSync lists the user's conversations, most recently active first, one-based page of pageSize (the IM caps it at 500), each with up to
// msgCount recent messages.
func (c *Client) ConversationSync(ctx context.Context, uid string, msgCount, page, pageSize int) ([]SyncedConversation, error) {
	var out []SyncedConversation
	err := c.do(ctx, "conversation_sync", http.MethodPost, "/conversation/sync", map[string]any{
		"uid": uid, "msg_count": msgCount, "page": page, "page_size": pageSize}, &out)
	return out, err
}

// ChannelMessages is one page of a channel's history.
type ChannelMessages struct {
	More     bool
	Messages []SyncedMessage
}

// ChannelMessageSync reads a page of the conversation between loginUID and the person channel peer. up=true pulls messages at or after startSeq in
// ascending order; up=false pulls older messages at or before startSeq. For person channels the IM resolves the pair from loginUID and the peer uid,
// so the caller can only ever read their own conversations.
func (c *Client) ChannelMessageSync(ctx context.Context, loginUID, peer string, startSeq uint64, limit int, up bool) (ChannelMessages, error) {
	mode := 0
	if up {
		mode = 1
	}
	var r struct {
		More     int             `json:"more"`
		Messages []SyncedMessage `json:"messages"`
	}
	err := c.do(ctx, "channel_message_sync", http.MethodPost, "/channel/messagesync", map[string]any{
		"login_uid": loginUID, "channel_id": peer, "channel_type": 1, "start_message_seq": startSeq, "limit": limit, "pull_mode": mode}, &r)
	return ChannelMessages{More: r.More == 1, Messages: r.Messages}, err
}

// ClearUnread marks the user's conversation as read up to its latest message.
func (c *Client) ClearUnread(ctx context.Context, uid, channelID string, channelType uint8) error {
	return c.do(ctx, "clear_unread", http.MethodPost, "/conversations/clearUnread", map[string]any{
		"uid": uid, "channel_id": channelID, "channel_type": channelType}, nil)
}

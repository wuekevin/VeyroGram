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
	return c.do(ctx, "device_quit", http.MethodPost, "/user/device_quit", map[string]any{"uid": uid, "device_flag": deviceFlag}, nil)
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

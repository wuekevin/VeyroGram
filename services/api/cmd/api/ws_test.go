package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// wsClient is a minimal WuKongIM JSON-RPC-over-WebSocket client (the dialect the Android EasySDK speaks: text frames, snake_case), enough to prove
// what a real client sees: connect accepted or refused, being kicked, and receiving a message.
type wsClient struct {
	t    *testing.T
	conn *websocket.Conn
	in   chan rpcMsg // everything the server sends, in order; closed when the connection ends
}

type rpcMsg struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func dialWS(t *testing.T, url string) *wsClient {
	t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("ws dial %s: %v", url, err)
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	c := &wsClient{t: t, conn: conn, in: make(chan rpcMsg, 64)}
	go func() {
		defer close(c.in)
		for {
			_, b, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var m rpcMsg
			if json.Unmarshal(b, &m) == nil {
				c.in <- m
			}
		}
	}()
	t.Cleanup(func() { conn.Close() })
	return c
}

// connect sends CONNECT and reports whether the IM accepted it (reason code 1). A refused connect is a normal outcome here, not a test failure.
func (c *wsClient) connect(uid, token, deviceID string, flag int) bool {
	c.t.Helper()
	id := fmt.Sprintf("c-%d", time.Now().UnixNano())
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": "connect", "params": map[string]any{
		"uid": uid, "token": token, "device_id": deviceID, "device_flag": flag, "client_timestamp": time.Now().UnixMilli()}})
	if err := c.conn.WriteMessage(websocket.TextMessage, req); err != nil {
		return false
	}
	deadline := time.After(8 * time.Second)
	for {
		select {
		case m, ok := <-c.in:
			if !ok {
				return false
			}
			if string(m.ID) != `"`+id+`"` {
				continue
			}
			var r struct {
				ReasonCode int `json:"reason_code"`
			}
			return len(m.Error) == 0 && json.Unmarshal(m.Result, &r) == nil && r.ReasonCode == 1
		case <-deadline:
			c.t.Fatal("ws connect: no answer within 8s")
		}
	}
}

// kicked waits for the server to end the connection (or any read error), which is how a revoked device's session shows up on the wire.
func (c *wsClient) kicked(within time.Duration) bool {
	deadline := time.After(within)
	for {
		select {
		case _, ok := <-c.in:
			if !ok {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// recvNotice waits for a `recv` push and returns its decoded payload and sender.
func (c *wsClient) recvNotice(within time.Duration) (from string, payload map[string]any, ok bool) {
	deadline := time.After(within)
	for {
		select {
		case m, open := <-c.in:
			if !open {
				return "", nil, false
			}
			if m.Method != "recv" {
				continue
			}
			var p struct {
				FromUID string          `json:"fromUid"`
				Payload json.RawMessage `json:"payload"`
			}
			if json.Unmarshal(m.Params, &p) != nil {
				return "", nil, false
			}
			var s string
			raw := []byte(p.Payload)
			if json.Unmarshal(p.Payload, &s) == nil { // some dialects wrap the JSON in a string
				raw = []byte(s)
			}
			var out map[string]any
			if json.Unmarshal(raw, &out) != nil {
				return p.FromUID, nil, false
			}
			return p.FromUID, out, true
		case <-deadline:
			return "", nil, false
		}
	}
}

package imgw_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"veyrogram/api/internal/platform/testim"
)

// connectWS opens a JSON-RPC WebSocket to one IM node and returns whether CONNECT was accepted plus a channel that closes when the node drops the connection.
func connectWS(t *testing.T, im *testim.IM, uid, token, device string) (ok bool, dropped <-chan struct{}) {
	t.Helper()
	var conn *websocket.Conn
	var err error
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(300 * time.Millisecond) { // the gateway listener comes up after the API in a cluster
		if conn, _, err = websocket.DefaultDialer.Dial(fmt.Sprintf("ws://127.0.0.1:%d", im.Ports["ws"]), nil); err == nil {
			break
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "c", "method": "connect", "params": map[string]any{
		"uid": uid, "token": token, "device_id": device, "device_flag": 1, "client_timestamp": time.Now().UnixMilli()}})
	_ = conn.WriteMessage(websocket.TextMessage, req)
	ch := make(chan struct{})
	accepted := make(chan bool, 1)
	go func() {
		defer close(ch)
		first := true
		for {
			_, b, err := conn.ReadMessage()
			if err != nil {
				if first {
					accepted <- false
				}
				return
			}
			if first {
				var m struct {
					Result struct {
						ReasonCode int `json:"reason_code"`
					} `json:"result"`
				}
				_ = json.Unmarshal(b, &m)
				accepted <- m.Result.ReasonCode == 1
				first = false
			}
		}
	}()
	select {
	case ok = <-accepted:
	case <-time.After(8 * time.Second):
		t.Fatal("connect: no answer")
	}
	return ok, ch
}

// What a real three-node cluster does with VeyroGram's revocation calls (AD-9). The user is connected to node 2; the business service talks to node 1.
func TestClusterDeviceQuitNeedsFanOutToReachASessionOwnedByAnotherNode(t *testing.T) {
	ims := testim.StartCluster(t, 3, testim.Options{})
	primary := newClient(t, ims[0])
	ctx := context.Background()
	if err := primary.UpdateToken(ctx, "alice", "tok-1", 1, 1); err != nil {
		t.Fatal(err)
	}
	// user state is cluster-wide: a token registered through node 1 authenticates a connection to node 2
	ok, dropped := connectWS(t, ims[1], "alice", "tok-1", "dev-1")
	if !ok {
		t.Fatal("a token registered through node 1 must authenticate a connection to node 2")
	}

	// characterisation of the pinned build: device_quit sent to node 1 alone does NOT close the session owned by node 2. If a future pin starts
	// propagating, this log line changes and the fan-out below becomes redundant (it stays harmless).
	if err := primary.DeviceQuit(ctx, "alice", 1); err != nil {
		t.Fatal(err)
	}
	select {
	case <-dropped:
		t.Log("NOTE: device_quit on node 1 reached node 2 by itself (upstream propagates now)")
	case <-time.After(4 * time.Second):
		t.Log("confirmed: device_quit on node 1 alone leaves the session on node 2 alive")
	}

	// the requirement: with the other nodes registered as peers, one DeviceQuit ends the session wherever it lives
	fan := newClient(t, ims[0])
	if err := fan.AddPeers(ims[1].APIURL, ims[2].APIURL); err != nil {
		t.Fatal(err)
	}
	select {
	case <-dropped:
	default:
		if err := fan.DeviceQuit(ctx, "alice", 1); err != nil {
			t.Fatal(err)
		}
		select {
		case <-dropped:
		case <-time.After(10 * time.Second):
			t.Fatal("the session on node 2 survived a fanned-out device_quit")
		}
	}
	// then the worker's second step: the new token, cluster-wide. The old token is dead everywhere, the new one works anywhere.
	if err := fan.UpdateToken(ctx, "alice", "tok-2", 1, 1); err != nil {
		t.Fatal(err)
	}
	for i, im := range ims {
		if ok, _ := connectWS(t, im, "alice", "tok-1", fmt.Sprintf("dev-old-%d", i)); ok {
			t.Errorf("node %d still accepts the revoked token", i+1)
		}
	}
	if ok, _ := connectWS(t, ims[2], "alice", "tok-2", "dev-2"); !ok {
		t.Error("the fresh token must work on node 3")
	}

	// a node that cannot be reached must make DeviceQuit fail, so the outbox retries instead of silently leaving a session alive
	ims[2].Stop()
	if err := fan.DeviceQuit(ctx, "alice", 1); err == nil {
		t.Fatal("DeviceQuit must fail while a cluster node is unreachable")
	}
}

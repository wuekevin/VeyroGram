// Package http serves /v1/sync/*: how a device catches up after being offline or after first sign-in (Story 1.16). Every handler takes the user from the
// verified access token and never from the request, so a caller can only ever read and change their own state.
package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"veyrogram/api/internal/conversation/app"
	"veyrogram/api/internal/conversation/domain"
	"veyrogram/api/internal/conversation/ports"
	"veyrogram/api/internal/platform/authn"
	"veyrogram/api/internal/platform/httpx"
)

type Handler struct {
	Svc  *app.Service
	Log  *slog.Logger
	Auth func(http.Handler) http.Handler
}

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("/v1/sync/", httpx.NotFound)
	if h.Auth == nil {
		return
	}
	mux.Handle("GET /v1/sync/conversations", h.Auth(http.HandlerFunc(h.conversations)))
	mux.Handle("GET /v1/sync/messages", h.Auth(http.HandlerFunc(h.messages)))
	mux.Handle("POST /v1/sync/read", h.Auth(http.HandlerFunc(h.read)))
	mux.Handle("PUT /v1/sync/drafts", h.Auth(http.HandlerFunc(h.putDraft)))
	mux.Handle("PUT /v1/sync/conversation-settings", h.Auth(http.HandlerFunc(h.putSetting)))
	mux.Handle("GET /v1/sync/state", h.Auth(http.HandlerFunc(h.state)))
}

func (h *Handler) uid(r *http.Request) string {
	c, _ := authn.FromContext(r.Context())
	return c.Subject
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil || dec.More() {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
		return false
	}
	return true
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request")
	case errors.Is(err, domain.ErrNotFound):
		httpx.WriteError(w, r, http.StatusNotFound, "not_found", "no such resource")
	case errors.Is(err, domain.ErrIMUnavailable):
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "im_unavailable", "the messaging service is unavailable, try again later")
	default:
		if h.Log != nil {
			h.Log.Error("sync request failed", "err", err.Error(), "trace_id", r.Header.Get("X-Request-Id"))
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal", "internal error")
	}
}

// intParam parses an optional non-negative integer query parameter; a malformed value is the caller's error.
func intParam(r *http.Request, name string) (uint64, bool) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return 0, true
	}
	v, err := strconv.ParseUint(s, 10, 63)
	return v, err == nil
}

type messageOut struct {
	MessageID   string          `json:"message_id"`
	MessageSeq  uint64          `json:"message_seq"`
	ClientMsgNo string          `json:"client_msg_no"`
	FromUID     string          `json:"from_uid"`
	Timestamp   int32           `json:"timestamp"`
	Payload     json.RawMessage `json:"payload"`
}

func msgOut(m ports.Message) messageOut {
	p := json.RawMessage("null")
	if json.Valid(m.Payload) {
		p = m.Payload
	}
	return messageOut{strconv.FormatInt(m.MessageID, 10), m.MessageSeq, m.ClientMsgNo, m.FromUID, m.Timestamp, p}
}

// GET /v1/sync/conversations?cursor=&limit=&msg_count=
func (h *Handler) conversations(w http.ResponseWriter, r *http.Request) {
	limit, ok1 := intParam(r, "limit")
	mc, ok2 := intParam(r, "msg_count")
	if !ok1 || !ok2 {
		h.fail(w, r, domain.ErrInvalidInput)
		return
	}
	p, err := h.Svc.Conversations(r.Context(), h.uid(r), r.URL.Query().Get("cursor"), int(limit), int(mc))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	type convOut struct {
		Peer        string       `json:"peer"`
		ChannelType uint8        `json:"channel_type"`
		Unread      uint64       `json:"unread"`
		Timestamp   int64        `json:"timestamp"`
		LastSeq     uint64       `json:"last_msg_seq"`
		ReadToSeq   uint64       `json:"read_to_seq"`
		Recents     []messageOut `json:"recents"`
	}
	items := make([]convOut, len(p.Items))
	for i, c := range p.Items {
		rs := make([]messageOut, len(c.Recents))
		for j, m := range c.Recents {
			rs[j] = msgOut(m)
		}
		items[i] = convOut{c.Peer, c.ChannelType, c.Unread, c.Timestamp, c.LastSeq, c.ReadToSeq, rs}
	}
	resp := map[string]any{"items": items, "has_more": p.NextCursor != ""}
	if p.NextCursor != "" {
		resp["next_cursor"] = p.NextCursor
	}
	writeJSON(w, http.StatusOK, resp)
}

// GET /v1/sync/messages?peer=&after_seq=&limit=  (oldest first; repeat with next_seq while has_more)
func (h *Handler) messages(w http.ResponseWriter, r *http.Request) {
	after, ok1 := intParam(r, "after_seq")
	limit, ok2 := intParam(r, "limit")
	if !ok1 || !ok2 {
		h.fail(w, r, domain.ErrInvalidInput)
		return
	}
	peer := r.URL.Query().Get("peer")
	p, err := h.Svc.Messages(r.Context(), h.uid(r), peer, after, int(limit))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]messageOut, len(p.Messages))
	next := after
	for i, m := range p.Messages {
		out[i] = msgOut(m)
		next = m.MessageSeq
	}
	writeJSON(w, http.StatusOK, map[string]any{"peer": peer, "messages": out, "next_seq": next, "has_more": p.HasMore})
}

// POST /v1/sync/read {peer}
func (h *Handler) read(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Peer string `json:"peer"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := h.Svc.MarkRead(r.Context(), h.uid(r), in.Peer); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUT /v1/sync/drafts {peer, content, base_version?}: empty content clears the draft. Without base_version the last writer wins; with it a stale write is 409.
func (h *Handler) putDraft(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Peer        string  `json:"peer"`
		Content     string  `json:"content"`
		BaseVersion *uint64 `json:"base_version"`
	}
	if !decode(w, r, &in) {
		return
	}
	v, err := h.Svc.PutDraft(r.Context(), h.uid(r), in.Peer, in.Content, in.BaseVersion)
	var ce *ports.ConflictError
	if errors.As(err, &ce) {
		// 409 with the current draft so the client can merge and resend with base_version = current.version
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "draft_conflict", "message": "the draft changed on another device",
			"trace_id": r.Header.Get("X-Request-Id"), "current": map[string]any{"content": ce.Current.Content, "deleted": ce.Current.Deleted, "version": ce.Current.Version}})
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]uint64{"version": v})
}

// PUT /v1/sync/conversation-settings {peer, pinned, muted}
func (h *Handler) putSetting(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Peer   string `json:"peer"`
		Pinned bool   `json:"pinned"`
		Muted  bool   `json:"muted"`
	}
	if !decode(w, r, &in) {
		return
	}
	v, err := h.Svc.PutSetting(r.Context(), h.uid(r), in.Peer, in.Pinned, in.Muted)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]uint64{"version": v})
}

// GET /v1/sync/state?since=&limit= : drafts and conversation settings changed after `since`, in version order.
func (h *Handler) state(w http.ResponseWriter, r *http.Request) {
	since, ok1 := intParam(r, "since")
	limit, ok2 := intParam(r, "limit")
	if !ok1 || !ok2 {
		h.fail(w, r, domain.ErrInvalidInput)
		return
	}
	p, err := h.Svc.State(r.Context(), h.uid(r), since, int(limit))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	type itemOut struct {
		Kind        string `json:"kind"`
		Version     uint64 `json:"version"`
		Peer        string `json:"peer"`
		ChannelType uint8  `json:"channel_type"`
		Content     string `json:"content,omitempty"`
		Deleted     *bool  `json:"deleted,omitempty"`
		Pinned      *bool  `json:"pinned,omitempty"`
		Muted       *bool  `json:"muted,omitempty"`
	}
	items := make([]itemOut, len(p.Items))
	for i, it := range p.Items {
		switch {
		case it.Draft != nil:
			d := it.Draft
			items[i] = itemOut{Kind: "draft", Version: d.Version, Peer: d.Peer, ChannelType: d.ChannelType, Content: d.Content, Deleted: &d.Deleted}
		case it.Setting != nil:
			s := it.Setting
			items[i] = itemOut{Kind: "setting", Version: s.Version, Peer: s.Peer, ChannelType: s.ChannelType, Pinned: &s.Pinned, Muted: &s.Muted}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_since": p.NextSince, "has_more": p.HasMore})
}

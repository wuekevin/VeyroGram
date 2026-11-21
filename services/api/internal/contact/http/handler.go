// Package http serves /v1/blocks. The user always comes from the verified access token.
package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"veyrogram/api/internal/contact/app"
	"veyrogram/api/internal/contact/domain"
	"veyrogram/api/internal/platform/authn"
	"veyrogram/api/internal/platform/httpx"
)

type Handler struct {
	Svc  *app.Service
	Log  *slog.Logger
	Auth func(http.Handler) http.Handler
}

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("/v1/blocks/", httpx.NotFound)
	if h.Auth == nil {
		return
	}
	mux.Handle("GET /v1/blocks", h.Auth(http.HandlerFunc(h.list)))
	mux.Handle("PUT /v1/blocks/{uid}", h.Auth(http.HandlerFunc(h.block)))
	mux.Handle("DELETE /v1/blocks/{uid}", h.Auth(http.HandlerFunc(h.unblock)))
}

func (h *Handler) uid(r *http.Request) string {
	c, _ := authn.FromContext(r.Context())
	return c.Subject
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request")
	case errors.Is(err, domain.ErrNotFound):
		httpx.WriteError(w, r, http.StatusNotFound, "not_found", "no such account")
	case errors.Is(err, domain.ErrTooMany):
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "block_list_full", "the block list is full")
	default:
		if h.Log != nil {
			h.Log.Error("block request failed", "err", err.Error(), "trace_id", r.Header.Get("X-Request-Id"))
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal", "internal error")
	}
}

// PUT /v1/blocks/{uid} -> 204 (idempotent)
func (h *Handler) block(w http.ResponseWriter, r *http.Request) {
	if err := h.Svc.Block(r.Context(), h.uid(r), r.PathValue("uid")); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /v1/blocks/{uid} -> 204 (idempotent)
func (h *Handler) unblock(w http.ResponseWriter, r *http.Request) {
	if err := h.Svc.Unblock(r.Context(), h.uid(r), r.PathValue("uid")); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /v1/blocks -> {items:[{uid, created_at}]}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	bs, err := h.Svc.List(r.Context(), h.uid(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	type item struct {
		UID       string `json:"uid"`
		CreatedAt string `json:"created_at"`
	}
	items := make([]item, len(bs))
	for i, b := range bs {
		items[i] = item{b.UID, b.CreatedAt.UTC().Format(time.RFC3339)}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}

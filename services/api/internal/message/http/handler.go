// Package http serves /v1/messages (business extension messages). Ordinary chat messages are sent by clients straight to the IM (AD-21).
package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"veyrogram/api/internal/message/app"
	"veyrogram/api/internal/message/domain"
	"veyrogram/api/internal/platform/authn"
	"veyrogram/api/internal/platform/httpx"
)

type Handler struct {
	Svc  *app.Service
	Log  *slog.Logger
	Auth func(http.Handler) http.Handler
}

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("/v1/messages/", httpx.NotFound)
	if h.Auth != nil {
		mux.Handle("POST /v1/messages", h.Auth(http.HandlerFunc(h.send)))
	}
}

type sendIn struct {
	ToUID       string          `json:"to_uid"`
	ClientMsgNo string          `json:"client_msg_no"`
	Payload     json.RawMessage `json:"payload"`
}

// POST /v1/messages {to_uid, client_msg_no (ULID), payload (JSON object)} -> 200 {message_id, message_seq, client_msg_no}.
// Retrying with the same client_msg_no returns the same message. Refusals carry a stable error code (see fail).
func (h *Handler) send(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, domain.MaxPayloadBytes+2<<10)
	var in sendIn
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || dec.More() {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			httpx.WriteError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "the message is too large")
			return
		}
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	c, _ := authn.FromContext(r.Context())
	res, err := h.Svc.Send(r.Context(), app.SendInput{From: c.Subject, To: in.ToUID, ClientMsgNo: in.ClientMsgNo, Payload: in.Payload})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"message_id": strconv.FormatInt(res.MessageID, 10), "message_seq": res.MessageSeq, "client_msg_no": in.ClientMsgNo})
}

// IMMuted is the IM's native SENDACK reason for a send-banned sender (verified against the pinned build: HTTP 200 with reason 25).
const IMMuted uint8 = 25

// Reason mapping (the contract the clients' SENDACK handling is tested against):
//
//	128 unsupported_channel 422 | 129 sender_unavailable 403 | 130 recipient_unavailable 404 | 131 invalid_payload 400 | 132 payload_too_large 413
//	200 policy_unavailable 503 (retry later, same client_msg_no) | IM native 25 muted 403 | any other IM refusal: message_rejected 403
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var rej *domain.Rejected
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_request", "to_uid or client_msg_no is invalid (client_msg_no must be a ULID)")
	case errors.As(err, &rej):
		switch {
		case rej.Native && rej.Reason == IMMuted:
			httpx.WriteError(w, r, http.StatusForbidden, "muted", "you are not allowed to send messages")
		case rej.Native:
			httpx.WriteError(w, r, http.StatusForbidden, "message_rejected", "the message was rejected")
		default:
			switch rej.Reason {
			case domain.ReasonUnsupportedChannel:
				httpx.WriteError(w, r, http.StatusUnprocessableEntity, "unsupported_channel", "this conversation type is not supported")
			case domain.ReasonSenderUnavailable:
				httpx.WriteError(w, r, http.StatusForbidden, "sender_unavailable", "your account cannot send messages")
			case domain.ReasonRecipientUnavailable:
				httpx.WriteError(w, r, http.StatusNotFound, "recipient_unavailable", "the recipient cannot receive messages")
			case domain.ReasonInvalidPayload:
				httpx.WriteError(w, r, http.StatusBadRequest, "invalid_payload", "the message payload is invalid")
			case domain.ReasonPayloadTooLarge:
				httpx.WriteError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "the message is too large")
			default:
				httpx.WriteError(w, r, http.StatusServiceUnavailable, "policy_unavailable", "the message could not be checked, try again later")
			}
		}
	case errors.Is(err, domain.ErrIMUnavailable):
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "im_unavailable", "the messaging service is unavailable, try again later")
	default:
		if h.Log != nil {
			h.Log.Error("message send failed", "err", err.Error(), "trace_id", r.Header.Get("X-Request-Id"))
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal", "internal error")
	}
}

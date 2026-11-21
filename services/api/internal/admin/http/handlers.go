package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"veyrogram/api/internal/admin/domain"
	"veyrogram/api/internal/admin/ports"
	"veyrogram/api/internal/platform/httpx"
)

// Mount registers every admin route with its policy. Adding a route means choosing its policy here; there is no other way to add one.
func (r *Router) Mount(log *slog.Logger) {
	h := &handlers{r: r, log: log}
	pub := Policy{Public: true}
	r.Handle("POST", "/v1/manager/login", pub, h.login)
	r.Handle("POST", "/v1/manager/mfa/verify", pub, h.mfaVerify)
	r.Handle("POST", "/v1/manager/mfa/enroll", pub, h.mfaEnroll)
	r.Handle("POST", "/v1/manager/logout", Policy{Permission: domain.PermSelf}, h.logout)
	r.Handle("GET", "/v1/manager/me", Policy{Permission: domain.PermSelf}, h.me)
	r.Handle("GET", "/v1/manager/admins", Policy{Permission: domain.PermAdminRead}, h.listAdmins)
	r.Handle("POST", "/v1/manager/admins", Policy{Permission: domain.PermAdminWrite}, h.createAdmin)
	r.Handle("POST", "/v1/manager/admins/{id}/disable", Policy{Permission: domain.PermAdminWrite}, h.disableAdmin)
	r.Handle("GET", "/v1/manager/audit", Policy{Permission: domain.PermAuditRead}, h.audit)
	r.Handle("GET", "/v1/manager/audit/verify", Policy{Permission: domain.PermAuditVerify}, h.auditVerify)
}

type handlers struct {
	r   *Router
	log *slog.Logger
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decode(w http.ResponseWriter, req *http.Request, v any) bool {
	req.Body = http.MaxBytesReader(w, req.Body, 8<<10)
	dec := json.NewDecoder(req.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil || dec.More() {
		httpx.WriteError(w, req, http.StatusBadRequest, "invalid_request", "malformed request body")
		return false
	}
	return true
}

func (h *handlers) fail(w http.ResponseWriter, req *http.Request, err error) {
	var weak *domain.WeakPasswordError
	switch {
	case errors.As(err, &weak):
		httpx.WriteErrorDetails(w, req, http.StatusUnprocessableEntity, "weak_password", "the password does not meet the policy", weak.Problems)
	case errors.Is(err, domain.ErrInvalidCredentials):
		httpx.WriteError(w, req, http.StatusUnauthorized, "invalid_credentials", "username or password is wrong")
	case errors.Is(err, domain.ErrInvalidChallenge):
		httpx.WriteError(w, req, http.StatusUnauthorized, "invalid_challenge", "start the sign-in again")
	case errors.Is(err, domain.ErrInvalidCode):
		httpx.WriteError(w, req, http.StatusUnauthorized, "invalid_code", "the code is wrong; start the sign-in again")
	case errors.Is(err, domain.ErrLocked):
		w.Header().Set("Retry-After", "900")
		httpx.WriteError(w, req, http.StatusTooManyRequests, "locked", "too many failed attempts, try again later")
	case errors.Is(err, domain.ErrReasonRequired):
		httpx.WriteError(w, req, http.StatusUnprocessableEntity, "reason_required", "state the reason for this action (at least 8 characters)")
	case errors.Is(err, domain.ErrInvalidInput):
		httpx.WriteError(w, req, http.StatusBadRequest, "invalid_request", "invalid request")
	case errors.Is(err, domain.ErrNotFound):
		httpx.WriteError(w, req, http.StatusNotFound, "not_found", "no such admin")
	case errors.Is(err, domain.ErrConflict):
		httpx.WriteError(w, req, http.StatusConflict, "already_exists", "that username is taken")
	case errors.Is(err, domain.ErrLastOwner):
		httpx.WriteError(w, req, http.StatusConflict, "last_owner", "the last active owner cannot be disabled")
	default:
		if h.log != nil {
			h.log.Error("admin request failed", "err", err.Error(), "trace_id", req.Header.Get("X-Request-Id"))
		}
		httpx.WriteError(w, req, http.StatusInternalServerError, "internal", "internal error")
	}
}

// POST /v1/manager/login {username, password} -> {state: "mfa_required"|"enrollment_required", token}
func (h *handlers) login(w http.ResponseWriter, req *http.Request, _ ports.Principal) {
	var in struct{ Username, Password string }
	if !decode(w, req, &in) {
		return
	}
	res, err := h.r.Svc.Login(req.Context(), in.Username, in.Password, h.r.ip(req))
	if err != nil {
		h.fail(w, req, err)
		return
	}
	state := "mfa_required"
	if res.Kind == "enroll" {
		state = "enrollment_required"
	}
	writeJSON(w, http.StatusOK, map[string]string{"state": state, "token": res.Token})
}

func (h *handlers) complete(kind string) handler {
	return func(w http.ResponseWriter, req *http.Request, _ ports.Principal) {
		var in struct{ Token, Code string }
		if !decode(w, req, &in) {
			return
		}
		s, err := h.r.Svc.Complete(req.Context(), kind, in.Token, in.Code, h.r.ip(req))
		if err != nil {
			h.fail(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session_token": s.Token, "expires_at": s.ExpiresAt.UTC().Format(time.RFC3339), "username": s.Username, "role": s.Role})
	}
}

func (h *handlers) mfaVerify(w http.ResponseWriter, req *http.Request, p ports.Principal) {
	h.complete("mfa")(w, req, p)
}
func (h *handlers) mfaEnroll(w http.ResponseWriter, req *http.Request, p ports.Principal) {
	h.complete("enroll")(w, req, p)
}

func (h *handlers) logout(w http.ResponseWriter, req *http.Request, p ports.Principal) {
	if err := h.r.Svc.Logout(req.Context(), p); err != nil {
		h.fail(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) me(w http.ResponseWriter, req *http.Request, p ports.Principal) {
	var perms []string
	for k := range p.Perms {
		perms = append(perms, k)
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": p.AdminID, "username": p.Username, "role": domain.RoleNames[p.RoleID], "permissions": sortedStrings(perms)})
}

func (h *handlers) listAdmins(w http.ResponseWriter, req *http.Request, _ ports.Principal) {
	as, err := h.r.Svc.ListAdmins(req.Context())
	if err != nil {
		h.fail(w, req, err)
		return
	}
	type item struct {
		ID       uint64 `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
		Active   bool   `json:"active"`
		Enrolled bool   `json:"mfa_enrolled"`
	}
	out := make([]item, len(as))
	for i, a := range as {
		out[i] = item{a.ID, a.Username, a.Role, a.Active, a.Enrolled}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// POST /v1/manager/admins {username, role, password, reason} -> 201 {id, totp_secret, totp_uri}. The secret is shown once, for hand-over.
func (h *handlers) createAdmin(w http.ResponseWriter, req *http.Request, p ports.Principal) {
	var in struct{ Username, Role, Password, Reason string }
	if !decode(w, req, &in) {
		return
	}
	id, prov, err := h.r.Svc.CreateAdmin(req.Context(), p, in.Username, in.Role, in.Password, in.Reason)
	if err != nil {
		h.fail(w, req, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "totp_secret": prov.Secret, "totp_uri": prov.URI})
}

func (h *handlers) disableAdmin(w http.ResponseWriter, req *http.Request, p ports.Principal) {
	id, err := strconv.ParseUint(req.PathValue("id"), 10, 64)
	if err != nil {
		h.fail(w, req, domain.ErrInvalidInput)
		return
	}
	var in struct{ Reason string }
	if !decode(w, req, &in) {
		return
	}
	if err := h.r.Svc.DisableAdmin(req.Context(), p, id, in.Reason); err != nil {
		h.fail(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /v1/manager/audit?actor=&action=&before_id=&limit= -> newest first; next_before_id continues the listing. Read-only.
func (h *handlers) audit(w http.ResponseWriter, req *http.Request, _ ports.Principal) {
	q := req.URL.Query()
	before, err1 := strconv.ParseUint(orZero(q.Get("before_id")), 10, 63)
	limit, err2 := strconv.Atoi(orZero(q.Get("limit")))
	if err1 != nil || err2 != nil || limit < 0 {
		h.fail(w, req, domain.ErrInvalidInput)
		return
	}
	rows, err := h.r.Svc.AuditRows(req.Context(), ports.AuditFilter{Actor: q.Get("actor"), Action: q.Get("action"), BeforeID: before, Limit: limit})
	if err != nil {
		h.fail(w, req, err)
		return
	}
	type item struct {
		ID        uint64 `json:"id"`
		Actor     string `json:"actor"`
		Action    string `json:"action"`
		Target    string `json:"target"`
		Detail    string `json:"detail,omitempty"`
		TraceID   string `json:"trace_id"`
		CreatedAt string `json:"created_at"`
	}
	out := make([]item, len(rows))
	for i, r := range rows {
		out[i] = item{r.ID, r.Actor, r.Action, r.Target, r.Detail, r.TraceID, r.CreatedAt.UTC().Format(time.RFC3339Nano)}
	}
	resp := map[string]any{"items": out}
	if len(rows) > 0 {
		resp["next_before_id"] = rows[len(rows)-1].ID
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *handlers) auditVerify(w http.ResponseWriter, req *http.Request, _ ports.Principal) {
	head, err := h.r.Svc.VerifyAudit(req.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "the audit chain does not verify"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "rows": head.Rows, "head_id": head.ID, "head_hash": head.Hash})
}

func orZero(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

func sortedStrings(s []string) []string {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
	return s
}

package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	identityadapters "veyrogram/api/internal/identity/adapters"
	"veyrogram/api/internal/message/adapters"
	"veyrogram/api/internal/message/app"
	"veyrogram/api/internal/message/domain"
	messagehttp "veyrogram/api/internal/message/http"
	"veyrogram/api/internal/message/ports"
	"veyrogram/api/internal/platform/imgw"
	"veyrogram/api/internal/platform/imwebhook"
	"veyrogram/api/internal/platform/metrics"
)

// accountDirectory answers the message module's Accounts port from the identity module's query.
type accountDirectory struct {
	ids    *identityadapters.MySQLStore
	tenant uint64
}

func (a accountDirectory) Active(ctx context.Context, uids []string) (map[string]bool, error) {
	return a.ids.ActiveUIDs(ctx, a.tenant, uids)
}

// imSender adapts the IM gateway to the message module's Sender port, classifying failures: an IM refusal is a *domain.Rejected, anything else
// (unreachable, 5xx, timeouts) is domain.ErrIMUnavailable.
type imSender struct{ im *imgw.Client }

func (s imSender) Send(ctx context.Context, from, channelID string, channelType uint8, clientMsgNo string, payload []byte) (ports.Ack, error) {
	r, err := s.im.Send(ctx, from, channelID, channelType, clientMsgNo, payload)
	var rej *imgw.SendRejected
	var api *imgw.APIError
	switch {
	case err == nil:
		return ports.Ack{MessageID: r.MessageID, MessageSeq: r.MessageSeq}, nil
	case errors.As(err, &rej):
		return ports.Ack{}, &domain.Rejected{Reason: rej.Reason, Native: true}
	case errors.As(err, &api) && api.Status >= 400 && api.Status < 500:
		return ports.Ack{}, &domain.Rejected{Reason: 0, Native: true}
	default:
		return ports.Ack{}, domain.ErrIMUnavailable
	}
}

// messageGate builds the send-policy gate shared by the IM before_send hook and /v1/messages.
func messageGate(pool *sql.DB, pm *metrics.Platform, log *slog.Logger) *app.Gate {
	acc := accountDirectory{ids: &identityadapters.MySQLStore{DB: pool}, tenant: 1}
	return &app.Gate{Rules: app.StandardRules(acc), Budget: app.DefaultBudget, Observe: func(rule, result string) {
		pm.MessageGate.Inc(rule, result)
		if result == "error" || result == "timeout" {
			log.Warn("send gate could not decide: denying (fail-closed)", "rule", rule, "result", result)
		}
	}}
}

// beforeSendHook turns the gate into the IM's synchronous before_send decision.
func beforeSendHook(g *app.Gate) func(context.Context, imwebhook.BeforeSend) (imwebhook.Decision, error) {
	return func(ctx context.Context, in imwebhook.BeforeSend) (imwebhook.Decision, error) {
		v := g.Evaluate(ctx, domain.Send{From: in.FromUID, ChannelID: in.ChannelID, ChannelType: in.ChannelType, ClientMsgNo: in.ClientMsgNo, Payload: in.Payload})
		return imwebhook.Decision{Allow: v.Allow, ReasonCode: v.Reason}, nil
	}
}

func messageMux(pool *sql.DB, log *slog.Logger, im *imgw.Client, gate *app.Gate, auth func(http.Handler) http.Handler) *http.ServeMux {
	svc := &app.Service{Gate: gate, IM: imSender{im}, Refs: &adapters.MySQLStore{DB: pool}, Tenant: 1}
	mux := http.NewServeMux()
	(&messagehttp.Handler{Svc: svc, Log: log, Auth: auth}).Mount(mux)
	return mux
}

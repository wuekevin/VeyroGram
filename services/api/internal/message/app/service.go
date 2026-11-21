package app

import (
	"context"
	"errors"
	"time"

	"veyrogram/api/internal/message/domain"
	"veyrogram/api/internal/message/ports"
)

// Service is the use-case layer behind POST /v1/messages (business extension messages; ordinary chat goes client -> IM directly, AD-21).
type Service struct {
	Gate   *Gate
	IM     ports.Sender
	Refs   ports.Refs
	Tenant uint64
	Now    func() time.Time
}

type SendInput struct {
	From        string
	To          string
	ClientMsgNo string
	Payload     []byte
}

type Result struct {
	MessageID  int64
	MessageSeq uint64
}

// Send runs the same gate the IM hook runs (so the caller gets the precise reason immediately), hands the message to the IM and records the
// business reference. Retrying with the same ClientMsgNo yields the same message: the IM de-duplicates on (sender, client_msg_no, channel), and
// the reference insert is idempotent on the same key.
func (s *Service) Send(ctx context.Context, in SendInput) (Result, error) {
	if !domain.ValidUID(in.From) || !domain.ValidUID(in.To) || !domain.ValidULID(in.ClientMsgNo) {
		return Result{}, domain.ErrInvalidInput
	}
	m := domain.Send{From: in.From, ChannelID: in.To, ChannelType: domain.ChannelPerson, ClientMsgNo: in.ClientMsgNo, Payload: in.Payload}
	if v := s.Gate.Evaluate(ctx, m); !v.Allow {
		return Result{}, &domain.Rejected{Reason: v.Reason}
	}
	ack, err := s.IM.Send(ctx, in.From, in.To, domain.ChannelPerson, in.ClientMsgNo, in.Payload)
	if err != nil {
		return Result{}, err // *domain.Rejected (IM-native refusal) or domain.ErrIMUnavailable, as classified by the adapter
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	if err := s.Refs.Record(ctx, ports.Ref{Tenant: s.Tenant, MessageID: ack.MessageID, MessageSeq: ack.MessageSeq, From: in.From, ChannelID: domain.PersonChannel(in.From, in.To),
		ChannelType: domain.ChannelPerson, ClientMsgNo: in.ClientMsgNo, At: now}); err != nil {
		// The message is already in the IM. Failing here would make clients retry (harmless: same client_msg_no) but would hide an accepted message
		// behind an error, so surface it as an internal error only; the unique key makes the retry repair the reference.
		return Result{}, errors.Join(errors.New("message: reference not recorded"), err)
	}
	return Result{MessageID: ack.MessageID, MessageSeq: ack.MessageSeq}, nil
}

// RecordNotified stores the business reference of messages the IM has committed, whichever path they took (client SDK straight to the IM, or the API).
// It is idempotent on (sender, conversation, client_msg_no), so the IM's retries and the API path's own insert never duplicate a row. System notices and
// anything that is not a one-to-one message are not business messages and are skipped.
func (s *Service) RecordNotified(ctx context.Context, batch []Notified) error {
	for _, m := range batch {
		if m.ChannelType != domain.ChannelPerson || m.FromUID == domain.SystemUID || m.MessageID == 0 {
			continue
		}
		ch, ok := domain.NotifiedPerson(m.FromUID, m.ChannelID)
		if !ok {
			continue
		}
		if err := s.Refs.Record(ctx, ports.Ref{Tenant: s.Tenant, MessageID: m.MessageID, MessageSeq: m.MessageSeq, From: m.FromUID, ChannelID: ch,
			ChannelType: m.ChannelType, ClientMsgNo: m.ClientMsgNo, At: time.Unix(m.Timestamp, 0)}); err != nil {
			return err
		}
	}
	return nil
}

// Notified is one committed message as reported by the IM.
type Notified struct {
	MessageID   int64
	MessageSeq  uint64
	ClientMsgNo string
	FromUID     string
	ChannelID   string
	ChannelType uint8
	Timestamp   int64
}

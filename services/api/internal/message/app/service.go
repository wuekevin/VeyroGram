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
	if err := s.Refs.Record(ctx, ports.Ref{Tenant: s.Tenant, MessageID: ack.MessageID, MessageSeq: ack.MessageSeq, From: in.From, ChannelID: in.To,
		ChannelType: domain.ChannelPerson, ClientMsgNo: in.ClientMsgNo, At: now}); err != nil {
		// The message is already in the IM. Failing here would make clients retry (harmless: same client_msg_no) but would hide an accepted message
		// behind an error, so surface it as an internal error only; the unique key makes the retry repair the reference.
		return Result{}, errors.Join(errors.New("message: reference not recorded"), err)
	}
	return Result{MessageID: ack.MessageID, MessageSeq: ack.MessageSeq}, nil
}

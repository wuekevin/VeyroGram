// Package ports defines what the message module needs from the outside world.
package ports

import (
	"context"
	"time"
)

// Accounts answers whether accounts may take part in a conversation. It is implemented by the composition root on top of the identity module's
// ports; this module never reads identity tables (AD-1).
type Accounts interface {
	// Active returns, for each given uid, whether the account exists and is enabled. Unknown uids are simply absent or false.
	Active(ctx context.Context, uids []string) (map[string]bool, error)
}

// Ack is the IM's acceptance of a message.
type Ack struct {
	MessageID  int64
	MessageSeq uint64
}

// Sender hands a message to the IM. A refusal by the IM (HTTP 200 with a failure reason) is returned as *domain.Rejected with Native set.
type Sender interface {
	Send(ctx context.Context, from, channelID string, channelType uint8, clientMsgNo string, payload []byte) (Ack, error)
}

type Ref struct {
	Tenant      uint64
	MessageID   int64
	MessageSeq  uint64
	From        string
	ChannelID   string
	ChannelType uint8
	ClientMsgNo string
	At          time.Time
}

// Refs records the business reference of an accepted message. Recording the same (sender, channel, client_msg_no) again is a no-op.
type Refs interface {
	Record(ctx context.Context, r Ref) error
}

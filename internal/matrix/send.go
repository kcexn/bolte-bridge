package matrix

import (
	"context"
	"fmt"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func (c *matrixClient) Send(ctx context.Context, msg OutboundEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	roomID := id.RoomID(msg.RoomID)
	intent := c.as.Intent(id.UserID(msg.Sender))

	// Impersonating a ghost requires it to exist and be a room member.
	if err := intent.EnsureRegistered(ctx); err != nil {
		return fmt.Errorf("matrix: ensure %s registered: %w", msg.Sender, err)
	}
	if err := intent.EnsureJoined(ctx, roomID); err != nil {
		return fmt.Errorf("matrix: ensure %s joined %s: %w", msg.Sender, msg.RoomID, err)
	}

	content := event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    msg.Body,
	}
	content.RelatesTo = relatesTo(msg)

	if _, err := intent.SendMessageEvent(ctx, roomID, event.EventMessage, &content); err != nil {
		return fmt.Errorf("matrix: send to %s: %w", msg.RoomID, err)
	}
	return nil
}

// relatesTo builds the m.relates_to for an outbound message, or nil when the
// message is neither threaded nor a reply. A thread root takes precedence: a
// reply within a thread is expressed as the thread relation with the reply as
// its fallback, matching Matrix's threading model.
func relatesTo(msg OutboundEvent) *event.RelatesTo {
	switch {
	case msg.ThreadRoot != "":
		return (&event.RelatesTo{}).SetThread(id.EventID(msg.ThreadRoot), id.EventID(msg.ReplyTo))
	case msg.ReplyTo != "":
		return (&event.RelatesTo{}).SetReplyTo(id.EventID(msg.ReplyTo))
	default:
		return nil
	}
}

package matrix

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func (c *matrixClient) Fetch(ctx context.Context, since string) ([]RawEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	filter, err := c.makeRoomFilter()
	if err != nil {
		return nil, err
	}

	resp, err := c.as.BotClient().FullSyncRequest(ctx, mautrix.ReqSync{
		FilterID: filter,
	})
	if err != nil {
		return nil, fmt.Errorf("matrix: sync: %w", err)
	}

	joined := resp.Rooms.Join[id.RoomID(c.cfg.RoomID)]
	if joined == nil {
		return nil, fmt.Errorf("matrix: sync: not joined to room %q", c.cfg.RoomID)
	}

	return c.timelineSince(ctx, resp.NextBatch, since)
}

// timelineLimit is used to limit how many events are returned in each paginated
// request.
const timelineLimit = 10

// toRawEvent projects a decoded m.room.message event into a RawEvent.
// Returns false if the event has been redacted or is otherwise empty.
func toRawEvent(evt *event.Event, roomID string) (RawEvent, bool) {
	if err := evt.Content.ParseRaw(event.EventMessage); err != nil {
		return RawEvent{}, false
	}
	msg := evt.Content.AsMessage()
	if msg == nil || msg.Body == "" {
		return RawEvent{}, false
	}

	re := RawEvent{
		EventID:   evt.ID.String(),
		Sender:    evt.Sender.String(),
		RoomID:    roomID,
		Body:      msg.Body,
		MsgType:   string(msg.MsgType),
		Timestamp: time.UnixMilli(evt.Timestamp),
	}
	if rel := msg.RelatesTo; rel != nil {
		re.ThreadRoot = rel.GetThreadParent().String()
		re.ReplyTo = rel.GetReplyTo().String()
	}
	return re, true
}

// makeRoomFilter serialises a filter restricting a sync to the configured room's
// m.room.message timeline events, keeping the /sync response small. The JSON is
// passed inline as the request's filter parameter.
func (c *matrixClient) makeRoomFilter() (string, error) {
	roomID := id.RoomID(c.cfg.RoomID)
	filter := mautrix.Filter{
		Room: &mautrix.RoomFilter{
			Rooms: []id.RoomID{roomID},
			Timeline: &mautrix.FilterPart{
				Rooms: []id.RoomID{roomID},
			},
		},
	}
	raw, err := json.Marshal(&filter)
	if err != nil {
		return "", fmt.Errorf("matrix: marshal sync filter: %w", err)
	}
	return string(raw), nil
}

// timelineSince paginates the room timeline backward from the given batch
// token, collecting m.room.message events until it reaches the "since" event.
// Events are returned in oldest->newest order.
func (c *matrixClient) timelineSince(ctx context.Context, from, since string) ([]RawEvent, error) {
	events := make([]RawEvent, 0, timelineLimit)

	roomID := id.RoomID(c.cfg.RoomID)
	sinceID := id.EventID(since)
	done := false

	// Efficiently append events in reverse timeline order.
	for !done {
		messages, err := c.as.BotClient().
			Messages(ctx, roomID, from, "", mautrix.DirectionBackward, nil, timelineLimit)
		if err != nil {
			return nil, fmt.Errorf("matrix: messages: %w", err)
		}

		done = len(messages.Chunk) == 0
		for _, event := range messages.Chunk {
			if event.ID == sinceID {
				done = true
				break
			}
			if rawEvent, ok := toRawEvent(event, c.cfg.RoomID); ok {
				events = append(events, rawEvent)
			}
		}

		from = messages.End
	}

	// Reverse events so that the timeline is oldest->newest.
	slices.Reverse(events)

	return events, nil
}

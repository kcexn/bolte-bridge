package matrix

import (
	"context"
	"database/sql"
	"errors"

	"bolte-bridge/internal/relay"
	"bolte-bridge/internal/store"
)

func (a *Adapter) fetch(ctx context.Context) ([]relay.Message, error) {
	eventID, err := a.getCursor(ctx)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		lastID, err := a.client.LastEvent(ctx)
		if err != nil {
			return nil, err
		}
		return nil, a.setCursor(ctx, lastID)
	case err != nil:
		return nil, err
	default:
		return a.fetchMessages(ctx, eventID)
	}
}

// getCursor retrieves the current EventID from the store.
func (a *Adapter) getCursor(ctx context.Context) (string, error) {
	var eventID string
	err := store.Client().WithTx(ctx, func(ctx context.Context, tx store.Tx) error {
		e, err := tx.Matrix().Cursor(ctx, a.cfg.ServerName, a.cfg.RoomID)
		if err != nil {
			return err
		}
		eventID = e
		return nil
	})
	return eventID, err
}

// setCursor retrieves the current EventID from the store.
func (a *Adapter) setCursor(ctx context.Context, eventID string) error {
	return store.Client().WithTx(ctx, func(ctx context.Context, tx store.Tx) error {
		return tx.Matrix().SetCursor(ctx, a.cfg.ServerName, a.cfg.RoomID, eventID)
	})
}

func (a *Adapter) fetchMessages(ctx context.Context, eventID string) ([]relay.Message, error) {
	rawEvents, err := a.client.Fetch(ctx, eventID)
	if err != nil {
		return nil, err
	}

	return rawEventsToRelayMessages(rawEvents), nil
}

func rawEventsToRelayMessages(
	rawEvents []RawEvent,
) []relay.Message {
	messages := make([]relay.Message, len(rawEvents))

	for i, raw := range rawEvents {
		messages[i] = rawEventToRelayMessage(raw)
	}
	return messages
}

func rawEventToRelayMessage(raw RawEvent) relay.Message {
	return relay.Message{
		Sender: relay.Identity{Address: relay.Address{
			Mode: relay.MediumMatrix,
			ID:   raw.Sender,
		}},
		MessageID: raw.EventID,
		ThreadID:  raw.ReplyTo,
		Body:      raw.Body,
	}
}

package core

import (
	"context"
	"fmt"

	"bolte-bridge/internal/relay"
)

// Bridge is the top-level application object for one one-shot run. It owns one
// Adapter and one RelayEngine per medium and drives a single relay cycle in
// each direction.
type Bridge struct {
	emailAdapter  Adapter      // email medium edge (IMAP/SMTP)
	matrixAdapter Adapter      // matrix medium edge (appservice)
	emailEngine   *RelayEngine // room -> list: Publish -> matrixAdapter.Send
	matrixEngine  *RelayEngine // list -> room: Publish -> emailAdapter.Send
}

// NewBridge builds a Bridge around the two injected adapters.
func NewBridge(emailAdapter Adapter, matrixAdapter Adapter) *Bridge {
	return &Bridge{
		emailAdapter:  emailAdapter,
		matrixAdapter: matrixAdapter,
		emailEngine: &RelayEngine{
			Publish: func(ctx context.Context, msg relay.RoutedMessage) error {
				_, err := matrixAdapter.Send(ctx, msg)
				// TODO: Implement the email <-> matrix mapping.
				return err
			},
		},
		matrixEngine: &RelayEngine{
			Publish: func(ctx context.Context, msg relay.RoutedMessage) error {
				_, err := emailAdapter.Send(ctx, msg)
				// TODO: Implement the email <-> matrix mapping.
				return err
			},
		},
	}
}

// Run performs one full relay cycle.
func (b *Bridge) Run(ctx context.Context) error {
	if err := relayOnce(ctx, b.matrixAdapter, b.matrixEngine); err != nil {
		return fmt.Errorf("relay room->list: %w", err)
	}
	if err := relayOnce(ctx, b.emailAdapter, b.emailEngine); err != nil {
		return fmt.Errorf("relay list->room: %w", err)
	}
	return nil
}

// relayOnce drives one direction. It fetches everything pending from src, runs
// it through engine, then commits src's read cursor.
func relayOnce(ctx context.Context, src Adapter, engine *RelayEngine) error {
	msgs, err := src.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	if _, err := engine.Process(ctx, msgs); err != nil {
		return fmt.Errorf("process: %w", err)
	}
	// TODO: Get the message ID of the last published message in
	// the msgs slice from the count returned by Process. Then
	// commit it.
	if err := src.Commit(ctx, ""); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

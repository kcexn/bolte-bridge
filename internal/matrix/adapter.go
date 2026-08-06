package matrix

import (
	"context"

	"bolte-bridge/internal/core"
	"bolte-bridge/internal/relay"
	"bolte-bridge/internal/store"
)

// Adapter is the Matrix medium edge of the bridge. It owns the Client it talks
// to outright: the transport is an implementation detail of this package, so
// callers configure the adapter and never name the Client themselves.
//
// Its Fetch, Send, and Commit methods are not implemented yet.
type Adapter struct {
	client Client
}

// NewAdapter builds an Adapter and the Client it owns from ctx and cfg,
// reporting any configuration or initialization error from Client construction.
func NewAdapter(ctx context.Context, cfg Config) (*Adapter, error) {
	client, err := NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return &Adapter{
		client: client,
	}, nil
}

// Medium reports the Matrix medium.
func (a *Adapter) Medium() relay.Medium {
	return relay.MediumMatrix
}

// Fetch will fetch Matrix events past the committed sync token and translate
// them into relay messages, advancing only the in-memory cursor.
func (a *Adapter) Fetch(_ context.Context) ([]relay.Message, error) {
	return nil, nil
}

// Send will translate a routed message into a Matrix event, send it to the
// configured room, and return the event ID assigned by the homeserver.
func (a *Adapter) Send(_ context.Context, _ relay.RoutedMessage) (string, error) {
	return "", nil
}

// Commit will durably advance the Matrix sync token to cursor. An empty cursor
// commits everything returned by the preceding Fetch.
func (a *Adapter) Commit(ctx context.Context, cursor string) error {
	// If the cursor is empty, we have nothing to save.
	if cursor == "" {
		return nil
	}

	// Grab the global database client and open a transaction bubble
	return store.Client().WithTx(ctx, func(txCtx context.Context, tx store.Tx) error {
		return tx.SetMatrixCursor(txCtx, cursor)
	})
}

// Compile-time assertion that Adapter satisfies core.Adapter.
var _ core.Adapter = (*Adapter)(nil)

// Close closes the underlying Matrix client.
func (a *Adapter) Close(ctx context.Context) error {
	return a.client.Close(ctx)
}

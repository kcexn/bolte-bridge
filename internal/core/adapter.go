// Package core defines and implements the core relay component of the bridge.
// The core relay communicates with connected messaging adapters via the
// core.Adapter interface.
package core

import (
	"context"

	"bolte-bridge/internal/relay"
)

// Adapter is the medium-specific edge of the bridge.
type Adapter interface {
	// Medium reports which medium this adapter serves.
	Medium() relay.Medium

	// Fetch returns every message that has arrived since the last committed
	// cursor, oldest first. It reads from the medium (IMAP fetch, Matrix
	// `GET /messages`) and advances the adapter's in-memory cursor, but does NOT
	// durably persist that cursor (see Commit).
	Fetch(ctx context.Context) ([]relay.Message, error)

	// Send delivers one routed message into this adapter's medium and returns
	// the unique identifier the medium assigned it: the Message-ID for email,
	// the event ID for Matrix. The returned identifier is meaningful only
	// when err is nil.
	Send(ctx context.Context, msg relay.RoutedMessage) (string, error)

	// The cursor is an opaque, medium-specific position (an IMAP UID, a Matrix
	// sync token). Commit durably advances this adapter's read cursor,
	// committing every message up to and including cursor. An empty cursor
	// commits everything up to the last message returned by the preceding Fetch.
	Commit(ctx context.Context, cursor string) error
}

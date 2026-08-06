package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func TestMatrixCursor(t *testing.T) {
	ctx, s := openTestStore(t)

	serverName := "matrix.org"
	roomID := "!aaa-room:matrix.org"
	wantEventID := "!aaa-event:matrix.org"

	// Manually set the cursor.
	if err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		_, err := tx.Email.Tx.ExecContext(
			ctx,
			"INSERT INTO matrix_cursors (server_name, room_id, last_seen_event) VALUES (?, ?, ?)",
			serverName,
			roomID,
			wantEventID,
		)
		return err
	}); err != nil {
		t.Fatalf("insert test data: %v", err)
	}

	// Retrieve and verify the cursor.
	var gotEventID string
	if err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		eventID, err := tx.Matrix.Cursor(ctx, serverName, roomID)
		if err != nil {
			return err
		}
		gotEventID = eventID
		return nil
	}); err != nil {
		t.Fatalf("Cursor: %v", err)
	}
	if gotEventID != wantEventID {
		t.Errorf("EventID = %q, want %q", gotEventID, wantEventID)
	}
}

func TestMatrixCursorNotFound(t *testing.T) {
	ctx, s := openTestStore(t)

	serverName := "matrix.org"
	roomID := "!aaa-room:matrix.org"

	// Try to retrieve a cursor that was never inserted.
	err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		_, err := tx.Matrix.Cursor(ctx, serverName, roomID)
		return err
	})

	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Cursor error = %v, want sql.ErrNoRows", err)
	}
}

func TestMatrixSetCursorInsert(t *testing.T) {
	ctx, s := openTestStore(t)

	serverName := "matrix.org"
	roomID := "!aaa-room:matrix.org"
	wantEventID := "!aaa-event:matrix.org"

	// Set a cursor that doesn't exist yet.
	if err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		return tx.Matrix.SetCursor(ctx, serverName, roomID, wantEventID)
	}); err != nil {
		t.Fatalf("SetCursor: %v", err)
	}

	// Verify it was inserted.
	var gotEventID string
	if err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		eventID, err := tx.Matrix.Cursor(ctx, serverName, roomID)
		if err != nil {
			return err
		}
		gotEventID = eventID
		return nil
	}); err != nil {
		t.Fatalf("Cursor: %v", err)
	}
	if gotEventID != wantEventID {
		t.Errorf("UID = %q, want %q", gotEventID, wantEventID)
	}
}

func TestMatrixSetCursorUpdate(t *testing.T) {
	ctx, s := openTestStore(t)

	serverName := "matrix.org"
	roomID := "!aaa-room:matrix.org"
	oldEventID := "!aaa-event:matrix.org"
	newEventID := "!bbb-event:matrix.org"

	// Insert initial cursor.
	if err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		return tx.Matrix.SetCursor(ctx, serverName, roomID, oldEventID)
	}); err != nil {
		t.Fatalf("SetCursor (initial): %v", err)
	}

	// Update the cursor.
	if err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		return tx.Matrix.SetCursor(ctx, serverName, roomID, newEventID)
	}); err != nil {
		t.Fatalf("SetCursor (update): %v", err)
	}

	// Verify the values were updated.
	var gotEventID string
	if err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		eventID, err := tx.Matrix.Cursor(ctx, serverName, roomID)
		if err != nil {
			return err
		}
		gotEventID = eventID
		return nil
	}); err != nil {
		t.Fatalf("Cursor: %v", err)
	}
	if gotEventID != newEventID {
		t.Errorf("EventID = %q, want %q", gotEventID, newEventID)
	}
}

func TestMatrixSetCursorFailure(t *testing.T) {
	ctx, s := openTestStore(t)

	serverName := "matrix.org"
	roomID := "!aaa-room:matrix.org"
	eventID := "!aaa-event:matrix.org"

	// Create a cancelled context to cause ExecContext to fail.
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	// Attempt to set a cursor with the cancelled context.
	err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		// Use the cancelled context for SetCursor instead of the transaction context.
		return tx.Matrix.SetCursor(cancelledCtx, serverName, roomID, eventID)
	})

	if err == nil {
		t.Fatal("SetCursor with cancelled context returned nil error, want failure")
	}
	if !strings.Contains(err.Error(), "set matrix cursor") {
		t.Errorf(
			"SetCursor error message = %q, want to contain %q",
			err.Error(),
			"set matrix cursor",
		)
	}
}

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// openTestStore opens a fresh Store on a temporary database for testing.
func openTestStore(t *testing.T) (context.Context, *Store) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bolte.db")

	s, err := Open(ctx, Config{Path: path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close(ctx) })
	return ctx, s
}

// TestCursorRetrievesIMAPCursor checks that Cursor retrieves the persisted
// IMAP cursor (uid and uidValidity) for an account and mailbox.
func TestCursor(t *testing.T) {
	ctx, s := openTestStore(t)

	username := "test@example.com"
	mailbox := "INBOX"
	wantUID := uint32(42)
	wantValidity := uint32(100)

	// Insert test data into imap_cursors table.
	if err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		_, err := tx.Tx.ExecContext(
			ctx,
			"INSERT INTO imap_cursors (account_id, mailbox_name, last_seen_uid, uid_validity) VALUES (?, ?, ?, ?)",
			username,
			mailbox,
			wantUID,
			wantValidity,
		)
		return err
	}); err != nil {
		t.Fatalf("insert test data: %v", err)
	}

	// Retrieve and verify the cursor.
	var gotUID, gotValidity uint32
	if err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		var e error
		gotUID, gotValidity, e = tx.Cursor(ctx, username, mailbox)
		return e
	}); err != nil {
		t.Fatalf("Cursor: %v", err)
	}
	if gotUID != wantUID {
		t.Errorf("UID = %d, want %d", gotUID, wantUID)
	}
	if gotValidity != wantValidity {
		t.Errorf("Validity = %d, want %d", gotValidity, wantValidity)
	}
}

// TestCursorNotFound checks that Cursor returns sql.ErrNoRows when the
// account and mailbox have no stored cursor.
func TestCursorNotFound(t *testing.T) {
	ctx, s := openTestStore(t)

	username := "nonexistent@example.com"
	mailbox := "INBOX"

	// Try to retrieve a cursor that was never inserted.
	err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		_, _, err := tx.Cursor(ctx, username, mailbox)
		return err
	})

	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Cursor error = %v, want sql.ErrNoRows", err)
	}
}

// TestSetCursorInsert checks that SetCursor inserts a new cursor when none exists.
func TestSetCursorInsert(t *testing.T) {
	ctx, s := openTestStore(t)

	username := "new@example.com"
	mailbox := "INBOX"
	wantUID := uint32(123)
	wantValidity := uint32(999)

	// Set a cursor that doesn't exist yet.
	if err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		return tx.SetCursor(ctx, username, mailbox, wantUID, wantValidity)
	}); err != nil {
		t.Fatalf("SetCursor: %v", err)
	}

	// Verify it was inserted.
	var gotUID, gotValidity uint32
	if err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		var e error
		gotUID, gotValidity, e = tx.Cursor(ctx, username, mailbox)
		return e
	}); err != nil {
		t.Fatalf("Cursor: %v", err)
	}
	if gotUID != wantUID {
		t.Errorf("UID = %d, want %d", gotUID, wantUID)
	}
	if gotValidity != wantValidity {
		t.Errorf("Validity = %d, want %d", gotValidity, wantValidity)
	}
}

// TestSetCursorUpdate checks that SetCursor updates an existing cursor.
func TestSetCursorUpdate(t *testing.T) {
	ctx, s := openTestStore(t)

	username := "existing@example.com"
	mailbox := "INBOX"
	oldUID := uint32(42)
	oldValidity := uint32(100)
	newUID := uint32(200)
	newValidity := uint32(500)

	// Insert initial cursor.
	if err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		return tx.SetCursor(ctx, username, mailbox, oldUID, oldValidity)
	}); err != nil {
		t.Fatalf("SetCursor (initial): %v", err)
	}

	// Update the cursor.
	if err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		return tx.SetCursor(ctx, username, mailbox, newUID, newValidity)
	}); err != nil {
		t.Fatalf("SetCursor (update): %v", err)
	}

	// Verify the values were updated.
	var gotUID, gotValidity uint32
	if err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		var e error
		gotUID, gotValidity, e = tx.Cursor(ctx, username, mailbox)
		return e
	}); err != nil {
		t.Fatalf("Cursor: %v", err)
	}
	if gotUID != newUID {
		t.Errorf("UID = %d, want %d", gotUID, newUID)
	}
	if gotValidity != newValidity {
		t.Errorf("Validity = %d, want %d", gotValidity, newValidity)
	}
}

// TestSetCursorFailure checks that SetCursor returns an error when the
// database operation fails (e.g., with a cancelled context).
func TestSetCursorFailure(t *testing.T) {
	ctx, s := openTestStore(t)

	username := "fail@example.com"
	mailbox := "INBOX"
	uid := uint32(42)
	uidValidity := uint32(100)

	// Create a cancelled context to cause ExecContext to fail.
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	// Attempt to set a cursor with the cancelled context.
	err := s.WithTx(ctx, func(ctx context.Context, tx *Tx) error {
		// Use the cancelled context for SetCursor instead of the transaction context.
		return tx.SetCursor(cancelledCtx, username, mailbox, uid, uidValidity)
	})

	if err == nil {
		t.Fatal("SetCursor with cancelled context returned nil error, want failure")
	}
	if !strings.Contains(err.Error(), "set imap cursor") {
		t.Errorf(
			"SetCursor error message = %q, want to contain %q",
			err.Error(),
			"set imap cursor",
		)
	}
}

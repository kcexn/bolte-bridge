package sqlite

import (
	"context"
	"fmt"
)

// Cursor returns the current UID and UIDValidity values of the local IMAP client.
func (t *Tx) Cursor(ctx context.Context, username, mailbox string) (uint32, uint32, error) {
	var uid, uidValidity int
	err := t.Tx.QueryRowContext(
		ctx,
		`SELECT last_seen_uid, uid_validity
		 FROM imap_cursors
		 WHERE account_id=? AND mailbox_name=?`,
		username,
		mailbox,
	).Scan(&uid, &uidValidity)
	if err != nil {
		return 0, 0, err
	}
	return uint32(uid), uint32(uidValidity), nil
}

// SetCursor updates the current UID and UIDValidity values of the local IMAP client.
func (t *Tx) SetCursor(
	ctx context.Context,
	username, mailbox string,
	uid, uidValidity uint32,
) error {
	_, err := t.Tx.ExecContext(
		ctx,
		`INSERT INTO imap_cursors (account_id, mailbox_name, last_seen_uid, uid_validity)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT (account_id, mailbox_name)
		 DO UPDATE SET
		   last_seen_uid = excluded.last_seen_uid,
		   uid_validity = excluded.uid_validity,
		   updated_at = CURRENT_TIMESTAMP`,
		username,
		mailbox,
		uid,
		uidValidity,
	)
	if err != nil {
		return fmt.Errorf("store: set imap cursor: %w", err)
	}
	return nil
}

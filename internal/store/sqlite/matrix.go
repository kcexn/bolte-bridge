package sqlite

import (
	"context"
	"fmt"
)

func (t *TxMatrix) Cursor(ctx context.Context, serverName, roomID string) (string, error) {
	var eventID string
	err := t.Tx.QueryRowContext(
		ctx,
		`SELECT last_seen_event
		 FROM matrix_cursors
		 WHERE server_name=? AND room_id=?`,
		serverName,
		roomID,
	).Scan(&eventID)
	return eventID, err
}

func (t *TxMatrix) SetCursor(ctx context.Context, serverName, roomID, eventID string) error {
	_, err := t.Tx.ExecContext(
		ctx,
		`INSERT INTO matrix_cursors (server_name, room_id, last_seen_event)
		 VALUES (?, ?, ?)
		 ON CONFLICT (server_name, room_id)
		 DO UPDATE SET
		   last_seen_event = excluded.last_seen_event,
		   updated_at = CURRENT_TIMESTAMP`,
		serverName,
		roomID,
		eventID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: set matrix cursor: %w", err)
	}
	return nil
}

func (t *TxMatrix) SetGhostMapping(ctx context.Context, mxid, email string) error {
	_, err := t.Tx.ExecContext(
		ctx,
		`INSERT INTO matrix_ghost_users (mxid, email_address)
		 VALUES (?, ?)
		 ON CONFLICT (mxid)
		 DO UPDATE SET
		   email_address = excluded.email_address,
		   updated_at = CURRENT_TIMESTAMP`,
		mxid,
		email,
	)
	if err != nil {
		return fmt.Errorf("sqlite: set matrix ghost mapping: %w", err)
	}
	return nil
}

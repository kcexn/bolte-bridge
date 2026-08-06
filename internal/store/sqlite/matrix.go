package sqlite

import (
	"context"
	"fmt"
)

// SetMatrixCursor updates the current sync token (EventID) for the Matrix client.
func (t *Tx) SetMatrixCursor(ctx context.Context, cursor string) error {
	_, err := t.Tx.ExecContext(
		ctx,
		`INSERT INTO matrix_cursor (client_id, sync_token)
		 VALUES ('default', ?)
		 ON CONFLICT (client_id)
		 DO UPDATE SET
		   sync_token = excluded.sync_token,
		   updated_at = CURRENT_TIMESTAMP`,
		cursor,
	)
	if err != nil {
		return fmt.Errorf("store: set matrix cursor: %w", err)
	}
	return nil
}

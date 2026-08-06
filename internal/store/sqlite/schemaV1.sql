-- Migration 1: initial provisioning (PRAGMA user_version 0 -> 1).
--
-- imap_cursors keeps track of the last seen IMAP UID for every mailbox.
CREATE TABLE IF NOT EXISTS imap_cursors (
    account_id    TEXT NOT NULL,
    mailbox_name  TEXT NOT NULL,
    uid_validity  INTEGER NOT NULL,
    last_seen_uid INTEGER NOT NULL DEFAULT 0,
    updated_at    TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (account_id, mailbox_name)
) STRICT;

-- matrix_cursor keeps track of the last seen EventID for the Matrix room.
CREATE TABLE IF NOT EXISTS matrix_cursor (
    client_id  TEXT PRIMARY KEY DEFAULT 'default',
    sync_token TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;
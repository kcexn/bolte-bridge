-- Migration 2: map ghost MXIDs to email addresses (PRAGMA user_version 1 -> 2).
--
-- matrix_ghost_users keeps track of the mapping from generated MXID back to
-- the original sender's email address.
CREATE TABLE IF NOT EXISTS matrix_ghost_users (
    mxid          TEXT PRIMARY KEY,
    email_address TEXT NOT NULL,
    updated_at    TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;

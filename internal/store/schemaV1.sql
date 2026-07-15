-- Migration 1: initial provisioning (PRAGMA user_version 0 -> 1).
--
-- bridge_meta is infrastructure metadata, not a domain table: it gives the
-- migration mechanism something real to create so that provisioning is
-- verifiable. Domain tables (cursors, message↔event map, identities) are added
-- by later migrations as those features are designed.
CREATE TABLE IF NOT EXISTS bridge_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;

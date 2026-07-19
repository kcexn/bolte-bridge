package config

import (
	"errors"

	"bolte-bridge/internal/store"
)

// defaultDBPath is the state database location used when neither --db-path nor
// BOLTE_BRIDGE_DB_PATH is set.
const defaultDBPath = "bolte-bridge.db"

// storeSection configures the state store. It currently owns only the SQLite
// database path; further store settings are added here as the store grows,
// without touching Load or Config's other blocks.
func storeSection(b *Binder) (ApplyFunc, error) {
	b.StringP("db.path", "db-path", "d", defaultDBPath, "path to the bolte-bridge database.")

	return func(cfg *Config) error {
		path := b.Viper().GetString("db.path")
		if path == "" {
			return errors.New("db.path must not be empty")
		}

		cfg.Store = store.Config{
			SQLite: store.SQLiteConfig{
				Path: path,
			},
		}
		return nil
	}, nil
}

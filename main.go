package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"bolte-bridge/internal/store"
)

// defaultDBPath is used when BOLTE_BRIDGE_DB is unset.
const defaultDBPath = "bolte-bridge.db"

// run performs one invocation of the bridge.
func run() error {
	ctx := context.Background()

	dbPath := os.Getenv("BOLTE_BRIDGE_DB")
	if dbPath == "" {
		dbPath = defaultDBPath
	}

	if err := store.Init(ctx, store.Config{
		SQLite: store.SQLiteConfig{Path: dbPath},
	}); err != nil {
		return fmt.Errorf("initialise store: %w", err)
	}
	defer func() { _ = store.Client().Close(ctx) }()

	log.Printf("store initialised at %s", dbPath)
	// TODO: implement the rest of the relay here.
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("bolte-bridge: %v", err)
	}
}

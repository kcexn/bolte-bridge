package config

import (
	"errors"

	"bolte-bridge/internal/matrix"
)

// matrixSection configures the Matrix appservice client. Homeserver, identity,
// and room take flags; the appservice tokens are env-only secrets, like the
// email password, so they never appear in argv.
func matrixSection(b *Binder) (ApplyFunc, error) {
	b.String(
		"matrix.homeserver-url",
		"matrix-homeserver-url",
		"",
		"base URL of the homeserver Client-Server API (e.g. https://matrix.org).",
	)
	b.String(
		"matrix.server-name",
		"matrix-server-name",
		"",
		"homeserver server_name (e.g. matrix.org).",
	)
	b.String("matrix.appservice-id", "matrix-appservice-id", "", "appservice registration id.")
	b.Secret("matrix.as-token", "")
	b.Secret("matrix.hs-token", "")
	b.String(
		"matrix.sender-localpart",
		"matrix-sender-localpart",
		"",
		"appservice bot user localpart; its user reads the room via /sync.",
	)
	b.String("matrix.room-id", "matrix-room-id", "", "the Matrix room to bridge (!room:server).")

	return func(cfg *Config) error {
		v := b.Viper()

		homeserverURL := v.GetString("matrix.homeserver-url")
		if homeserverURL == "" {
			return errors.New("matrix.homeserver-url must not be empty")
		}
		serverName := v.GetString("matrix.server-name")
		if serverName == "" {
			return errors.New("matrix.server-name must not be empty")
		}
		appServiceID := v.GetString("matrix.appservice-id")
		if appServiceID == "" {
			return errors.New("matrix.appservice-id must not be empty")
		}
		asToken := v.GetString("matrix.as-token")
		if asToken == "" {
			return errors.New(
				"matrix.as-token must not be empty (set BOLTE_BRIDGE_MATRIX_AS_TOKEN)",
			)
		}
		hsToken := v.GetString("matrix.hs-token")
		if hsToken == "" {
			return errors.New(
				"matrix.hs-token must not be empty (set BOLTE_BRIDGE_MATRIX_HS_TOKEN)",
			)
		}
		senderLocalpart := v.GetString("matrix.sender-localpart")
		if senderLocalpart == "" {
			return errors.New("matrix.sender-localpart must not be empty")
		}
		roomID := v.GetString("matrix.room-id")
		if roomID == "" {
			return errors.New("matrix.room-id must not be empty")
		}

		cfg.Matrix = matrix.Config{
			HomeserverURL:   homeserverURL,
			ServerName:      serverName,
			AppServiceID:    appServiceID,
			ASToken:         asToken,
			HSToken:         hsToken,
			SenderLocalpart: senderLocalpart,
			RoomID:          roomID,
		}
		return nil
	}, nil
}

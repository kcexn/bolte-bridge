// Package matrix provides the bridge's Matrix Adapter transport client.
package matrix

import (
	"context"
	"fmt"
	"time"

	"maunium.net/go/mautrix/appservice"
)

// Config holds the homeserver and appservice-registration settings for a
// Client. Values are resolved by the configuration factory (see
// internal/config); this package applies no defaults of its own.
type Config struct {
	// HomeserverURL is the base URL of the homeserver's Client-Server API
	// (e.g. "https://matrix.org").
	HomeserverURL string

	// ServerName is the homeserver's server_name — the domain half of a user
	// ID (e.g. "matrix.org"). It is used to build the bot and ghost user IDs.
	ServerName string

	// AppServiceID is the appservice registration's id.
	AppServiceID string

	// ASToken is the appservice access token (as_token). It authenticates both
	// the /sync read (as the bot user) and impersonated ghost-user sends.
	ASToken string

	// HSToken is the homeserver access token (hs_token) from the registration.
	HSToken string

	// SenderLocalpart is the appservice bot user's localpart. Its user,
	// "@<SenderLocalpart>:<ServerName>", is the identity used and must
	// be a member of RoomID for the room's timeline to be visible.
	SenderLocalpart string

	// RoomID is the single Matrix room this client bridges ("!room:server").
	RoomID string
}

// RawEvent is a single fetched m.room.message event, projected to the fields
// the core relay needs. The sync cursor is returned separately by Fetch (as a
// next_batch token), not carried per event, because Matrix advances the read
// cursor once per sync rather than once per message.
type RawEvent struct {
	// EventID is the event's unique ID ("$...").
	EventID string

	// Sender is the full Matrix user ID of the sender ("@user:server"). The
	// core relay uses it for identity mapping and loop prevention; this client
	// does not filter its own traffic.
	Sender string

	// RoomID is the room the event was sent to ("!room:server").
	RoomID string

	// Body is the plain-text message body.
	Body string

	// MsgType is the m.room.message msgtype (e.g. "m.text").
	MsgType string

	// ThreadRoot is the event ID of the thread root when the message is part of
	// a thread (m.relates_to rel_type m.thread), otherwise empty.
	ThreadRoot string

	// ReplyTo is the event ID this message replies to (m.in_reply_to),
	// otherwise empty.
	ReplyTo string

	// Timestamp is the origin_server_ts the homeserver stamped on the event.
	Timestamp time.Time
}

// OutboundEvent is a single message to post into a room, as a specific sender.
// It is the egress counterpart of RawEvent.
type OutboundEvent struct {
	// Sender is the full Matrix user ID to post as ("@ghost:server"). The
	// client impersonates it via the appservice token, registering and joining
	// the user as needed.
	Sender string

	// RoomID is the room to post into ("!room:server").
	RoomID string

	// Body is the plain-text message body.
	Body string

	// ThreadRoot, when set, threads the message under the given thread-root
	// event ID (m.relates_to rel_type m.thread).
	ThreadRoot string

	// ReplyTo, when set, marks the message as a reply to the given event ID
	// (m.in_reply_to).
	ReplyTo string
}

// Client is the transport surface of the Matrix Adapter. Reads are a one-shot
// sync; sends post as impersonated ghost users. Advancing the read cursor is
// the caller's responsibility, not a side effect of Fetch.
type Client interface {
	// Fetch returns every m.room.message in the configured room that arrived
	// after the sync token since, oldest first, together with the new
	// next_batch token to pass as since on the following call. A since of ""
	// performs an initial sync. Advancing the cursor — persisting next_batch —
	// remains the caller's job, not a side effect of Fetch.
	Fetch(ctx context.Context, since string) (events []RawEvent, err error)

	// Send posts one message into msg.RoomID as msg.Sender, ensuring that user
	// is registered and joined first. It is called once per message and reports
	// the outcome of this message alone; it does not batch.
	Send(ctx context.Context, msg OutboundEvent) error
}

// NewClient constructs a Client for the given appservice account.
func NewClient(ctx context.Context, cfg Config) (Client, error) {
	return newMatrixClient(ctx, cfg)
}

// matrixClient is the mautrix-appservice implementation of Client. It holds
// validated configuration and a single appservice handle from which the sync
// client and per-user intents are derived.
type matrixClient struct {
	cfg Config
	as  *appservice.AppService
}

// validate reports the first required field left unset on the Config.
func (cfg Config) validate() error {
	required := []struct {
		name  string
		value string
	}{
		{"HomeserverURL", cfg.HomeserverURL},
		{"ServerName", cfg.ServerName},
		{"AppServiceID", cfg.AppServiceID},
		{"ASToken", cfg.ASToken},
		{"HSToken", cfg.HSToken},
		{"SenderLocalpart", cfg.SenderLocalpart},
		{"RoomID", cfg.RoomID},
	}
	for _, f := range required {
		if f.value == "" {
			return fmt.Errorf("matrix: Config.%s is required", f.name)
		}
	}
	return nil
}

// newAppService builds an appservice handle from cfg, wiring the registration
// tokens and homeserver coordinates.
func newAppService(cfg Config) (*appservice.AppService, error) {
	reg := appservice.CreateRegistration()
	reg.ID = cfg.AppServiceID
	reg.AppToken = cfg.ASToken
	reg.ServerToken = cfg.HSToken
	reg.SenderLocalpart = cfg.SenderLocalpart

	as, err := appservice.CreateFull(appservice.CreateOpts{
		Registration:     reg,
		HomeserverDomain: cfg.ServerName,
		HomeserverURL:    cfg.HomeserverURL,
	})
	if err != nil {
		return nil, fmt.Errorf("matrix: create appservice: %w", err)
	}
	return as, nil
}

// newMatrixClient validates cfg and returns a ready matrixClient.
func newMatrixClient(ctx context.Context, cfg Config) (*matrixClient, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	as, err := newAppService(cfg)
	if err != nil {
		return nil, err
	}

	if err := ensureJoined(ctx, as, cfg.RoomID); err != nil {
		return nil, err
	}

	return &matrixClient{cfg: cfg, as: as}, nil
}

// ensureJoined joins the bot to roomID if it is not already a member.
func ensureJoined(ctx context.Context, as *appservice.AppService, roomID string) error {
	if _, err := as.BotClient().JoinRoom(ctx, roomID, nil); err != nil {
		return fmt.Errorf("matrix: join_room %q: %w", roomID, err)
	}
	return nil
}

// Compile-time assertion that matrixClient satisfies Client.
var _ Client = (*matrixClient)(nil)

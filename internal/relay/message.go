// Package relay defines the medium-agnostic types exchanged between the
// Email and Matrix adapters and the Core Relay. Adapters parse native events
// into these types on ingest and reconstruct native events on egress.
// The core never deals with the underlying wire formats.
package relay

// Medium identifies which adapter a Message came from (or is bound for).
type Medium int

const (
	MediumEmail Medium = iota
	MediumMatrix
)

// Address identifies an address on a given Medium (email or Matrix).
// It is used to identify both the sender and the intended recipient of a Message.
// Adapters use address information to route messages to their correct destinations.
type Address struct {
	Mode Medium // The mode of communication. Used to disambiguate and parse the ID.
	ID   string // The identifier of the address (email address/Matrix room/Matrix user).
}

// Identity provides sender/recipient information. The Core Relay maps
// the source-side identity (email address or Matrix user) to a
// target-side identity (Matrix user or email address) via the state store.
// Adapters do not resolve the mapping themselves.
type Identity struct {
	Address     Address // The source-side identity of the sender/recipient.
	DisplayName string  // "Alice" — best-effort, for display fidelity
}

// Message is the internal, medium-agnostic representation used by the core-relay to
// receive messages from adapters.
type Message struct {
	// --- Envelope ---
	Sender Identity // The source-side identity of the sender.

	// --- Metadata ---
	MessageID string // Unique identifier for this message. Provided by the sender.
	ThreadID  string // Unique identifier for the thread this message belongs to. Provided by the sender.

	// --- Content ---
	Body string // canonical plain-text body (always populated)
}

// RoutedMessage is a Message that has been routed to a specific target-side address
// by the core-relay. RoutedMessage is used by adapters to send messages to the
// correct destination in the target medium.
type RoutedMessage struct {
	// --- Message Payload ---
	Message Message // The message being routed.

	// --- Routing ---
	To Address // The target-side address to send this message to.
}

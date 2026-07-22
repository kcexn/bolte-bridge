package core

import (
	"context"

	"bolte-bridge/internal/relay"
)

// FilterFunc reports whether a message should continue through the pipeline.
// Returning keep=false drops the message silently — this is where loop
// prevention lives: the bridge's own traffic must never be re-bridged. An
// error, by contrast, aborts the whole tick so nothing is committed and the
// messages replay on the next run.
type FilterFunc func(ctx context.Context, msg relay.Message) (keep bool, err error)

// SanitizeFunc transforms a message's content as it moves through the
// pipeline — normalizing the body, stripping list footers or quoted
// signatures, and the like. Sanitizers are chained: each receives the output
// of the previous one, so ordering is significant.
type SanitizeFunc func(ctx context.Context, msg relay.Message) (relay.Message, error)

// MapFunc folds a medium-agnostic Message toward a fully-addressed
// RoutedMessage — resolving the target-side identity via the state store,
// setting the destination Address, and correlating threads. Mappers are
// chained: the first receives the sanitized Message wrapped in a zero-valued
// RoutedMessage, and each subsequent mapper refines that result.
type MapFunc func(ctx context.Context, rm relay.RoutedMessage) (relay.RoutedMessage, error)

// PublishFunc is the terminal sink of the pipeline: it delivers one
// fully-routed message into the target medium, typically by calling an
// Adapter's Send.
type PublishFunc func(ctx context.Context, rm relay.RoutedMessage) error

// RelayEngine is a configurable message-processing pipeline. Each stage is
// parameterized as a slice (or, for the terminal sink, a single func).
// Construct one per direction (room→list, list→room) with stages
// appropriate to that flow.
type RelayEngine struct {
	// Filters gate messages into the pipeline. Every filter must return
	// keep=true for a message to proceed; the first keep=false drops it.
	Filters []FilterFunc

	// Sanitizers transform message content. They are applied in order, each
	// receiving the previous one's output.
	Sanitizers []SanitizeFunc

	// Mappers resolve routing and identity. They are applied in order, each
	// refining the RoutedMessage produced so far.
	Mappers []MapFunc

	// Publish delivers the final RoutedMessage. It is mandatory.
	Publish PublishFunc
}

// Process runs msgs one at a time through the pipeline in order: filters,
// sanitizers, mappers, publish.
func (e *RelayEngine) Process(ctx context.Context, msgs []relay.Message) error {
	if e.Publish == nil {
		panic("core: RelayEngine.Process called with nil Publish")
	}

	for _, msg := range msgs {
		keep, err := e.filter(ctx, msg)
		if err != nil {
			return err
		}
		if !keep {
			continue
		}

		msg, err = e.sanitize(ctx, msg)
		if err != nil {
			return err
		}

		routed, err := e.mapMessage(ctx, msg)
		if err != nil {
			return err
		}

		if err := e.Publish(ctx, routed); err != nil {
			return err
		}
	}

	return nil
}

// filter consults every filter in order, returning keep=false as soon as one
// rejects the message.
func (e *RelayEngine) filter(ctx context.Context, msg relay.Message) (bool, error) {
	for _, f := range e.Filters {
		keep, err := f(ctx, msg)
		if err != nil {
			return false, err
		}
		if !keep {
			return false, nil
		}
	}
	return true, nil
}

// sanitize chains the sanitizers, threading each one's output into the next.
func (e *RelayEngine) sanitize(ctx context.Context, msg relay.Message) (relay.Message, error) {
	for _, s := range e.Sanitizers {
		var err error
		msg, err = s(ctx, msg)
		if err != nil {
			return relay.Message{}, err
		}
	}
	return msg, nil
}

// mapMessage seeds a RoutedMessage from the sanitized Message and folds it
// through each mapper in order.
func (e *RelayEngine) mapMessage(
	ctx context.Context,
	msg relay.Message,
) (relay.RoutedMessage, error) {
	routed := relay.RoutedMessage{Message: msg}
	for _, m := range e.Mappers {
		var err error
		routed, err = m(ctx, routed)
		if err != nil {
			return relay.RoutedMessage{}, err
		}
	}
	return routed, nil
}

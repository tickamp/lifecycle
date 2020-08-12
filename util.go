package lifecycle

import (
	"context"
	"time"
)

// DropContext is a helper function wrapping a context-naive hook as a context
// hook. The context provided to the resulting ContextHook is discarded.
func DropContext(hook Hook) ContextHook {
	if hook == nil {
		return nil
	}
	return func(ctx context.Context) error {
		return hook()
	}
}

// Wait is a helper function replacing a readiness probe for cases where it
// is relevant to wait a specified amount of time before a service is ready.
// This function returns a channel that is closed after the specified duration.
func Wait(duration time.Duration) func() <-chan error {
	return func() <-chan error {
		ch := make(chan error)
		go func() {
			<-time.After(duration)
			close(ch)
		}()
		return ch
	}
}

package lifecycle

import "context"

// Service represents a process that can be started, shut down or terminated. It
// exposes a set of methods to control the state of the service.
type Service interface {
	// Name provides a user-friendly name for the service, that is used in
	// the logs.
	Name() string
	// Starts the service. This function blocks until the service is
	// stopped.
	Start() error
	// Starts the service providing context. This function blocks until the
	// service is stopped.
	StartCtx(ctx context.Context) error
	// StartBackground starts the service in the background. This function does
	// not block. It should typically be used in combination with Done.
	StartBackground() error
	// StartBackground starts the service in the background providing context.
	// This function does not block. It should typically be used in combination
	// with Done.
	StartBackgroundCtx(ctx context.Context) error
	// Shutdown shuts the service down gracefully.
	Shutdown() error
	// ShutdownCtx shuts the service down gracefully providing context.
	ShutdownCtx(ctx context.Context) error
	// Terminate forcefully terminates the service.
	Terminate() error
	// TerminateCtx forcefully terminates the service providing context.
	TerminateCtx(ctx context.Context) error
	// Ready returns a chan that is closed when the service is either started or
	// has transitioned to an Error state.
	Ready() <-chan struct{}
	// Done returns a chan that is closed when the service is either stopped or
	// has transitioned to an Error state.
	Done() <-chan struct{}
	// State returns the current state of the service.
	State() State
	// Observes registers a chan on which the service will post lifecycle events
	// such as state changes and errors. No action is taken if ch is nil.
	Observe(ch chan<- Event)
	// Unobserve removes the provided chan from the list of observers. No action
	// is taken if ch is nil or not in the list of observers.
	Unobserve(ch chan<- Event)
}

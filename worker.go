package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Event is a struct passed to a service observers on a state change. It
// contains contextual information about the event.
type Event struct {
	// The context of the event, typically the one passed to the function from
	// which the event originated.
	Context context.Context
	// The error that caused this status change, if any.
	Error error
	// The previous status of the service.
	From State
	// The new status of the service.
	To State
}

// Hooks contain the functions called by the worker to control the underlying
// service.
type Hooks struct {
	// A friendly name for the service (optional)
	Name string
	// Start the service. This function is expected to block while the service
	// is running. Returning from this function will cause the service to
	// transition to Stopped if no error is returned or if the returned error is
	// ignored by the Error hook. In other cases, the service will transition to
	// an Error state.
	Start ContextHook
	// Gracefully shuts down the service. This function is expected to block
	// until the service is shut down. Returning an error from this function
	// will cause the service to transition to an Error state, unless the error
	// is ignored by the Error hook. In this case, the server will remain in a
	// ShuttingDown state until it is either terminated, or the Start hook
	// returns.
	Shutdown ContextHook
	// Terminates the service. This function is expected to quickly terminate
	// the service. The service will remain blocked until this function returns.
	// Returning an error from this function will cause the service to
	// transition to an Error state, unless the error is ignored by the Error
	// hook. In this case, the server will remain in a Terminating state until
	// it is either terminated, or the Start hook returns.
	Terminate ContextHook
	// Error receives error events. The event struct contains the context
	// passed to the hook from which it occured, as well as the error itself.
	// The error returned by this function will be passed to the caller. If nil
	// is returned, the error is ignored. This can be useful to suppress errors,
	// for example http.ErrServerClosed when the service wraps an HTTP server.
	Error ErrorHook
}

func (h Hooks) copy() *Hooks {
	return &h
}

// ServiceOptions contains options for the service.
type ServiceOptions struct {
	// ReadinessProbe allows to specify how the service transitions from a
	// Starting to a Started state. If provided, the service will wait for the
	// chan returned by this function to either be closed, or to return
	// an error. In the latter case, the service will transition to an Error
	// state, unless the error is ignored by the Error hook.
	ReadinessProbe func() <-chan error
	// ShutdownTimeout defines a maximum amount of time for which the service
	// can remain in ShuttingDown state. When the specified amount of time
	// is elapsed, the service is terminated (default: 15 seconds).
	ShutdownTimeout time.Duration
	// Signals defines the signals to listen to. When one of these signals is
	// received, the action defined by SignalAction will be taken (default:
	// syscall.SIGINT, syscall.SIGTERM).
	Signals []os.Signal
	// Defines the action to be taken when a signal is received (default:
	// Shutdown)
	SignalAction Action
	// Sets the Logger to use to log worker events. If nil, the logging messages
	// are discarded.
	Logger Logger
}

func (o ServiceOptions) copy() *ServiceOptions {
	return &o
}

// Worker is a Service that can be started, stopped and terminated based on a
// set of provided hooks.
type Worker struct {
	// Service hooksification
	hooks *Hooks
	// Service options
	opts *ServiceOptions
	// Current state
	state State
	// Enforces atomic state change
	mut sync.Mutex
	// Prevent against double close of the done chan
	unlockOnce sync.Once
	// Closed when we are ready
	ready chan struct{}
	// Closed when we are done
	done chan struct{}
	// Observers
	observers []chan<- Event
}

// NewWorker creates a Worker with the provided hooks. It returns nil if either
// of the hook structure, the start hook or the shutdown hook is nil.
func NewWorker(hooks *Hooks) *Worker {
	return NewWorkerWithOptions(hooks, nil)
}

// NewWorkerWithOptions creates a Worker with the provided hooks and options It
// returns nil if either  of the hook structure, the start hook or the shutdown
// hook is nil.
func NewWorkerWithOptions(hooks *Hooks, opts *ServiceOptions) *Worker {
	if hooks == nil || hooks.Start == nil || hooks.Shutdown == nil {
		return nil
	}
	if opts == nil {
		opts = &ServiceOptions{}
	}
	hooks = hooks.copy()
	opts = opts.copy()
	if opts.Signals == nil {
		opts.Signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}
	if opts.ShutdownTimeout == 0 {
		opts.ShutdownTimeout = 15 * time.Second
	}
	if opts.SignalAction == Undefined {
		opts.SignalAction = Shutdown
	}
	return &Worker{
		hooks: hooks,
		opts:  opts,
		state: Initial,
		ready: make(chan struct{}),
		done:  make(chan struct{}),
	}
}

// Start the service. This function blocks until the service is stopped. This
// function returns a non-nil error if either the start hook or the readiness
// probe return an error, unless the error is ignored by the Error hook.
func (c *Worker) Start() error {
	return c.StartCtx(context.Background())
}

// StartCtx starts the service providing context. This function blocks until the
// service is stopped. This function returns a non-nil error if either the start
// hook or the readiness probe return an error, unless the error is ignored by
// the Error hook.
func (c *Worker) StartCtx(ctx context.Context) error {
	if err := c.StartBackgroundCtx(ctx); err != nil {
		return err
	}
	<-c.Done()
	return nil
}

// StartBackground starts the service in the background. This function does not
// block. It should typically be used in combination with Done. This function
// returns a non-nil error if either the start hook or the readiness probe
// return an error, unless the error is ignored by the Error hook.
func (c *Worker) StartBackground() error {
	return c.StartBackgroundCtx(context.Background())
}

// StartBackgroundCtx starts the service in the background providing background.
// This function does not block. It should typically be used in combination with
// Done. This function returns a non-nil error if either the start hook or the
// readiness probe return an error, unless the error is ignored by the Error
// hook.
func (c *Worker) StartBackgroundCtx(ctx context.Context) error {
	// Transition to starting
	if _, err := c.transition(ctx, Starting,
		[]State{Initial}, nil); err != nil {
		return err
	}

	// Start service
	go func() {
		defer c.unblockWaiters()

		var err error
		if c.hooks.Start != nil {
			err = c.hooks.Start(ctx)
		}
		if err != nil {
			c.handleError(ctx, err)
		}

		// Transition to Stopped ; exclude error state in case it was set
		// already by handleError above.
		c.transition(ctx, Stopped,
			[]State{Starting, Started, ShuttingDown, Terminating}, nil)
	}()

	// Execute readiness probe
	ready := make(chan error)
	go func() {
		defer close(ready)
		if c.opts.ReadinessProbe != nil {
			c.info("waiting for readiness")
			select {
			case err := <-c.opts.ReadinessProbe():
				ready <- err
			case <-c.done:
				c.info("interrupting readiness probe")
			}
		}
	}()

	// Install signal handlers
	if len(c.opts.Signals) > 0 {
		go func() {
			sc := make(chan os.Signal, 1)
			signal.Notify(sc, c.opts.Signals...)

			// Wait for a signal to show up or for the server to terminate
			select {
			case sig := <-sc:
				c.info("received signal", "signal", sig)
				if c.opts.SignalAction == Shutdown {
					go c.handleError(ctx, c.Shutdown())
				} else {
					go c.handleError(ctx, c.Terminate())
				}
			case <-c.done:
			}

			// Uninstall signal handlers
			signal.Stop(sc)
		}()
	}

	// Wait for the service to be ready ; the readiness probe wait is
	// interrupted on close, so we will not keep blocking
	// the caller here.
	err := <-ready
	if err != nil {
		return c.handleError(ctx, err)
	}
	close(c.ready)

	// Transition to Started
	c.transition(ctx, Started, []State{Starting}, nil)

	return nil
}

// Shutdown shuts the service down gracefully. This function returns a non-nil
// error if the Shutdown hook returns an error, unless the error is ignored by
// the Error hook.
func (c *Worker) Shutdown() error {
	return c.ShutdownCtx(context.Background())
}

// ShutdownCtx shuts the service down gracefully providing context. This
// function returns a non-nil error if the Shutdown hook returns an error,
// unless the error is ignored by the Error hook.
func (c *Worker) ShutdownCtx(ctx context.Context) error {
	// Transition to stopping
	if _, err := c.transition(ctx, ShuttingDown,
		[]State{Starting, Started}, nil); err != nil {
		return err
	}

	// Gracefully shutdown the service
	ctx, cancel := context.WithTimeout(ctx, c.opts.ShutdownTimeout)
	defer cancel()
	gracefulTermination := make(chan error)
	go func() {
		c.info("starting graceful shutdown", "timeout", c.opts.ShutdownTimeout)
		if c.hooks.Shutdown != nil {
			err := c.hooks.Shutdown(ctx)
			if err != nil {
				gracefulTermination <- err
			}
		}
		close(gracefulTermination)
	}()

	// Wait for either the server to gracefully shut down, or kill it
	var err error
	select {
	case <-ctx.Done():
		c.info("service did not terminate in time -- terminating")
		return c.TerminateCtx(ctx)
	case err = <-gracefulTermination:
	}

	if err != nil {
		return c.handleError(ctx, err)
	}

	// Transition to Started
	c.transition(ctx, Stopped, []State{ShuttingDown}, nil)

	return nil
}

// Terminate forcefully terminates the service. This function returns a non-nil
// error if the Shutdown hook returns an error, unless the error is ignored by
// the Error hook.
func (c *Worker) Terminate() error {
	return c.TerminateCtx(context.Background())
}

func (c *Worker) TerminateCtx(ctx context.Context) error {
	// Transition to stopping
	if _, err := c.transition(ctx, Terminating,
		[]State{Starting, Started, ShuttingDown}, nil); err != nil {
		return err
	}

	var err error
	if c.hooks.Terminate != nil {
		err = c.hooks.Terminate(ctx)
	}

	if err != nil {
		return c.handleError(ctx, err)
	}

	// Transition to stopped
	c.transition(ctx, Stopped, []State{Terminating}, nil)

	// Unblock the waiters
	c.unblockWaiters()

	return nil
}

// Done returns a chan that is closed when the service is either stopped or has
// transitioned to an Error state.
func (c *Worker) Done() <-chan struct{} {
	return c.done
}

// Ready returns a chan that is closed when the service is either started or
// has transitioned to an Error state.
func (c *Worker) Ready() <-chan struct{} {
	return c.ready
}

// Name provides a user-friendly name for the service, that is used in
// the logs.
func (c *Worker) Name() string {
	return c.hooks.Name
}

// State returns the current state of the service.
func (c *Worker) State() State {
	return c.state
}

// Observe registers a chan on which the service will post lifecycle events
// such as state changes and errors. No action is taken if ch is nil.
func (c *Worker) Observe(ch chan<- Event) {
	if ch == nil {
		return
	}
	c.mut.Lock()
	defer c.mut.Unlock()
	c.observers = append(c.observers, ch)
}

// Unobserve removes the provided chan from the list of observers. No action is
// taken if ch is nil or not in the list of observers.
func (c *Worker) Unobserve(ch chan<- Event) {
	c.mut.Lock()
	defer c.mut.Unlock()
	for i, o := range c.observers {
		if o == ch {
			c.observers = append(c.observers[:i], c.observers[i+1:]...)
			break
		}
	}
}

// unblockWaiters unlocks the done chan. It is protected by a Once struct to
// avoid multiple closes, that could happen when terminate is invoked
// concurrently with shutdown.
func (c *Worker) unblockWaiters() {
	c.unlockOnce.Do(func() {
		close(c.done)
	})
}

// Transition transitions the service to a new stare. If a non-empty list of
// allowed states is provided, the service is transitioned only if the current
// state is in this list. It optionally takes the error causing this state
// change. This function is thread-safe.
func (c *Worker) transition(ctx context.Context, to State,
	allowedFromStates []State, cause error) (State, error) {
	c.mut.Lock()
	defer c.mut.Unlock()

	current := c.state
	if len(allowedFromStates) > 0 && !c.isStateOneOf(allowedFromStates) {
		err := fmt.Errorf("cannot transition from %s to %s: %w",
			current.String(), to.String(), errInvalidState)
		return current, err
	}

	c.state = to
	if to != current {
		c.info("transitioned to state", "to", to.String(), "from",
			current.String())
	}

	// Notify observers
	event := Event{
		From:  current,
		To:    to,
		Error: cause,
	}
	isFinalState := cause != nil || to == Stopped || to == Error
	for _, observer := range c.observers {
		observer <- event
		if isFinalState {
			close(observer)
		}
	}
	if isFinalState {
		c.observers = nil
	}

	return current, nil
}

// isStateOneOf checks whether the current state is in the list of provided
// states. This function is not thread-safe.
func (c *Worker) isStateOneOf(states []State) bool {
	for _, state := range states {
		if c.state == state {
			return true
		}
	}
	return false
}

// info logs an information message.
func (c *Worker) info(msg string, keysAndValues ...interface{}) {
	if c.opts.Logger != nil {
		c.opts.Logger.Info(msg, append(keysAndValues, "name", c.hooks.Name)...)
	}
}

// error logs an error
func (c *Worker) error(err error, msg string, keysAndValues ...interface{}) {
	if c.opts.Logger != nil {
		c.opts.Logger.Error(err, msg, append(keysAndValues, "name",
			c.hooks.Name)...)
	}
}

// handleError handles an error caused by an invalid state, an interruption or
// returned by a hook. It calls the Error hook if defined to transform the
// error. It also transitions the service to an Error state if the provided
// error is not an interruption error (in which case the error is expected and
// the interrupting event will set the new state itself).
func (c *Worker) handleError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	// Pass / transform error
	if c.hooks.Error != nil {
		err = c.hooks.Error(Event{
			Context: ctx,
			Error:   err,
			From:    c.State(),
		})
		if err == nil {
			return nil
		}
	}

	// Transition to Error state and unblock waiters
	c.error(err, "received error")
	if !IsInterrupted(err) {
		c.transition(ctx, Error,
			[]State{Starting, Started, ShuttingDown, Terminating}, err)
		c.unblockWaiters()
	}

	return err
}

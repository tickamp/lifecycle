package lifecycle

import "errors"

var (
	errInvalidState = errors.New("invalid state")
	errInterrupted  = errors.New("interrupted")
)

// IsInvalidState returns true if the cause of the error is an invalid initial
// state. This can be for example trying to start a stopped service, or stopping
// a stopped service.
func IsInvalidState(err error) bool {
	return errors.Is(err, errInvalidState)
}

// IsInterrupted returns true if the cause of the error is an interruption. This
// is for example returned by the service Start function when it is being
// shut down before the service is started.
func IsInterrupted(err error) bool {
	return errors.Is(err, errInterrupted)
}

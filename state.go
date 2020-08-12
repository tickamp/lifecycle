package lifecycle

import "fmt"

type State uint8

const (
	Initial State = iota
	Starting
	Started
	ShuttingDown
	Terminating
	Stopped
	Error
)

func (s State) String() string {
	switch s {
	case Initial:
		return "Initial"
	case Starting:
		return "Starting"
	case Started:
		return "Started"
	case ShuttingDown:
		return "ShuttingDown"
	case Terminating:
		return "Terminating"
	case Stopped:
		return "Stopped"
	case Error:
		return "Error"
	default:
		return fmt.Sprintf("%d", int(s))
	}
}

// Action defines the action to be taken when an event occurs.
type Action uint8

const (
	// Undefined action.
	Undefined Action = iota
	// Do nothing.
	DoNothing
	// Shutdown the service.
	Shutdown
	// Terminate the service.
	Terminate
)

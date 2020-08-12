package lifecycle

import "fmt"

// State represents a state in the lifecycle state machine. The state machine
// provided by this package is the following:
//
//          +--------------+
//          | Initial      |
//          +-+------------+
//            |
//          +-v------------+
//     +----+ Starting     +----+
//     |    +-+------------+    |
//     |      |                 |
//     |    +-v------------+    |
//     +----+ Started      +----+
//     |    +-+------------+    |
//     |      |                 |
//     |    +-v------------+    |
//     +----+ ShuttingDown +----+
//     |    +-+------------+    |
//     |      |                 |
//     |    +-v------------+  +-v------------+
//     +----+ Stopped      <--+ Terminating  |
//     |    +--------------+  +-+------------+
//     |                        |
//     |    +--------------+    |
//     +----> Error        <----+
//          +--------------+
type State uint8

const (
	// Initial state of a service
	Initial State = iota
	// Starting represents a system that is in the process of starting.
	Starting
	// Started represents a running service.
	Started
	// ShuttingDown represents a process being shut down gracefully.
	ShuttingDown
	// Terminating represents a process being forcefully terminated.
	Terminating
	// Stopped represents a service shut down without errors.
	Stopped
	// Error represents a service having reached an error.
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
	// DoNothing instructs no action.
	DoNothing
	// Shutdown the service.
	Shutdown
	// Terminate the service.
	Terminate
)

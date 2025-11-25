package goverseer

import "time"

// EventType represents the type of supervisor event.
type EventType int

const (
	// ChildStarted is emitted when a child process starts.
	ChildStarted EventType = iota
	// ChildExited is emitted when a child process exits normally.
	ChildExited
	// ChildRestarted is emitted when a child process is restarted.
	ChildRestarted
	// SupervisorStopping is emitted when the supervisor begins shutdown.
	SupervisorStopping
	// SupervisorFailedIntensity is emitted when restart intensity is exceeded.
	SupervisorFailedIntensity
	// ChildPanicked is emitted when a child process panics.
	ChildPanicked
)

// String returns the string representation of an EventType.
func (et EventType) String() string {
	switch et {
	case ChildStarted:
		return "ChildStarted"
	case ChildExited:
		return "ChildExited"
	case ChildRestarted:
		return "ChildRestarted"
	case SupervisorStopping:
		return "SupervisorStopping"
	case SupervisorFailedIntensity:
		return "SupervisorFailedIntensity"
	case ChildPanicked:
		return "ChildPanicked"
	default:
		return "Unknown"
	}
}

// Event represents a supervisor lifecycle event.
// Events are emitted for significant state changes and can be used
// for logging, metrics collection, and monitoring.
type Event struct {
	// Time is when the event occurred.
	Time time.Time
	// ChildName is the name of the child involved in the event (if applicable).
	ChildName string
	// Type is the type of event.
	Type EventType
	// Err is any error associated with the event (if applicable).
	Err error
	// StackTrace contains the panic stack trace for ChildPanicked events.
	StackTrace string
}

// EventHandler is a function that processes supervisor events.
// Multiple handlers can be registered with WithEventHandler.
// Handlers should return quickly to avoid blocking the supervisor.
type EventHandler func(e Event)

// emitEvent sends an event to all registered event handlers.
func (s *Supervisor) emitEvent(e Event) {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}

	for _, handler := range s.eventHandlers {
		// Call handlers inline - they should be fast
		// For slow handlers, users should use buffered channels
		handler(e)
	}
}

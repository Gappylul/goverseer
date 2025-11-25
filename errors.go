package goverseer

import "errors"

var (
	// ErrSupervisorStopped is returned when operations are attempted on a stopped supervisor.
	ErrSupervisorStopped = errors.New("supervisor is stopped")

	// ErrIntensityExceeded is returned when restart intensity limits are exceeded.
	// This indicates too many restarts occurred in the configured time window.
	ErrIntensityExceeded = errors.New("restart intensity exceeded")

	// ErrChildNotFound is returned when a child with the given name doesn't exist.
	ErrChildNotFound = errors.New("child not found")

	// ErrChildAlreadyExists is returned when adding a child with a name that's already in use.
	ErrChildAlreadyExists = errors.New("child already exists")

	// ErrInvalidShutdownTimeout is returned when shutdown timeout is invalid.
	ErrInvalidShutdownTimeout = errors.New("shutdown timeout must be positive")
)

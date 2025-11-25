package goverseer

import "context"

// ChildFunc is the function signature for a supervised child process.
// The function receives a context that will be canceled when the supervisor
// wants the child to stop. Children should monitor this context and exit gracefully.
//
// Returning nil indicates normal exit. Returning an error indicates abnormal exit.
// Panics are automatically recovered and treated as abnormal exits.
//
// Example:
//
//	func worker(ctx context.Context) error {
//	    ticker := time.NewTicker(time.Second)
//	    defer ticker.Stop()
//
//	    for {
//	        select {
//	        case <-ctx.Done():
//	            return nil // Graceful shutdown
//	        case <-ticker.C:
//	            if err := doWork(); err != nil {
//	                return err // Will trigger restart based on RestartType
//	            }
//	        }
//	    }
//	}
type ChildFunc func(ctx context.Context) error

// ChildSpec defines a child process specification.
// This struct describes how a child should be started and restarted.
type ChildSpec struct {
	// Name is the unique identifier for this child.
	// It's used for logging, metrics, and runtime management (AddChild, RemoveChild, etc.).
	Name string

	// Start is the function that runs the child process.
	// It receives a context that will be canceled when the child should stop.
	Start ChildFunc

	// Restart determines when this child should be restarted after exit.
	// - Permanent: Always restart (use for critical services)
	// - Transient: Restart only on error/panic (use for retriable tasks)
	// - Temporary: Never restart (use for one-off tasks)
	Restart RestartType
}

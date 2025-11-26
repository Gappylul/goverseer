package goverseer

import (
	"context"
	"time"
)

// Option configures a Supervisor during creation.
type Option func(*Supervisor)

// WithName sets the supervisor's name for logging and debugging.
//
// Example:
//
//	sup := goverseer.New(
//	    goverseer.OneForOne,
//	    goverseer.WithName("http-supervisor"),
//	)
func WithName(name string) Option {
	return func(s *Supervisor) {
		s.name = name
	}
}

// WithIntensity sets restart intensity limits to prevent restart loops.
// If more than maxRestarts occur within the time window, the supervisor
// stops permanently and returns ErrIntensityExceeded.
//
// Example:
//
//	// Allow up to 10 restarts per minute
//	sup := goverseer.New(
//	    goverseer.OneForOne,
//	    goverseer.WithIntensity(10, time.Minute),
//	)
func WithIntensity(maxRestarts int, window time.Duration) Option {
	return func(s *Supervisor) {
		s.maxRestarts = maxRestarts
		s.restartWindow = window
	}
}

// WithBackoff sets the backoff policy for restart delays.
// The policy determines how long to wait before restarting a failed child.
//
// Example:
//
//	sup := goverseer.New(
//	    goverseer.OneForOne,
//	    goverseer.WithBackoff(
//	        goverseer.ExponentialBackoff(100*time.Millisecond, 5*time.Second),
//	    ),
//	)
func WithBackoff(policy BackoffPolicy) Option {
	return func(s *Supervisor) {
		s.backoff = policy
	}
}

// WithEventHandler adds an event handler to receive supervisor events.
// Multiple handlers can be registered by calling this option multiple times.
// Handlers should return quickly to avoid blocking the supervisor.
//
// Example:
//
//	sup := goverseer.New(
//	    goverseer.OneForOne,
//	    goverseer.WithEventHandler(func(e goverseer.Event) {
//	        log.Printf("[%s] %s: %v", e.Type, e.ChildName, e.Err)
//	    }),
//	)
func WithEventHandler(handler EventHandler) Option {
	return func(s *Supervisor) {
		s.eventHandlers = append(s.eventHandlers, handler)
	}
}

// WithShutdownTimeout sets the maximum time to wait for children to stop gracefully.
// After this timeout, the supervisor will exit even if children are still running.
// The default is 30 seconds. If timeout is <= 0, the default is used.
//
// Example:
//
//	sup := goverseer.New(
//	    goverseer.OneForOne,
//	    goverseer.WithShutdownTimeout(10*time.Second),
//	)
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(s *Supervisor) {
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		s.shutdownTimeout = timeout
	}
}

// WithChildren adds initial children to the supervisor.
// Children are not started automatically; call Start() to begin supervision.
//
// Example:
//
//	sup := goverseer.New(
//	    goverseer.OneForOne,
//	    goverseer.WithChildren(
//	        goverseer.ChildSpec{Name: "worker-1", Start: worker1Func, Restart: goverseer.Permanent},
//	        goverseer.ChildSpec{Name: "worker-2", Start: worker2Func, Restart: goverseer.Transient},
//	    ),
//	)
func WithChildren(specs ...ChildSpec) Option {
	return func(s *Supervisor) {
		for _, spec := range specs {
			ch := &child{
				spec: spec,
			}
			s.children = append(s.children, ch)
			s.childMap[spec.Name] = ch
		}
	}
}

// WithContext sets a custom context for the supervisor instead of using context.Background().
// The supervisor and all its children will be canceled when this context is canceled.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
//	sup := goverseer.New(
//	    goverseer.OneForOne,
//	    goverseer.WithContext(ctx),
//	)
func WithContext(ctx context.Context) Option {
	return func(s *Supervisor) {
		s.cancel() // Cancel the default context
		s.ctx, s.cancel = context.WithCancel(ctx)
	}
}

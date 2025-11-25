// Package goverseer provides production-ready process supervision for Go applications.
// It implements Erlang/OTP-style supervision trees with restart strategies, intensity limits,
// backoff policies, and hierarchical supervision.
//
// Basic usage:
//
//	sup := goverseer.New(
//	    goverseer.OneForOne,
//	    goverseer.WithName("my-supervisor"),
//	    goverseer.WithChildren(
//	        goverseer.ChildSpec{
//	            Name:    "worker",
//	            Start:   workerFunc,
//	            Restart: goverseer.Permanent,
//	        },
//	    ),
//	)
//	sup.Start()
//	sup.Wait()
package goverseer

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Supervisor manages child processes with configurable restart strategies and intensity limits.
// It provides fault tolerance by automatically restarting failed children according to
// configured policies. Supervisors can be nested to create supervision trees.
//
// All methods are safe for concurrent use. The supervisor uses an actor model internally
// to ensure race-free state management.
type Supervisor struct {
	// Configuration
	name            string
	strategy        Strategy
	maxRestarts     int
	restartWindow   time.Duration
	backoff         BackoffPolicy
	shutdownTimeout time.Duration
	eventHandlers   []EventHandler

	// State (protected by mu or accessed via commands channel)
	mu             sync.RWMutex
	children       []*child
	childMap       map[string]*child
	ctx            context.Context
	cancel         context.CancelFunc
	done           chan struct{}
	commands       chan command
	restartHistory []time.Time
	stopped        bool
	finalErr       error
}

// command represents an internal command to the supervisor's actor loop.
type command struct {
	action   string     // "add", "remove", "restart"
	spec     *ChildSpec // for "add"
	name     string     // for "remove", "restart"
	response chan error // synchronous response channel
}

// New creates a new Supervisor with the given strategy and options.
// The supervisor must be started with Start() before it begins managing children.
//
// Example:
//
//	sup := goverseer.New(
//	    goverseer.OneForOne,
//	    goverseer.WithName("app-supervisor"),
//	    goverseer.WithIntensity(10, time.Minute),
//	    goverseer.WithBackoff(goverseer.ExponentialBackoff(100*time.Millisecond, 5*time.Second)),
//	)

func New(strategy Strategy, opts ...Option) *Supervisor {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Supervisor{
		name:            "supervisor",
		strategy:        strategy,
		maxRestarts:     10,
		restartWindow:   time.Minute,
		backoff:         ExponentialBackoff(100*time.Millisecond, 5*time.Second),
		shutdownTimeout: 30 * time.Second,
		childMap:        make(map[string]*child),
		ctx:             ctx,
		cancel:          cancel,
		done:            make(chan struct{}),
		commands:        make(chan command, 10),
		restartHistory:  make([]time.Time, 0),
	}

	for _, opt := range opts {
		opt(s)
	}

	go s.run()

	return s
}

// Start starts the supervisor and all its children in order.
// Children are started sequentially, and if any child fails to start,
// Start returns an error immediately without starting remaining children.
//
// Returns ErrSupervisorStopped if the supervisor has already been stopped.
func (s *Supervisor) Start() error {
	s.mu.RLock()
	if s.stopped {
		s.mu.RUnlock()
		return ErrSupervisorStopped
	}
	children := s.children
	s.mu.RUnlock()

	for _, ch := range children {
		if err := s.startChild(ch); err != nil {
			return fmt.Errorf("failed to start child %s: %w", ch.spec.Name, err)
		}
	}

	return nil
}

// AddChild dynamically adds a child to the supervisor at runtime.
// The child is started immediately. If a child with the same name already exists,
// returns ErrChildAlreadyExists.
//
// This operation is safe to call from any goroutine.
func (s *Supervisor) AddChild(spec ChildSpec) error {
	response := make(chan error, 1)
	s.commands <- command{
		action:   "add",
		spec:     &spec,
		response: response,
	}
	return <-response
}

// RemoveChild removes a child from the supervisor and stops it gracefully.
// If the child doesn't exist, returns ErrChildNotFound.
//
// This operation is safe to call from any goroutine.
func (s *Supervisor) RemoveChild(name string) error {
	response := make(chan error, 1)
	s.commands <- command{
		action:   "remove",
		name:     name,
		response: response,
	}
	return <-response
}

// RestartChild manually restarts a specific child by name.
// The child is stopped and a new instance is started with the same specification.
// If the child doesn't exist, returns ErrChildNotFound.
//
// This operation is safe to call from any goroutine.
func (s *Supervisor) RestartChild(name string) error {
	response := make(chan error, 1)
	s.commands <- command{
		action:   "restart",
		name:     name,
		response: response,
	}
	return <-response
}

// Stop gracefully stops the supervisor and all its children.
// It cancels the supervisor's context, waits for all children to exit
// (up to the configured shutdown timeout), and returns any final error.
//
// This method blocks until shutdown is complete.
func (s *Supervisor) Stop() error {
	s.cancel()
	<-s.done

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.finalErr
}

// Wait blocks until the supervisor stops and returns any error that caused it to stop.
// This includes errors from intensity limit violations or context cancellation.
//
// Use this in your main function to keep the supervisor running:
//
//	if err := sup.Wait(); err != nil {
//	    log.Fatal(err)
//	}
func (s *Supervisor) Wait() error {
	<-s.done
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.finalErr
}

// run is the main supervisor event loop implementing the actor model.
// All state mutations happen in this single goroutine, ensuring race-free operation.
func (s *Supervisor) run() {
	defer close(s.done)
	defer s.shutdownChildren()

	childExits := make(chan *childExit, len(s.children)+10)

	for {
		select {
		case <-s.ctx.Done():
			s.emitEvent(Event{
				Time: time.Now(),
				Type: SupervisorStopping,
			})
			return

		case cmd := <-s.commands:
			s.handleCommand(cmd, childExits)

		case exit := <-childExits:
			if err := s.handleChildExit(exit, childExits); err != nil {
				s.mu.Lock()
				s.finalErr = err
				s.stopped = true
				s.mu.Unlock()
				s.cancel()
				return
			}
		}
	}
}

// handleCommand processes commands from the commands channel.
func (s *Supervisor) handleCommand(cmd command, childExits chan *childExit) {
	var err error

	switch cmd.action {
	case "add":
		err = s.doAddChild(cmd.spec, childExits)
	case "remove":
		err = s.doRemoveChild(cmd.name)
	case "restart":
		err = s.doRestartChild(cmd.name, childExits)
	}

	cmd.response <- err
}

// doAddChild implements the add child operation.
func (s *Supervisor) doAddChild(spec *ChildSpec, childExits chan *childExit) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return ErrSupervisorStopped
	}

	if _, exists := s.childMap[spec.Name]; exists {
		return ErrChildAlreadyExists
	}

	ch := newChild(*spec, s.ctx, childExits)
	s.children = append(s.children, ch)
	s.childMap[spec.Name] = ch

	return s.startChild(ch)
}

// doRemoveChild implements the remove child operation.
func (s *Supervisor) doRemoveChild(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch, exists := s.childMap[name]
	if !exists {
		return ErrChildNotFound
	}

	ch.stop()

	// Remove from slice
	for i, c := range s.children {
		if c == ch {
			s.children = append(s.children[:i], s.children[i+1:]...)
			break
		}
	}

	delete(s.childMap, name)
	return nil
}

// doRestartChild implements the restart child operation.
func (s *Supervisor) doRestartChild(name string, childExits chan *childExit) error {
	s.mu.Lock()
	ch, exists := s.childMap[name]
	s.mu.Unlock()

	if !exists {
		return ErrChildNotFound
	}

	ch.stop()

	s.mu.Lock()
	defer s.mu.Unlock()

	ch = newChild(ch.spec, s.ctx, childExits)
	s.childMap[name] = ch

	for i, c := range s.children {
		if c.spec.Name == name {
			s.children[i] = ch
			break
		}
	}

	return s.startChild(ch)
}

// shutdownChildren gracefully shuts down all children with a timeout.
func (s *Supervisor) shutdownChildren() {
	s.mu.Lock()
	children := make([]*child, len(s.children))
	copy(children, s.children)
	s.mu.Unlock()

	// Stop all children by canceling their contexts
	for _, ch := range children {
		ch.stop()
	}

	// Wait for all to stop with timeout
	timeout := time.After(s.shutdownTimeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		allStopped := true

		s.mu.RLock()
		for _, ch := range s.children {
			if !ch.isStopped() {
				allStopped = false
				break
			}
		}
		s.mu.RUnlock()

		if allStopped {
			return
		}

		select {
		case <-timeout:
			// Force exit after timeout
			return
		case <-ticker.C:
			continue
		}
	}
}

// startChild starts a single child and emits the appropriate event.
func (s *Supervisor) startChild(ch *child) error {
	s.emitEvent(Event{
		Time:      time.Now(),
		ChildName: ch.spec.Name,
		Type:      ChildStarted,
	})

	ch.start()
	return nil
}

// handleChildExit processes a child exit and decides whether to restart.
func (s *Supervisor) handleChildExit(exit *childExit, childExits chan *childExit) error {
	eventType := ChildExited
	if exit.panic {
		eventType = ChildPanicked
	}

	s.emitEvent(Event{
		Time:       time.Now(),
		ChildName:  exit.child.spec.Name,
		Type:       eventType,
		Err:        exit.err,
		StackTrace: exit.stackTrace,
	})

	// Check if we should restart based on restart type
	shouldRestart := s.shouldRestart(exit)

	if !shouldRestart {
		return nil
	}

	// Check restart intensity to prevent restart loops
	if !s.checkRestartIntensity() {
		s.emitEvent(Event{
			Time:      time.Now(),
			ChildName: exit.child.spec.Name,
			Type:      SupervisorFailedIntensity,
		})
		return ErrIntensityExceeded
	}

	// Apply backoff delay before restart
	delay := s.backoff.ComputeDelay(exit.child.restartCount)
	if delay > 0 {
		time.Sleep(delay)
	}

	// Execute the configured restart strategy
	return s.executeStrategy(exit, childExits)
}

// shouldRestart determines if a child should be restarted based on its restart type.
func (s *Supervisor) shouldRestart(exit *childExit) bool {
	switch exit.child.spec.Restart {
	case Permanent:
		return true
	case Transient:
		return exit.err != nil || exit.panic
	case Temporary:
		return false
	default:
		return false
	}
}

// checkRestartIntensity checks if restart rate is within configured limits.
// Returns false if too many restarts have occurred in the time window.
func (s *Supervisor) checkRestartIntensity() bool {
	now := time.Now()
	s.restartHistory = append(s.restartHistory, now)

	// Remove old entries outside the window
	cutoff := now.Add(-s.restartWindow)
	start := 0
	for i, t := range s.restartHistory {
		if t.After(cutoff) {
			start = i
			break
		}
	}
	s.restartHistory = s.restartHistory[start:]

	return len(s.restartHistory) <= s.maxRestarts
}

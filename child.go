package goverseer

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
)

// child represents a supervised child process.
type child struct {
	spec         ChildSpec
	ctx          context.Context
	cancel       context.CancelFunc
	exits        chan *childExit
	restartCount int
	mu           sync.RWMutex
	stopped      bool
}

// childExit represents the exit of a child process.
type childExit struct {
	child      *child
	err        error
	panic      bool
	stackTrace string
}

// newChild creates a new child with the given specification.
func newChild(spec ChildSpec, parentCtx context.Context, exits chan *childExit) *child {
	ctx, cancel := context.WithCancel(parentCtx)

	return &child{
		spec:   spec,
		ctx:    ctx,
		cancel: cancel,
		exits:  exits,
	}
}

// start begins executing the child process in a new goroutine.
func (c *child) start() {
	go c.runWithRecovery()
}

// runWithRecovery runs the child function with panic recovery.
func (c *child) runWithRecovery() {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			err := fmt.Errorf("panic: %v", r)

			c.exits <- &childExit{
				child:      c,
				err:        err,
				panic:      true,
				stackTrace: stack,
			}
		}
	}()

	err := c.spec.Start(c.ctx)

	c.exits <- &childExit{
		child: c,
		err:   err,
		panic: false,
	}
}

// stop cancels the child's context, signaling it to shut down.
func (c *child) stop() {
	c.mu.Lock()
	c.stopped = true
	c.mu.Unlock()
	c.cancel()
}

// isStopped returns whether the child has been stopped.
func (c *child) isStopped() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stopped
}

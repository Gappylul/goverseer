package goverseer

import (
	"fmt"
	"time"
)

// Strategy defines how children are restarted when one fails.
type Strategy int

const (
	// OneForOne restarts only the failed child.
	// Use this when children are independent and can fail/restart individually.
	OneForOne Strategy = iota

	// OneForAll stops all children and restarts all when one fails.
	// Use this when children are tightly coupled and depend on each other.
	OneForAll

	// RestForOne stops and restarts the failed child and all children started after it.
	// Use this when children have startup dependencies (e.g., A must start before B).
	RestForOne

	// SimpleOneForOne is for dynamic worker pools where children are added/removed at runtime.
	// Behaves like OneForOne but optimized for many similar children.
	SimpleOneForOne
)

// String returns the string representation of a Strategy.
func (s Strategy) String() string {
	switch s {
	case OneForOne:
		return "OneForOne"
	case OneForAll:
		return "OneForAll"
	case RestForOne:
		return "RestForOne"
	case SimpleOneForOne:
		return "SimpleOneForOne"
	default:
		return "Unknown"
	}
}

// RestartType determines when a child should be restarted.
type RestartType int

const (
	// Permanent children are always restarted, even on normal exit.
	// Use this for critical services that must always be running.
	Permanent RestartType = iota

	// Transient children are restarted only if they exit abnormally (error or panic).
	// Use this for tasks that can complete successfully but should retry on failure.
	Transient

	// Temporary children are never restarted.
	// Use this for one-off initialization tasks or operations that should not retry.
	Temporary
)

// String returns the string representation of a RestartType.
func (rt RestartType) String() string {
	switch rt {
	case Permanent:
		return "Permanent"
	case Transient:
		return "Transient"
	case Temporary:
		return "Temporary"
	default:
		return "Unknown"
	}
}

// executeStrategy executes the configured restart strategy after a child fails.
func (s *Supervisor) executeStrategy(exit *childExit, childExits chan *childExit) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.strategy {
	case OneForOne:
		return s.restartOne(exit, childExits)
	case OneForAll:
		return s.restartAll(childExits)
	case RestForOne:
		return s.restartRestForOne(exit, childExits)
	case SimpleOneForOne:
		return s.restartOne(exit, childExits)
	default:
		return fmt.Errorf("unknown strategy: %d", s.strategy)
	}
}

// restartOne restarts only the failed child (OneForOne and SimpleOneForOne strategies).
func (s *Supervisor) restartOne(exit *childExit, childExits chan *childExit) error {
	newChild := newChild(exit.child.spec, s.ctx, childExits)
	newChild.restartCount = exit.child.restartCount + 1

	// Replace in map and slice
	s.childMap[exit.child.spec.Name] = newChild
	for i, ch := range s.children {
		if ch.spec.Name == exit.child.spec.Name {
			s.children[i] = newChild
			break
		}
	}

	s.emitEvent(Event{
		Time:      time.Now(),
		ChildName: newChild.spec.Name,
		Type:      ChildRestarted,
	})

	return s.startChild(newChild)
}

// restartAll stops all children and restarts all (OneForAll strategy).
func (s *Supervisor) restartAll(childExits chan *childExit) error {
	// Stop all children
	for _, ch := range s.children {
		ch.stop()
	}

	// Create new children
	newChildren := make([]*child, 0, len(s.children))
	for _, ch := range s.children {
		newChild := newChild(ch.spec, s.ctx, childExits)
		newChild.restartCount = ch.restartCount + 1
		newChildren = append(newChildren, newChild)
		s.childMap[ch.spec.Name] = newChild
	}

	s.children = newChildren

	// Start all children in order
	for _, ch := range s.children {
		s.emitEvent(Event{
			Time:      time.Now(),
			ChildName: ch.spec.Name,
			Type:      ChildRestarted,
		})
		if err := s.startChild(ch); err != nil {
			return err
		}
	}

	return nil
}

// restartRestForOne restarts the failed child and all children started after it (RestForOne strategy).
func (s *Supervisor) restartRestForOne(exit *childExit, childExits chan *childExit) error {
	// Find the index of the failed child
	failedIndex := -1
	for i, ch := range s.children {
		if ch.spec.Name == exit.child.spec.Name {
			failedIndex = i
			break
		}
	}

	if failedIndex == -1 {
		return nil
	}

	// Stop children from failedIndex onwards
	for i := failedIndex; i < len(s.children); i++ {
		s.children[i].stop()
	}

	// Restart from failedIndex onwards
	for i := failedIndex; i < len(s.children); i++ {
		oldChild := s.children[i]
		newChild := newChild(oldChild.spec, s.ctx, childExits)
		newChild.restartCount = oldChild.restartCount + 1

		s.children[i] = newChild
		s.childMap[newChild.spec.Name] = newChild

		s.emitEvent(Event{
			Time:      time.Now(),
			ChildName: newChild.spec.Name,
			Type:      ChildRestarted,
		})

		if err := s.startChild(newChild); err != nil {
			return err
		}
	}

	return nil
}

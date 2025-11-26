package goverseer

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestSupervisorBasicStartStop tests basic supervisor lifecycle
func TestSupervisorBasicStartStop(t *testing.T) {
	var started atomic.Bool

	worker := func(ctx context.Context) error {
		started.Store(true)
		<-ctx.Done()
		return nil
	}

	sup := New(
		OneForOne,
		WithName("test-supervisor"),
		WithChildren(
			ChildSpec{
				Name:    "worker",
				Start:   worker,
				Restart: Permanent,
			},
		),
	)

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	// Give worker time to start
	time.Sleep(100 * time.Millisecond)

	if !started.Load() {
		t.Fatal("worker did not start")
	}

	// Stop supervisor
	if err := sup.Stop(); err != nil {
		t.Fatalf("failed to stop supervisor: %v", err)
	}
}

// TestPermanentRestartOnError tests that Permanent children restart on error
func TestPermanentRestartOnError(t *testing.T) {
	var runCount atomic.Int32

	worker := func(ctx context.Context) error {
		count := runCount.Add(1)
		if count < 3 {
			return errors.New("simulated error")
		}
		<-ctx.Done()
		return nil
	}

	sup := New(
		OneForOne,
		WithName("permanent-test"),
		WithBackoff(ConstantBackoff(10*time.Millisecond)),
		WithChildren(
			ChildSpec{
				Name:    "failing-worker",
				Start:   worker,
				Restart: Permanent,
			},
		),
	)

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	// Wait for multiple restarts
	time.Sleep(200 * time.Millisecond)

	if runCount.Load() < 3 {
		t.Fatalf("expected at least 3 runs, got %d", runCount.Load())
	}

	sup.Stop()
}

// TestPermanentRestartOnNormalExit tests that Permanent children restart even on normal exit
func TestPermanentRestartOnNormalExit(t *testing.T) {
	var runCount atomic.Int32

	worker := func(ctx context.Context) error {
		runCount.Add(1)
		// Exit normally immediately
		return nil
	}

	sup := New(
		OneForOne,
		WithName("permanent-normal-exit"),
		WithBackoff(ConstantBackoff(10*time.Millisecond)),
		WithIntensity(5, time.Second),
		WithChildren(
			ChildSpec{
				Name:    "exiting-worker",
				Start:   worker,
				Restart: Permanent,
			},
		),
	)

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	// Wait for restarts
	time.Sleep(200 * time.Millisecond)

	if runCount.Load() < 3 {
		t.Fatalf("expected at least 3 runs, got %d", runCount.Load())
	}

	sup.Stop()
}

// TestTransientNoRestartOnNormalExit tests that Transient children don't restart on normal exit
func TestTransientNoRestartOnNormalExit(t *testing.T) {
	var runCount atomic.Int32

	worker := func(ctx context.Context) error {
		runCount.Add(1)
		return nil // Normal exit
	}

	sup := New(
		OneForOne,
		WithName("transient-test"),
		WithChildren(
			ChildSpec{
				Name:    "transient-worker",
				Start:   worker,
				Restart: Transient,
			},
		),
	)

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	// Wait and verify it stops
	err := sup.Wait()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runCount.Load() != 1 {
		t.Fatalf("expected exactly 1 run, got %d", runCount.Load())
	}
}

// TestTransientRestartOnError tests that Transient children restart on error
func TestTransientRestartOnError(t *testing.T) {
	var runCount atomic.Int32

	worker := func(ctx context.Context) error {
		count := runCount.Add(1)
		if count < 3 {
			return errors.New("simulated error")
		}
		return nil // Success on 3rd try
	}

	sup := New(
		OneForOne,
		WithName("transient-error-test"),
		WithBackoff(ConstantBackoff(10*time.Millisecond)),
		WithChildren(
			ChildSpec{
				Name:    "transient-worker",
				Start:   worker,
				Restart: Transient,
			},
		),
	)

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	err := sup.Wait()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runCount.Load() != 3 {
		t.Fatalf("expected exactly 3 runs, got %d", runCount.Load())
	}
}

// TestTemporaryNeverRestarts tests that Temporary children never restart
func TestTemporaryNeverRestarts(t *testing.T) {
	var runCount atomic.Int32

	worker := func(ctx context.Context) error {
		runCount.Add(1)
		return errors.New("error")
	}

	sup := New(
		OneForOne,
		WithName("temporary-test"),
		WithChildren(
			ChildSpec{
				Name:    "temporary-worker",
				Start:   worker,
				Restart: Temporary,
			},
		),
	)

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	err := sup.Wait()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runCount.Load() != 1 {
		t.Fatalf("expected exactly 1 run, got %d", runCount.Load())
	}
}

// TestPanicRecovery tests that panics are caught and recovered
func TestPanicRecovery(t *testing.T) {
	var runCount atomic.Int32
	var panicCaught atomic.Bool

	worker := func(ctx context.Context) error {
		count := runCount.Add(1)
		if count == 1 {
			panic("intentional panic")
		}
		<-ctx.Done()
		return nil
	}

	sup := New(
		OneForOne,
		WithName("panic-test"),
		WithBackoff(ConstantBackoff(10*time.Millisecond)),
		WithEventHandler(func(e Event) {
			if e.Type == ChildPanicked {
				panicCaught.Store(true)
			}
		}),
		WithChildren(
			ChildSpec{
				Name:    "panicking-worker",
				Start:   worker,
				Restart: Permanent,
			},
		),
	)

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !panicCaught.Load() {
		t.Fatal("panic was not caught")
	}

	if runCount.Load() < 2 {
		t.Fatalf("worker should have restarted after panic, runs: %d", runCount.Load())
	}

	sup.Stop()
}

// TestIntensityLimit tests that restart intensity limits work
func TestIntensityLimit(t *testing.T) {
	worker := func(ctx context.Context) error {
		return errors.New("always fails")
	}

	sup := New(
		OneForOne,
		WithName("intensity-test"),
		WithIntensity(3, 100*time.Millisecond),
		WithBackoff(ConstantBackoff(1*time.Millisecond)),
		WithChildren(
			ChildSpec{
				Name:    "failing-worker",
				Start:   worker,
				Restart: Permanent,
			},
		),
	)

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	err := sup.Wait()
	if !errors.Is(err, ErrIntensityExceeded) {
		t.Fatalf("expected ErrIntensityExceeded, got: %v", err)
	}
}

// TestDynamicChildManagement tests adding and removing children at runtime
func TestDynamicChildManagement(t *testing.T) {
	worker := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}

	sup := New(OneForOne, WithName("dynamic-test"))

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	// Add a child
	if err := sup.AddChild(ChildSpec{
		Name:    "dynamic-worker",
		Start:   worker,
		Restart: Permanent,
	}); err != nil {
		t.Fatalf("failed to add child: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Try to add duplicate
	err := sup.AddChild(ChildSpec{
		Name:    "dynamic-worker",
		Start:   worker,
		Restart: Permanent,
	})
	if !errors.Is(err, ErrChildAlreadyExists) {
		t.Fatalf("expected ErrChildAlreadyExists, got: %v", err)
	}

	// Remove the child
	if err := sup.RemoveChild("dynamic-worker"); err != nil {
		t.Fatalf("failed to remove child: %v", err)
	}

	// Try to remove non-existent child
	err = sup.RemoveChild("non-existent")
	if !errors.Is(err, ErrChildNotFound) {
		t.Fatalf("expected ErrChildNotFound, got: %v", err)
	}

	sup.Stop()
}

// TestManualRestart tests manually restarting a child
func TestManualRestart(t *testing.T) {
	var runCount atomic.Int32

	worker := func(ctx context.Context) error {
		runCount.Add(1)
		<-ctx.Done()
		return nil
	}

	sup := New(
		OneForOne,
		WithName("manual-restart-test"),
		WithChildren(
			ChildSpec{
				Name:    "worker",
				Start:   worker,
				Restart: Permanent,
			},
		),
	)

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	initialCount := runCount.Load()

	// Manually restart
	if err := sup.RestartChild("worker"); err != nil {
		t.Fatalf("failed to restart child: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if runCount.Load() <= initialCount {
		t.Fatal("child was not restarted")
	}

	sup.Stop()
}

// TestEventSystem tests that events are properly emitted
func TestEventSystem(t *testing.T) {
	var started, exited, restarted atomic.Bool

	worker := func(ctx context.Context) error {
		return errors.New("error") // Will cause restart
	}

	sup := New(
		OneForOne,
		WithName("event-test"),
		WithBackoff(ConstantBackoff(10*time.Millisecond)),
		WithIntensity(5, time.Second),
		WithEventHandler(func(e Event) {
			switch e.Type {
			case ChildStarted:
				started.Store(true)
			case ChildExited:
				exited.Store(true)
			case ChildRestarted:
				restarted.Store(true)
			}
		}),
		WithChildren(
			ChildSpec{
				Name:    "worker",
				Start:   worker,
				Restart: Permanent,
			},
		),
	)

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !started.Load() {
		t.Error("ChildStarted event not received")
	}
	if !exited.Load() {
		t.Error("ChildExited event not received")
	}
	if !restarted.Load() {
		t.Error("ChildRestarted event not received")
	}

	sup.Stop()
}

// TestShutdownTimeout tests graceful shutdown with timeout
func TestShutdownTimeout(t *testing.T) {
	worker := func(ctx context.Context) error {
		<-ctx.Done()
		// Simulate slow shutdown
		time.Sleep(2 * time.Second)
		return nil
	}

	sup := New(
		OneForOne,
		WithName("shutdown-test"),
		WithShutdownTimeout(100*time.Millisecond),
		WithChildren(
			ChildSpec{
				Name:    "slow-worker",
				Start:   worker,
				Restart: Permanent,
			},
		),
	)

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	start := time.Now()
	sup.Stop()
	elapsed := time.Since(start)

	// Should timeout and force exit
	if elapsed > 500*time.Millisecond {
		t.Fatalf("shutdown took too long: %v", elapsed)
	}
}

// TestConcurrentOperations tests thread safety
func TestConcurrentOperations(t *testing.T) {
	worker := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}

	sup := New(OneForOne, WithName("concurrent-test"))

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	// Add children concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			_ = sup.AddChild(ChildSpec{
				Name:    "worker-" + string(rune('0'+id)),
				Start:   worker,
				Restart: Permanent,
			})
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	time.Sleep(100 * time.Millisecond)
	sup.Stop()
}

// BenchmarkSupervisorOverhead benchmarks supervisor overhead
func BenchmarkSupervisorOverhead(b *testing.B) {
	worker := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}

	for i := 0; i < b.N; i++ {
		sup := New(
			OneForOne,
			WithChildren(
				ChildSpec{
					Name:    "worker",
					Start:   worker,
					Restart: Permanent,
				},
			),
		)
		sup.Start()
		sup.Stop()
	}
}

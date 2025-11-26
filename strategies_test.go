package goverseer

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestOneForOneStrategy tests OneForOne restart strategy
func TestOneForOneStrategy(t *testing.T) {
	var worker1Count, worker2Count atomic.Int32

	worker1 := func(ctx context.Context) error {
		worker1Count.Add(1)
		return errors.New("worker1 error")
	}

	worker2 := func(ctx context.Context) error {
		worker2Count.Add(1)
		<-ctx.Done()
		return nil
	}

	sup := New(
		OneForOne,
		WithName("one-for-one-test"),
		WithBackoff(ConstantBackoff(10*time.Millisecond)),
		WithIntensity(5, time.Second),
		WithChildren(
			ChildSpec{Name: "worker1", Start: worker1, Restart: Permanent},
			ChildSpec{Name: "worker2", Start: worker2, Restart: Permanent},
		),
	)

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Worker1 should restart multiple times
	if worker1Count.Load() < 3 {
		t.Fatalf("worker1 should have restarted, count: %d", worker1Count.Load())
	}

	// Worker2 should start only once (not affected by worker1 failures)
	if worker2Count.Load() != 1 {
		t.Fatalf("worker2 should start once, count: %d", worker2Count.Load())
	}

	sup.Stop()
}

// TestOneForAllStrategy tests OneForAll restart strategy
func TestOneForAllStrategy(t *testing.T) {
	var worker1Count, worker2Count atomic.Int32

	worker1 := func(ctx context.Context) error {
		count := worker1Count.Add(1)
		if count < 3 {
			return errors.New("worker1 error")
		}
		<-ctx.Done()
		return nil
	}

	worker2 := func(ctx context.Context) error {
		worker2Count.Add(1)
		<-ctx.Done()
		return nil
	}

	sup := New(
		OneForAll,
		WithName("one-for-all-test"),
		WithBackoff(ConstantBackoff(10*time.Millisecond)),
		WithChildren(
			ChildSpec{Name: "worker1", Start: worker1, Restart: Permanent},
			ChildSpec{Name: "worker2", Start: worker2, Restart: Permanent},
		),
	)

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Both workers should restart together
	if worker1Count.Load() < 2 {
		t.Fatalf("worker1 count should be >= 2, got: %d", worker1Count.Load())
	}
	if worker2Count.Load() < 2 {
		t.Fatalf("worker2 should restart with worker1, count: %d", worker2Count.Load())
	}

	sup.Stop()
}

// TestRestForOneStrategy tests RestForOne restart strategy
func TestRestForOneStrategy(t *testing.T) {
	var worker1Count, worker2Count, worker3Count atomic.Int32

	worker1 := func(ctx context.Context) error {
		worker1Count.Add(1)
		<-ctx.Done()
		return nil
	}

	worker2 := func(ctx context.Context) error {
		count := worker2Count.Add(1)
		if count < 3 {
			return errors.New("worker2 error")
		}
		<-ctx.Done()
		return nil
	}

	worker3 := func(ctx context.Context) error {
		worker3Count.Add(1)
		<-ctx.Done()
		return nil
	}

	sup := New(
		RestForOne,
		WithName("rest-for-one-test"),
		WithBackoff(ConstantBackoff(10*time.Millisecond)),
		WithChildren(
			ChildSpec{Name: "worker1", Start: worker1, Restart: Permanent},
			ChildSpec{Name: "worker2", Start: worker2, Restart: Permanent},
			ChildSpec{Name: "worker3", Start: worker3, Restart: Permanent},
		),
	)

	if err := sup.Start(); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Worker1 should start once (not affected)
	if worker1Count.Load() != 1 {
		t.Fatalf("worker1 should start once, count: %d", worker1Count.Load())
	}

	// Worker2 should restart multiple times
	if worker2Count.Load() < 2 {
		t.Fatalf("worker2 should restart, count: %d", worker2Count.Load())
	}

	// Worker3 should restart with worker2
	if worker3Count.Load() < 2 {
		t.Fatalf("worker3 should restart with worker2, count: %d", worker3Count.Load())
	}

	sup.Stop()
}

// ====================================================================
// backoff_test.go - Backoff Policy Tests
// ====================================================================

// TestExponentialBackoff tests exponential backoff calculation
func TestExponentialBackoff(t *testing.T) {
	policy := ExponentialBackoff(100*time.Millisecond, 5*time.Second)

	tests := []struct {
		restarts int
		min      time.Duration
		max      time.Duration
	}{
		{0, 90 * time.Millisecond, 110 * time.Millisecond},
		{1, 190 * time.Millisecond, 210 * time.Millisecond},
		{2, 390 * time.Millisecond, 410 * time.Millisecond},
		{10, 5 * time.Second, 5 * time.Second}, // Capped at max
	}

	for _, tt := range tests {
		delay := policy.ComputeDelay(tt.restarts)
		if delay < tt.min || delay > tt.max {
			t.Errorf("restarts=%d: expected delay between %v and %v, got %v",
				tt.restarts, tt.min, tt.max, delay)
		}
	}
}

// TestConstantBackoff tests constant backoff
func TestConstantBackoff(t *testing.T) {
	policy := ConstantBackoff(time.Second)

	for i := 0; i < 10; i++ {
		delay := policy.ComputeDelay(i)
		if delay != time.Second {
			t.Errorf("expected 1s, got %v", delay)
		}
	}
}

// TestLinearBackoff tests linear backoff
func TestLinearBackoff(t *testing.T) {
	policy := LinearBackoff(100*time.Millisecond, 200*time.Millisecond, 2*time.Second)

	tests := []struct {
		restarts int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 300 * time.Millisecond},
		{2, 500 * time.Millisecond},
		{10, 2 * time.Second}, // Capped
	}

	for _, tt := range tests {
		delay := policy.ComputeDelay(tt.restarts)
		if delay != tt.expected {
			t.Errorf("restarts=%d: expected %v, got %v", tt.restarts, tt.expected, delay)
		}
	}
}

// TestJitterBackoff tests jitter backoff
func TestJitterBackoff(t *testing.T) {
	base := ConstantBackoff(time.Second)
	policy := JitterBackoff(base, 0.2)

	for i := 0; i < 10; i++ {
		delay := policy.ComputeDelay(i)
		// Should be within 800ms to 1200ms (±20% of 1s)
		if delay < 800*time.Millisecond || delay > 1200*time.Millisecond {
			t.Errorf("delay %v outside expected range", delay)
		}
	}
}

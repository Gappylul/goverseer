package goverseer

import (
	"math"
	"math/rand"
	"time"
)

// BackoffPolicy calculates restart delays based on the number of restarts.
// This helps prevent resource exhaustion from rapid restart loops.
type BackoffPolicy interface {
	// ComputeDelay calculates the delay before the next restart attempt.
	// The restarts parameter indicates how many times this child has already restarted.
	ComputeDelay(restarts int) time.Duration
}

// exponentialBackoff implements exponential backoff with a maximum delay.
type exponentialBackoff struct {
	initial time.Duration
	max     time.Duration
}

// ExponentialBackoff creates a backoff policy that doubles the delay with each restart.
// The delay starts at initial and is capped at max.
//
// Example: ExponentialBackoff(100*time.Millisecond, 5*time.Second)
// - 1st restart: 100ms
// - 2nd restart: 200ms
// - 3rd restart: 400ms
// - 4th restart: 800ms
// - 5th restart: 1.6s
// - 6th+ restart: 5s (capped)
func ExponentialBackoff(initial, max time.Duration) BackoffPolicy {
	return &exponentialBackoff{initial: initial, max: max}
}

func (e *exponentialBackoff) ComputeDelay(restarts int) time.Duration {
	delay := time.Duration(float64(e.initial) * math.Pow(2, float64(restarts)))
	if delay > e.max {
		delay = e.max
	}
	return delay
}

// constantBackoff implements a constant delay between restarts.
type constantBackoff struct {
	delay time.Duration
}

// ConstantBackoff creates a backoff policy with a fixed delay between restarts.
// This is useful when you want predictable restart timing.
//
// Example: ConstantBackoff(time.Second)
// - All restarts wait 1 second
func ConstantBackoff(delay time.Duration) BackoffPolicy {
	return &constantBackoff{delay: delay}
}

func (c *constantBackoff) ComputeDelay(restarts int) time.Duration {
	return c.delay
}

// linearBackoff implements linear backoff with a maximum delay.
type linearBackoff struct {
	initial   time.Duration
	increment time.Duration
	max       time.Duration
}

// LinearBackoff creates a backoff policy that increases linearly with each restart.
// The delay starts at initial and increases by increment for each restart, capped at max.
//
// Example: LinearBackoff(100*time.Millisecond, 500*time.Millisecond, 10*time.Second)
// - 1st restart: 100ms
// - 2nd restart: 600ms
// - 3rd restart: 1.1s
// - 4th restart: 1.6s
// - etc., capped at 10s
func LinearBackoff(initial, increment, max time.Duration) BackoffPolicy {
	return &linearBackoff{initial: initial, increment: increment, max: max}
}

func (l *linearBackoff) ComputeDelay(restarts int) time.Duration {
	delay := l.initial + time.Duration(restarts)*l.increment
	if delay > l.max {
		delay = l.max
	}
	return delay
}

// jitterBackoff wraps another backoff policy and adds randomness.
type jitterBackoff struct {
	base   BackoffPolicy
	factor float64
}

// JitterBackoff wraps another backoff policy and adds random jitter.
// This helps prevent thundering herd problems when multiple processes restart simultaneously.
//
// The factor determines the amount of jitter: 0.0 means no jitter, 1.0 means up to 100% jitter.
// The jitter is applied symmetrically (can increase or decrease the delay).
//
// Example: JitterBackoff(ExponentialBackoff(1*time.Second, 10*time.Second), 0.2)
// - A 1s delay becomes 0.8s-1.2s (±20%)
func JitterBackoff(base BackoffPolicy, factor float64) BackoffPolicy {
	if factor < 0 {
		factor = 0
	}
	if factor > 1 {
		factor = 1
	}
	return &jitterBackoff{base: base, factor: factor}
}

func (j *jitterBackoff) ComputeDelay(restarts int) time.Duration {
	baseDelay := j.base.ComputeDelay(restarts)
	// Random jitter between -factor and +factor
	jitter := time.Duration(float64(baseDelay) * j.factor * (rand.Float64()*2 - 1))
	delay := baseDelay + jitter
	if delay < 0 {
		delay = 0
	}
	return delay
}

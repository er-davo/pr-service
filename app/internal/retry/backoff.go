package retry

import (
	"math"
	"math/rand/v2"
	"time"
)

// Backoff defines the interface for calculating delay between retry attempts.
type Backoff interface {
	// Next returns the duration to wait before the next attempt.
	// The attempt parameter is 0-based.
	Next(attempt int) time.Duration
}

// FixedBackoff provides a constant retry interval with optional jitter.
type FixedBackoff struct {
	Interval time.Duration // fixed interval between retries
	Jitter   float64       // optional jitter as a fraction [0,1)
}

// Next returns the next wait duration for FixedBackoff.
func (f FixedBackoff) Next(attempt int) time.Duration {
	return addJitter(time.Duration(f.Interval), f.Jitter)
}

// LinearBackoff increases the retry interval linearly with each attempt.
type LinearBackoff struct {
	Base   time.Duration // initial interval
	Step   time.Duration // added interval per attempt
	Max    time.Duration // maximum interval cap
	Jitter float64       // optional jitter
}

// Next returns the next wait duration for LinearBackoff.
func (l LinearBackoff) Next(attempt int) time.Duration {
	d := l.Base + time.Duration(attempt)*l.Step
	if l.Max > 0 && d > l.Max {
		return l.Max
	}
	return addJitter(time.Duration(d), l.Jitter)
}

// ExponentialBackoff increases the retry interval exponentially with each attempt.
type ExponentialBackoff struct {
	Base   time.Duration // initial interval
	Factor float64       // exponential growth factor
	Max    time.Duration // maximum interval cap
	Jitter float64       // optional jitter
}

// Next returns the next wait duration for ExponentialBackoff.
func (e ExponentialBackoff) Next(attempt int) time.Duration {
	d := float64(e.Base) * math.Pow(e.Factor, float64(attempt))
	if e.Max > 0 && d > float64(e.Max) {
		return e.Max
	}
	return addJitter(time.Duration(d), e.Jitter)
}

// addJitter applies random jitter to a duration. Jitter should be in [0,1).
func addJitter(d time.Duration, jitter float64) time.Duration {
	if jitter <= 0 || jitter >= 1 {
		return d
	}
	delta := (rand.Float64()*2 - 1) * jitter
	return time.Duration(float64(d) * (1 + delta))
}

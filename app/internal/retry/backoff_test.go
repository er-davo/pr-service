package retry

import (
	"testing"
	"time"
)

func inRange(t *testing.T, got, expected time.Duration, jitter float64) {
	min := float64(expected) * (1 - jitter)
	max := float64(expected) * (1 + jitter)
	if float64(got) < min || float64(got) > max {
		t.Errorf("got %v, expected within [%v, %v]", got, time.Duration(min), time.Duration(max))
	}
}

func TestFixedBackoff(t *testing.T) {
	t.Run("no jitter", func(t *testing.T) {
		b := FixedBackoff{Interval: time.Second, Jitter: 0}
		got := b.Next(5)
		if got != time.Second {
			t.Errorf("expected 1s, got %v", got)
		}
	})

	t.Run("with jitter", func(t *testing.T) {
		b := FixedBackoff{Interval: time.Second, Jitter: 0.2}
		got := b.Next(3)
		inRange(t, got, time.Second, 0.2)
	})
}

func TestLinearBackoff(t *testing.T) {
	t.Run("no jitter", func(t *testing.T) {
		b := LinearBackoff{Base: time.Second, Step: 500 * time.Millisecond, Max: 3 * time.Second, Jitter: 0}
		tests := []struct {
			attempt int
			want    time.Duration
		}{
			{0, 1 * time.Second},
			{1, 1500 * time.Millisecond},
			{2, 2 * time.Second},
			{10, 3 * time.Second}, // capped by Max
		}

		for _, tt := range tests {
			got := b.Next(tt.attempt)
			if got != tt.want {
				t.Errorf("attempt %d: expected %v, got %v", tt.attempt, tt.want, got)
			}
		}
	})

	t.Run("with jitter", func(t *testing.T) {
		b := LinearBackoff{Base: time.Second, Step: time.Second, Max: 5 * time.Second, Jitter: 0.1}
		got := b.Next(2) // expected 3s ±10%
		inRange(t, got, 3*time.Second, 0.1)
	})
}

func TestExponentialBackoff(t *testing.T) {
	t.Run("no jitter", func(t *testing.T) {
		b := ExponentialBackoff{Base: time.Second, Factor: 2, Max: 10 * time.Second, Jitter: 0}
		tests := []struct {
			attempt int
			want    time.Duration
		}{
			{0, 1 * time.Second},
			{1, 2 * time.Second},
			{2, 4 * time.Second},
			{3, 8 * time.Second},
			{4, 10 * time.Second}, // capped by Max
		}

		for _, tt := range tests {
			got := b.Next(tt.attempt)
			if got != tt.want {
				t.Errorf("attempt %d: expected %v, got %v", tt.attempt, tt.want, got)
			}
		}
	})

	t.Run("with jitter", func(t *testing.T) {
		b := ExponentialBackoff{Base: time.Second, Factor: 2, Max: 0, Jitter: 0.2}
		got := b.Next(2) // expected 4s ±20%
		inRange(t, got, 4*time.Second, 0.2)
	})
}

func TestAddJitter(t *testing.T) {
	t.Run("no jitter", func(t *testing.T) {
		got := addJitter(time.Second, 0)
		if got != time.Second {
			t.Errorf("expected 1s, got %v", got)
		}
	})

	t.Run("invalid jitter >=1", func(t *testing.T) {
		got := addJitter(time.Second, 1)
		if got != time.Second {
			t.Errorf("expected 1s, got %v", got)
		}
	})

	t.Run("valid jitter", func(t *testing.T) {
		got := addJitter(100*time.Millisecond, 0.5)
		if got < 50*time.Millisecond || got > 150*time.Millisecond {
			t.Errorf("expected within [50ms, 150ms], got %v", got)
		}
	})
}

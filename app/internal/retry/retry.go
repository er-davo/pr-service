package retry

import (
	"context"
	"fmt"
	"time"
)

// RetryOption configures a Retrier.
type RetryOption func(*retrier)

// AttemptFunc is the function executed on each retry attempt.
type AttemptFunc func() error

// IsRetryableFunc determines if an error should be retried.
type IsRetryableFunc func(error) bool

// Retrier executes an operation with retry logic.
type Retrier interface {
	// Do executes the attempt function with retry according to the retrier configuration.
	Do(ctx context.Context, f AttemptFunc) error
}

// retrier is the default implementation of Retrier.
type retrier struct {
	backoff     Backoff         // strategy for calculating delay between attempts
	maxAttempts int             // maximum number of attempts (0 = unlimited)
	isRetryable IsRetryableFunc // function to determine if an error is retryable
}

// New constructs a new Retrier with optional configurations.
func New(opts ...RetryOption) Retrier {
	r := &retrier{
		backoff:     defaultBackoff(),
		maxAttempts: defaultAttempts(),
		isRetryable: defaultIsRetryableFunc(),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// NoRetry returns a Retrier that performs no retries.
func NoRetry() Retrier {
	return New(
		WithMaxAttempts(1),
		WithBackoff(LinearBackoff{}),
	)
}

// Do executes the given AttemptFunc with retries according to the retrier's configuration.
// Returns nil if the attempt succeeds, or the last error if all retries fail.
func (r retrier) Do(ctx context.Context, f AttemptFunc) error {
	var err error

	for attempt := 0; r.maxAttempts == 0 || attempt < r.maxAttempts; attempt++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		if err = f(); err == nil {
			return nil
		}

		if r.isRetryable != nil && !r.isRetryable(err) {
			return fmt.Errorf("unretryable error: %w", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(r.backoff.Next(attempt)):
		}
	}

	return fmt.Errorf("all attempts failed: %w", err)
}

// defaultAttempts returns the default maximum number of retry attempts.
func defaultAttempts() int {
	return 3
}

// defaultBackoff returns a default LinearBackoff.
func defaultBackoff() Backoff {
	return LinearBackoff{
		Base:   time.Second,
		Step:   time.Second,
		Max:    10 * time.Second,
		Jitter: 0.1,
	}
}

// defaultIsRetryableFunc returns the default retryable function (retry any non-nil error).
func defaultIsRetryableFunc() IsRetryableFunc {
	return func(err error) bool {
		return err != nil
	}
}

// WithMaxAttempts sets the maximum number of retry attempts.
func WithMaxAttempts(maxAttempts int) RetryOption {
	return func(r *retrier) {
		r.maxAttempts = maxAttempts
	}
}

// WithBackoff sets a custom Backoff strategy.
func WithBackoff(backoff Backoff) RetryOption {
	return func(r *retrier) {
		r.backoff = backoff
	}
}

// WithIsRetryableFunc sets a custom function to determine if an error is retryable.
func WithIsRetryableFunc(isRetryable IsRetryableFunc) RetryOption {
	return func(r *retrier) {
		r.isRetryable = isRetryable
	}
}

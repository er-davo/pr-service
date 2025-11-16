package retry

import (
	"context"
)

// Do executes the given AttemptFunc with retries.
// - ctx controls cancellation and timeout.
// - maxAttempts = 0 -> retry indefinitely until ctx is done.
func Do(ctx context.Context, maxAttempts int, f AttemptFunc) error {
	return New(
		WithMaxAttempts(maxAttempts),
	).Do(ctx, f)
}

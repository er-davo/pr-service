package app

import (
	"errors"
	"pr-service/internal/config"
	"pr-service/internal/repository"
	"pr-service/internal/retry"
)

func newRepoRetrier(cfg config.Retry, retryableFunc retry.IsRetryableFunc) retry.Retrier {
	opts := []retry.RetryOption{
		retry.WithMaxAttempts(cfg.MaxAttempts),
	}

	if retryableFunc != nil {
		opts = append(opts, retry.WithIsRetryableFunc(retryableFunc))
	}

	if cfg.Backoff == "exponential" {
		opts = append(opts, retry.WithBackoff(retry.ExponentialBackoff{
			Base:   cfg.Base,
			Factor: cfg.Factor,
			Max:    cfg.Max,
			Jitter: cfg.Jitter,
		}))
	}

	return retry.New(opts...)
}

func isRetryableFunc(err error) bool {
	unretryableErrors := []error{
		repository.ErrDuplicate,
		repository.ErrNotFound,
		repository.ErrInvalidID,
		repository.ErrForeignKeyViolation,
		repository.ErrNotFound,
		repository.ErrTxAborted,
	}

	for _, unretryableErr := range unretryableErrors {
		if errors.Is(err, unretryableErr) {
			return false
		}
	}

	return true
}

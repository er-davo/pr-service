package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errAlwaysFail = errors.New("always fail")
	errCustom     = errors.New("custom error")
)

func TestRetrier_Do(t *testing.T) {
	tests := []struct {
		name       string
		opts       []RetryOption
		fn         func() error
		ctx        func() context.Context
		wantErr    error
		wantErrMsg string
		wantCalls  int
	}{
		{
			name: "success first try",
			opts: []RetryOption{WithMaxAttempts(3)},
			fn: func() error {
				return nil
			},
			ctx:       context.Background,
			wantErr:   nil,
			wantCalls: 1,
		},
		{
			name: "success after retries",
			opts: []RetryOption{WithMaxAttempts(5), WithBackoff(FixedBackoff{Interval: time.Millisecond})},
			fn: func() error {
				return errAlwaysFail // будем обрабатывать через счетчик внутри теста
			},
			ctx:       context.Background,
			wantErr:   nil,
			wantCalls: 3,
		},
		{
			name: "all attempts failed",
			opts: []RetryOption{WithMaxAttempts(3), WithBackoff(FixedBackoff{Interval: time.Millisecond})},
			fn: func() error {
				return errAlwaysFail
			},
			ctx:        context.Background,
			wantErrMsg: "all attempts failed: always fail",
			wantCalls:  3,
		},
		{
			name: "non-retryable error",
			opts: []RetryOption{
				WithMaxAttempts(5),
				WithIsRetryableFunc(func(err error) bool { return false }),
			},
			fn: func() error {
				return errCustom
			},
			ctx:        context.Background,
			wantErrMsg: "unretryable error: custom error",
			wantCalls:  1,
		},
		{
			name: "context canceled",
			opts: []RetryOption{WithMaxAttempts(5), WithBackoff(FixedBackoff{Interval: time.Millisecond})},
			fn: func() error {
				return errAlwaysFail
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: context.Canceled,
		},
		{
			name: "context timeout",
			opts: []RetryOption{WithMaxAttempts(10), WithBackoff(FixedBackoff{Interval: 20 * time.Millisecond})},
			fn: func() error {
				return errAlwaysFail
			},
			ctx: func() context.Context {
				ctx, _ := context.WithTimeout(context.Background(), 50*time.Millisecond)
				return ctx
			},
			wantErr: context.DeadlineExceeded,
		},
		{
			name: "infinite attempts until success",
			opts: []RetryOption{WithMaxAttempts(0), WithBackoff(FixedBackoff{Interval: time.Millisecond})},
			fn: func() error {
				return errAlwaysFail
			},
			ctx:       context.Background,
			wantErr:   nil,
			wantCalls: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			wrappedFn := func() error {
				calls++
				switch tt.name {
				case "success after retries":
					if calls < 3 {
						return errAlwaysFail
					}
					return nil
				case "infinite attempts until success":
					if calls < 5 {
						return errAlwaysFail
					}
					return nil
				default:
					return tt.fn()
				}
			}

			ctx := tt.ctx()
			r := New(tt.opts...)
			err := r.Do(ctx, wrappedFn)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr, "unexpected error")
			} else if tt.wantErrMsg != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErrMsg)
			} else {
				assert.NoError(t, err)
			}

			if tt.wantCalls > 0 {
				assert.Equal(t, tt.wantCalls, calls)
			}
		})
	}
}

package internal

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

type TemporaryError interface {
	error
	Temporary() bool
}

type Retrier struct {
	Cooldown CooldownFactory
}

// Do repeatedly invokes the provided run function while the context remains
// active. It waits in between invocations of the provided functions by
// delegating to the provided Cooldown function
func (r Retrier) Do(ctx context.Context, run func() error) error {
	cooldown := r.Cooldown()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := run()
		if err != nil {
			// check to see if it's temporary
			var tempErr TemporaryError
			if ok := errors.As(err, &tempErr); ok && tempErr.Temporary() {
				err := cooldown()
				if err != nil {
					return fmt.Errorf("sleeping during retry: %w", err)
				}
				continue
			}

			// since it's not temporary, it can't be retried, so...
			return err
		}
		return nil
	}
}

// CooldownFunc is a function that will block when called. It is intended for
// use with retry logic.
type CooldownFunc func() error

// CooldownFactory is a function that returns CooldownFuncs. It helps
// CooldownFuncs dispose of any accumulated state so that they function
// correctly upon successive uses.
type CooldownFactory func() CooldownFunc

func Max(limit int, factory CooldownFactory) CooldownFactory {
	return func() CooldownFunc {
		cooldown := factory()
		count := 0
		return func() error {
			if count >= limit {
				return fmt.Errorf("maximum attempts reached (%d)", limit)
			}

			err := cooldown()
			if err != nil {
				return err
			}
			count++
			return nil
		}
	}
}

// AsFastAsPossible is a Cooldown strategy that does not block, allowing retry
// logic to proceed as fast as possible. This is particularly useful in tests
func AsFastAsPossible() CooldownFactory {
	return func() CooldownFunc {
		return func() error {
			return nil
		}
	}
}

func Exponential(interval time.Duration, base int) CooldownFactory {
	return func() CooldownFunc {
		count := 0
		return func() error {
			increment := math.Pow(float64(base), float64(count))
			delay := interval.Nanoseconds() * int64(increment)
			time.Sleep(time.Duration(delay))
			count++
			return nil
		}
	}
}

func Fixed(interval time.Duration) CooldownFactory {
	return func() CooldownFunc {
		return func() error {
			time.Sleep(interval)
			return nil
		}
	}
}

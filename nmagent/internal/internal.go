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
	Cooldown func() error
}

// Do repeatedly invokes the provided run function while the context remains
// active. It waits in between invocations of the provided functions by
// delegating to the provided Cooldown function
func (r Retrier) Do(ctx context.Context, run func() error) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := run()
		if err != nil {
			// check to see if it's temporary
			var tempErr TemporaryError
			if ok := errors.As(err, &tempErr); ok && tempErr.Temporary() {
				err := r.Cooldown()
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

func Max(limit int, f func() error) func() error {
	count := 0
	return func() error {
		if count >= limit {
			return fmt.Errorf("maximum attempts reached (%d)", limit)
		}

		err := f()
		if err != nil {
			return err
		}
		count++
		return nil
	}
}

func AsFastAsPossible() error { return nil }

func Exponential(interval time.Duration, base time.Duration) func() error {
	count := 0
	return func() error {
		increment := math.Pow(float64(base.Nanoseconds()), float64(count))
		delay := interval.Nanoseconds() * int64(increment)
		time.Sleep(time.Duration(delay))
		count++
		return nil
	}
}

func Fixed(interval time.Duration) func() error {
	return func() error {
		time.Sleep(interval)
		return nil
	}
}

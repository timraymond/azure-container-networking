package internal

import (
	"context"
	"errors"
)

type TemporaryError interface {
	error
	Temporary() bool
}

// BackoffRetry implements cancellable exponential backoff of some arbitrary
// function
func BackoffRetry(ctx context.Context, run func() error) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := run()
		if err != nil {
			// check to see if it's temporary
			var tempErr TemporaryError
			if ok := errors.As(err, &tempErr); ok && tempErr.Temporary() {
				continue
			}

			// since it's not temporary, it can't be retried, so...
			return err
		}
		return nil
	}
}

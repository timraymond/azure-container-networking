package internal

import (
	"context"
	"errors"
	"testing"
)

type TestError struct{}

func (t TestError) Error() string {
	return "oh no!"
}

func (t TestError) Temporary() bool {
	return true
}

func TestBackoffRetry(t *testing.T) {
	got := 0
	exp := 10

	ctx := context.Background()

	err := BackoffRetry(ctx, func() error {
		if got < exp {
			got++
			return TestError{}
		}
		return nil
	})

	if err != nil {
		t.Fatal("unexpected error: err:", err)
	}

	if got < exp {
		t.Error("unexpected number of invocations: got:", got, "exp:", exp)
	}
}

func TestBackoffRetryWithCancel(t *testing.T) {
	got := 0
	exp := 5
	total := 10

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := BackoffRetry(ctx, func() error {
		got++
		if got >= exp {
			cancel()
		}

		if got < total {
			return TestError{}
		}
		return nil
	})

	if err == nil {
		t.Error("expected context cancellation error, but received none")
	}

	if got != exp {
		t.Error("unexpected number of iterations: exp:", exp, "got:", got)
	}
}

func TestBackoffRetryUnretriableError(t *testing.T) {
	err := BackoffRetry(context.Background(), func() error {
		return errors.New("boom")
	})

	if err == nil {
		t.Fatal("expected an error, but none was returned")
	}
}

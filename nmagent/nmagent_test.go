package nmagent_test

import (
	"dnc/nmagent"
	"net/http"
	"testing"
	"time"
)

func TestErrorTemp(t *testing.T) {
	errorTests := []struct {
		name       string
		err        nmagent.Error
		shouldTemp bool
	}{
		{
			"regular",
			nmagent.Error{
				Code: http.StatusInternalServerError,
			},
			false,
		},
		{
			"processing",
			nmagent.Error{
				Code: http.StatusProcessing,
			},
			true,
		},
		{
			"unauthorized temporary",
			nmagent.Error{
				Runtime: 30 * time.Second,
				Limit:   1 * time.Minute,
				Code:    http.StatusUnauthorized,
			},
			true,
		},
		{
			"unauthorized permanent",
			nmagent.Error{
				Runtime: 2 * time.Minute,
				Limit:   1 * time.Minute,
				Code:    http.StatusUnauthorized,
			},
			false,
		},
		{
			"unauthorized zero values",
			nmagent.Error{
				Code: http.StatusUnauthorized,
			},
			false,
		},
		{
			"unauthorized zero limit",
			nmagent.Error{
				Runtime: 2 * time.Minute,
				Code:    http.StatusUnauthorized,
			},
			false,
		},
	}

	for _, test := range errorTests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if test.err.Temporary() && !test.shouldTemp {
				t.Fatal("test was temporary and not expected to be")
			}

			if !test.err.Temporary() && test.shouldTemp {
				t.Fatal("test was not temporary but expected to be")
			}
		})
	}
}

package nmagent_test

import (
	"dnc/nmagent"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPolicyMarshal(t *testing.T) {
	policyTests := []struct {
		name   string
		policy nmagent.Policy
		exp    string
	}{
		{
			"basic",
			nmagent.Policy{
				ID:   "policyID1",
				Type: "type1",
			},
			"\"policyID1, type1\"",
		},
	}

	for _, test := range policyTests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := json.Marshal(test.policy)
			if err != nil {
				t.Fatal("unexpected err marshaling policy: err", err)
			}

			if string(got) != test.exp {
				t.Errorf("marshaled policy does not match expectation: got: %q: exp: %q", string(got), test.exp)
			}

			var enc nmagent.Policy
			err = json.Unmarshal(got, &enc)
			if err != nil {
				t.Fatal("unexpected error unmarshaling: err:", err)
			}

			if !cmp.Equal(enc, test.policy) {
				t.Error("re-encoded policy differs from expectation: diff:", cmp.Diff(enc, test.policy))
			}
		})
	}
}

func TestDeleteContainerRequestValidation(t *testing.T) {
	dcrTests := []struct {
		name          string
		req           nmagent.DeleteContainerRequest
		shouldBeValid bool
	}{
		{
			"empty",
			nmagent.DeleteContainerRequest{},
			false,
		},
		{
			"missing ncid",
			nmagent.DeleteContainerRequest{
				PrimaryAddress:      "10.0.0.1",
				AuthenticationToken: "swordfish",
			},
			false,
		},
		{
			"missing primary address",
			nmagent.DeleteContainerRequest{
				NCID:                "00000000-0000-0000-0000-000000000000",
				AuthenticationToken: "swordfish",
			},
			false,
		},
		{
			"missing auth token",
			nmagent.DeleteContainerRequest{
				NCID:           "00000000-0000-0000-0000-000000000000",
				PrimaryAddress: "10.0.0.1",
			},
			false,
		},
	}

	for _, test := range dcrTests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.req.Validate()
			if err != nil && test.shouldBeValid {
				t.Fatal("unexpected validation errors: err:", err)
			}

			if err == nil && !test.shouldBeValid {
				t.Fatal("expected request to be invalid but wasn't")
			}
		})
	}
}

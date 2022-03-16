package nmagent_test

import (
	"context"
	"dnc/nmagent"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var _ http.RoundTripper = &TestTripper{}

// TestTripper is a RoundTripper with a customizeable RoundTrip method for
// testing purposes
type TestTripper struct {
	RoundTripF func(*http.Request) (*http.Response, error)
}

func (t *TestTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.RoundTripF(req)
}

func TestNMAgentClientJoinNetwork(t *testing.T) {
	joinNetTests := []struct {
		name       string
		id         string
		exp        string
		respStatus int
		shouldErr  bool
	}{
		{
			"happy path",
			"00000000-0000-0000-0000-000000000000",
			"/machine/plugins/?comp=nmagent&type=NetworkManagement/joinedVirtualNetworks/00000000-0000-0000-0000-000000000000/api-version/1",
			http.StatusOK,
			false,
		},
		{
			"empty network ID",
			"",
			"",
			http.StatusOK, // this shouldn't be checked
			true,
		},
		{
			"malformed UUID",
			"00000000-0000",
			"",
			http.StatusOK, // this shouldn't be checked
			true,
		},
		{
			"internal error",
			"00000000-0000-0000-0000-000000000000",
			"/machine/plugins/?comp=nmagent&type=NetworkManagement/joinedVirtualNetworks/00000000-0000-0000-0000-000000000000/api-version/1",
			http.StatusInternalServerError,
			true,
		},
	}

	for _, test := range joinNetTests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// create a client
			var got string
			client := nmagent.Client{
				HTTPClient: &http.Client{
					Transport: &TestTripper{
						RoundTripF: func(req *http.Request) (*http.Response, error) {
							got = req.URL.Path
							rr := httptest.NewRecorder()
							rr.WriteHeader(test.respStatus)
							return rr.Result(), nil
						},
					},
				},
			}

			// if the test provides a timeout, use it in the context
			var ctx context.Context
			if deadline, ok := t.Deadline(); ok {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(context.Background(), deadline)
				defer cancel()
			} else {
				ctx = context.Background()
			}

			// attempt to join network
			// TODO(timraymond): need a more realistic network ID, I think
			err := client.JoinNetwork(ctx, test.id)
			if err != nil && !test.shouldErr {
				t.Fatal("unexpected error: err:", err)
			}

			if err == nil && test.shouldErr {
				t.Fatal("expected error but received none")
			}

			if got != test.exp {
				t.Error("received URL differs from expectation: got", got, "exp:", test.exp)
			}
		})
	}
}

func TestNMAgentClientJoinNetworkRetry(t *testing.T) {
	// we want to ensure that the client will automatically follow up with
	// NMAgent, so we want to track the number of requests that it makes
	invocations := 0
	exp := 10

	client := nmagent.Client{
		HTTPClient: &http.Client{
			Transport: &TestTripper{
				RoundTripF: func(req *http.Request) (*http.Response, error) {
					rr := httptest.NewRecorder()
					if invocations < exp {
						rr.WriteHeader(http.StatusProcessing)
						invocations++
					} else {
						rr.WriteHeader(http.StatusOK)
					}
					return rr.Result(), nil
				},
			},
		},
	}

	// if the test provides a timeout, use it in the context
	var ctx context.Context
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(context.Background(), deadline)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	// attempt to join network
	err := client.JoinNetwork(ctx, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatal("unexpected error: err:", err)
	}

	if invocations != exp {
		t.Error("client did not make the expected number of API calls: got:", invocations, "exp:", exp)
	}
}

func TestNMAgentGetNetworkConfig(t *testing.T) {
	getTests := []struct {
		name       string
		vnetID     string
		expURL     string
		expVNet    nmagent.VirtualNetwork
		shouldCall bool
		shouldErr  bool
	}{
		{
			"happy path",
			"00000000-0000-0000-0000-000000000000",
			"/machine/plugins/?comp=nmagent&type=NetworkManagement/joinedVirtualNetworks/00000000-0000-0000-0000-000000000000/api-version/1",
			nmagent.VirtualNetwork{
				CNetSpace:      "10.10.1.0/24",
				DefaultGateway: "10.10.0.1",
				DNSServers: []string{
					"1.1.1.1",
					"1.0.0.1",
				},
				Subnets:     []nmagent.Subnet{},
				VNetSpace:   "10.0.0.0/8",
				VNetVersion: "2018", // TODO(timraymond): what's a real version look like?
			},
			true,
			false,
		},
	}

	for _, test := range getTests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var got string
			client := &nmagent.Client{
				HTTPClient: &http.Client{
					Transport: &TestTripper{
						RoundTripF: func(req *http.Request) (*http.Response, error) {
							rr := httptest.NewRecorder()
							got = req.URL.Path
							rr.WriteHeader(http.StatusOK)
							err := json.NewEncoder(rr).Encode(&test.expVNet)
							if err != nil {
								return nil, fmt.Errorf("encoding response: %w", err)
							}

							return rr.Result(), nil
						},
					},
				},
			}

			// if the test provides a timeout, use it in the context
			var ctx context.Context
			if deadline, ok := t.Deadline(); ok {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(context.Background(), deadline)
				defer cancel()
			} else {
				ctx = context.Background()
			}

			gotVNet, err := client.GetNetworkConfiguration(ctx, test.vnetID)
			if err != nil && !test.shouldErr {
				t.Fatal("unexpected error: err:", err)
			}

			if err == nil && test.shouldErr {
				t.Fatal("expected error but received none")
			}

			if got != test.expURL && test.shouldCall {
				t.Error("unexpected URL: got:", got, "exp:", test.expURL)
			}

			if !cmp.Equal(gotVNet, test.expVNet) {
				t.Error("received vnet differs from expected: diff:", cmp.Diff(gotVNet, test.expVNet))
			}
		})
	}
}

func TestNMAgentGetNetworkConfigRetry(t *testing.T) {
	t.Parallel()

	count := 0
	exp := 10
	client := &nmagent.Client{
		HTTPClient: &http.Client{
			Transport: &TestTripper{
				RoundTripF: func(req *http.Request) (*http.Response, error) {
					rr := httptest.NewRecorder()
					if count < exp {
						rr.WriteHeader(http.StatusProcessing)
						count++
					} else {
						rr.WriteHeader(http.StatusOK)
					}

					// we still need a fake response
					var out nmagent.VirtualNetwork
					err := json.NewEncoder(rr).Encode(&out)
					if err != nil {
						return nil, err
					}

					return rr.Result(), nil
				},
			},
		},
	}

	// if the test provides a timeout, use it in the context
	var ctx context.Context
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(context.Background(), deadline)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	_, err := client.GetNetworkConfiguration(ctx, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatal("unexpected error: err:", err)
	}

	if count != exp {
		t.Error("unexpected number of API calls: exp:", exp, "got:", count)
	}
}

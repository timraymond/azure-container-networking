package nmagent_test

import (
	"context"
	"dnc/nmagent"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
			client := nmagent.NewTestClient(&TestTripper{
				RoundTripF: func(req *http.Request) (*http.Response, error) {
					got = req.URL.Path
					rr := httptest.NewRecorder()
					rr.Write([]byte(fmt.Sprintf(`{"httpStatusCode":"%d"}`, test.respStatus)))
					rr.WriteHeader(http.StatusOK)
					return rr.Result(), nil
				},
			})

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
			err := client.JoinNetwork(ctx, nmagent.JoinNetworkRequest{test.id})
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

	client := nmagent.NewTestClient(&TestTripper{
		RoundTripF: func(req *http.Request) (*http.Response, error) {
			rr := httptest.NewRecorder()
			if invocations < exp {
				rr.WriteHeader(http.StatusProcessing)
				invocations++
			} else {
				rr.WriteHeader(http.StatusOK)
			}
			rr.Write([]byte(`{"httpStatusCode": "200"}`))
			return rr.Result(), nil
		},
	})

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
	err := client.JoinNetwork(ctx, nmagent.JoinNetworkRequest{"00000000-0000-0000-0000-000000000000"})
	if err != nil {
		t.Fatal("unexpected error: err:", err)
	}

	if invocations != exp {
		t.Error("client did not make the expected number of API calls: got:", invocations, "exp:", exp)
	}
}

// TODO(timraymond): this is super repetitive (see the retry test)
func TestNMAgentClientJoinNetworkUnauthorized(t *testing.T) {
	t.Parallel()

	// we want to ensure that the client will automatically follow up with
	// NMAgent, so we want to track the number of requests that it makes
	invocations := 0
	exp := 10

	client := nmagent.NewTestClient(&TestTripper{
		RoundTripF: func(req *http.Request) (*http.Response, error) {
			rr := httptest.NewRecorder()
			if invocations < exp {
				rr.WriteHeader(http.StatusUnauthorized)
				invocations++
			} else {
				rr.WriteHeader(http.StatusOK)
			}
			rr.Write([]byte(`{"httpStatusCode": "200"}`))
			return rr.Result(), nil
		},
	})

	client.UnauthorizedGracePeriod = 1 * time.Minute

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
	err := client.JoinNetwork(ctx, nmagent.JoinNetworkRequest{"00000000-0000-0000-0000-000000000000"})
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
		expVNet    map[string]interface{}
		shouldCall bool
		shouldErr  bool
	}{
		{
			"happy path",
			"00000000-0000-0000-0000-000000000000",
			"/machine/plugins/?comp=nmagent&type=NetworkManagement/joinedVirtualNetworks/00000000-0000-0000-0000-000000000000/api-version/1",
			map[string]interface{}{
				"httpStatusCode": "200",
				"cnetSpace":      "10.10.1.0/24",
				"defaultGateway": "10.10.0.1",
				"dnsServers": []string{
					"1.1.1.1",
					"1.0.0.1",
				},
				"subnets":     []map[string]interface{}{},
				"vnetSpace":   "10.0.0.0/8",
				"vnetVersion": "12345",
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
			client := nmagent.NewTestClient(&TestTripper{
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
			})

			// if the test provides a timeout, use it in the context
			var ctx context.Context
			if deadline, ok := t.Deadline(); ok {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(context.Background(), deadline)
				defer cancel()
			} else {
				ctx = context.Background()
			}

			gotVNet, err := client.GetNetworkConfiguration(ctx, nmagent.GetNetworkConfigRequest{test.vnetID})
			if err != nil && !test.shouldErr {
				t.Fatal("unexpected error: err:", err)
			}

			if err == nil && test.shouldErr {
				t.Fatal("expected error but received none")
			}

			if got != test.expURL && test.shouldCall {
				t.Error("unexpected URL: got:", got, "exp:", test.expURL)
			}

			// TODO(timraymond): this is ugly
			expVnet := nmagent.VirtualNetwork{
				CNetSpace:      test.expVNet["cnetSpace"].(string),
				DefaultGateway: test.expVNet["defaultGateway"].(string),
				DNSServers:     test.expVNet["dnsServers"].([]string),
				Subnets:        []nmagent.Subnet{},
				VNetSpace:      test.expVNet["vnetSpace"].(string),
				VNetVersion:    test.expVNet["vnetVersion"].(string),
			}
			if !cmp.Equal(gotVNet, expVnet) {
				t.Error("received vnet differs from expected: diff:", cmp.Diff(gotVNet, expVnet))
			}
		})
	}
}

func TestNMAgentGetNetworkConfigRetry(t *testing.T) {
	t.Parallel()

	count := 0
	exp := 10
	client := nmagent.NewTestClient(&TestTripper{
		RoundTripF: func(req *http.Request) (*http.Response, error) {
			rr := httptest.NewRecorder()
			if count < exp {
				rr.WriteHeader(http.StatusProcessing)
				count++
			} else {
				rr.WriteHeader(http.StatusOK)
			}

			// we still need a fake response
			rr.Write([]byte(`{"httpStatusCode": "200"}`))
			return rr.Result(), nil
		},
	})

	// if the test provides a timeout, use it in the context
	var ctx context.Context
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(context.Background(), deadline)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	_, err := client.GetNetworkConfiguration(ctx, nmagent.GetNetworkConfigRequest{"00000000-0000-0000-0000-000000000000"})
	if err != nil {
		t.Fatal("unexpected error: err:", err)
	}

	if count != exp {
		t.Error("unexpected number of API calls: exp:", exp, "got:", count)
	}
}

func TestNMAgentGetNetworkConfigUnauthorized(t *testing.T) {
	t.Parallel()

	count := 0
	exp := 10
	client := nmagent.NewTestClient(&TestTripper{
		RoundTripF: func(req *http.Request) (*http.Response, error) {
			rr := httptest.NewRecorder()
			if count < exp {
				rr.WriteHeader(http.StatusUnauthorized)
				count++
			} else {
				rr.WriteHeader(http.StatusOK)
			}

			// we still need a fake response
			rr.Write([]byte(`{"httpStatusCode": "200"}`))

			return rr.Result(), nil
		},
	})

	client.UnauthorizedGracePeriod = 1 * time.Minute

	// if the test provides a timeout, use it in the context
	var ctx context.Context
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(context.Background(), deadline)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	_, err := client.GetNetworkConfiguration(ctx, nmagent.GetNetworkConfigRequest{"00000000-0000-0000-0000-000000000000"})
	if err != nil {
		t.Fatal("unexpected error: err:", err)
	}

	if count != exp {
		t.Error("unexpected number of API calls: exp:", exp, "got:", count)
	}
}

func TestNMAgentPutNetworkContainer(t *testing.T) {
	putNCTests := []struct {
		name       string
		req        nmagent.PutNetworkContainerRequest
		shouldCall bool
		shouldErr  bool
	}{
		{
			"happy path",
			nmagent.PutNetworkContainerRequest{
				ID:         "350f1e3c-4283-4f51-83a1-c44253962ef1",
				Version:    uint64(12345),
				VNetID:     "be3a33e-61e3-42c7-bd23-6b949f57bd36",
				SubnetName: "TestSubnet",
				IPv4Addrs:  []string{"10.0.0.43"},
				Policies: []nmagent.Policy{
					{
						ID:   "policyID1",
						Type: "type1",
					},
					{
						ID:   "policyID2",
						Type: "type2",
					},
				},
				VlanID:              1234,
				AuthenticationToken: "swordfish",
				PrimaryAddress:      "10.0.0.1",
				VirtualNetworkID:    "0000000-0000-0000-0000-000000000000",
			},
			true,
			false,
		},
	}

	for _, test := range putNCTests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			didCall := false
			client := nmagent.NewTestClient(&TestTripper{
				RoundTripF: func(req *http.Request) (*http.Response, error) {
					rr := httptest.NewRecorder()
					rr.Write([]byte(`{"httpStatusCode": "200"}`))
					rr.WriteHeader(http.StatusOK)
					didCall = true
					return rr.Result(), nil
				},
			})

			err := client.PutNetworkContainer(context.TODO(), test.req)
			if err != nil && !test.shouldErr {
				t.Fatal("unexpected error: err", err)
			}

			if err == nil && test.shouldErr {
				t.Fatal("expected error but received none")
			}

			if test.shouldCall && !didCall {
				t.Fatal("expected call but received none")
			}

			if !test.shouldCall && didCall {
				t.Fatal("unexpected call. expected no call ")
			}
		})
	}
}

func TestNMAgentDeleteNC(t *testing.T) {
	deleteTests := []struct {
		name      string
		req       nmagent.DeleteContainerRequest
		exp       string
		shouldErr bool
	}{
		{
			"happy path",
			nmagent.DeleteContainerRequest{
				NCID:                "00000000-0000-0000-0000-000000000000",
				PrimaryAddress:      "10.0.0.1",
				AuthenticationToken: "swordfish",
			},
			"/machine/plugins/?comp=nmagent&type=NetworkManagement/interfaces/10.0.0.1/networkContainers/00000000-0000-0000-0000-000000000000/authenticationToken/swordfish/api-version/1/method/DELETE",
			false,
		},
	}

	var got string
	for _, test := range deleteTests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			client := nmagent.NewTestClient(&TestTripper{
				RoundTripF: func(req *http.Request) (*http.Response, error) {
					got = req.URL.Path
					rr := httptest.NewRecorder()
					rr.Write([]byte(`{"httpStatusCode": "200"}`))
					return rr.Result(), nil
				},
			})

			err := client.DeleteNetworkContainer(context.TODO(), test.req)
			if err != nil && !test.shouldErr {
				t.Fatal("unexpected error: err:", err)
			}

			if err == nil && test.shouldErr {
				t.Fatal("expected error but received none")
			}

			if test.exp != got {
				t.Errorf("received URL differs from expectation:\n\texp: %q:\n\tgot: %q", test.exp, got)
			}
		})
	}
}

package wireserver_test

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Azure/azure-container-networking/cns/wireserver"
)

var _ http.RoundTripper = &TestTripper{}

// TestTripper is a mock implementation of a round tripper that allows clients
// to substitute their own implementation, so that HTTP requests can be
// asserted against and stub responses can be generated.
type TestTripper struct {
	RoundTripF func(*http.Request) (*http.Response, error)
}

func (t *TestTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.RoundTripF(req)
}

type NOPLogger struct{}

func (m *NOPLogger) Printf(_ string, _ ...any) {}

func TestGetInterfaces(t *testing.T) {
	// create a wireserver client using a test tripper so that it can be asserted
	// that the correct requests are sent.
	expURL := "http://168.63.129.16/machine/plugins?comp=nmagent&type=getinterfaceinfov1"
	var reqURL string
	client := &wireserver.Client{
		Logger: &NOPLogger{},
		HTTPClient: &http.Client{
			Transport: &TestTripper{
				RoundTripF: func(req *http.Request) (*http.Response, error) {
					reqURL = req.URL.String()
					rr := httptest.NewRecorder()
					resp := wireserver.GetInterfacesResult{}
					err := xml.NewEncoder(rr).Encode(&resp)
					if err != nil {
						t.Fatal("unexpected error encoding mock wireserver response: err:", err)
					}

					return rr.Result(), nil
				},
			},
		},
	}

	// invoke the endpoint on Wireserver
	_, err := client.GetInterfaces(context.TODO())
	if err != nil {
		t.Fatal("unexpected error invoking GetInterfaces: err:", err)
	}

	if expURL != reqURL {
		t.Error("received request URL to wireserve does not match expectation:\n\texp:", expURL, "\n\tgot:", reqURL)
	}
}

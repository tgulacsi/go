// Copyright 2025 Tamás Gulácsi. All rights reserved.

package httpcircuitbreaker

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/sony/gobreaker/v2"
)

type BreakerSettings struct{ gobreaker.Settings }

func NewBreakerSettings(name string, bucketPeriod, timeout time.Duration) BreakerSettings {
	return BreakerSettings{gobreaker.Settings{Name: name, BucketPeriod: bucketPeriod, Timeout: timeout}}
}
func (st BreakerSettings) IsZero() bool { return st.Name == "" }

func NewHTTPClient(settings BreakerSettings, client *http.Client) (*http.Client, BreakerMonitor) {
	if client == nil {
		cl := *http.DefaultClient
		client = &cl
	}
	tr, _ := client.Transport.(*http.Transport)
	trcb := TransportWithCircuitBreaker(settings, tr)
	client.Transport = trcb
	return client, BreakerMonitor{breakerStater: trcb.breaker}
}

func TransportWithCircuitBreaker(
	settings BreakerSettings, rt http.RoundTripper,
) breakingTransport {
	if settings.IsSuccessful == nil {
		settings.IsSuccessful = func(err error) bool {
			var ue *url.Error
			return errors.As(err, &ue)
		}
	}
	if rt == nil {
		rt = http.DefaultTransport
	}
	return breakingTransport{
		RoundTripper: rt,
		breaker:      gobreaker.NewCircuitBreaker[*http.Response](settings.Settings),
	}
}

var (
	_ http.RoundTripper = breakingTransport{}
	_ breakerStater     = breakingTransport{}
)

type breakingTransport struct {
	http.RoundTripper
	breaker *gobreaker.CircuitBreaker[*http.Response]
}

func (btr breakingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return btr.breaker.Execute(func() (*http.Response, error) {
		return btr.RoundTripper.RoundTrip(req)
	})
}
func (btr breakingTransport) State() gobreaker.State {
	return btr.breaker.State()
}

type (
	breakerStater  interface{ State() gobreaker.State }
	BreakerMonitor struct{ breakerStater }
)

func (bm BreakerMonitor) IsOpen() bool {
	if bm.breakerStater == nil {
		return false
	}
	return bm.breakerStater.State() == gobreaker.StateOpen
}
func NewBreakerMonitor(bs breakerStater) BreakerMonitor { return BreakerMonitor{breakerStater: bs} }

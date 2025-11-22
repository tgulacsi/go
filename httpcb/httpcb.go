// Copyright 2025 Tamás Gulácsi. All rights reserved.

package httpcircuitbreaker

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/sony/gobreaker/v2"
)

type Settings struct{ gobreaker.Settings }

func NewBreakerSettings(name string, bucketPeriod, timeout time.Duration) Settings {
	return Settings{gobreaker.Settings{Name: name, BucketPeriod: bucketPeriod, Timeout: timeout}}
}
func (st Settings) IsZero() bool { return st.Name == "" }

func NewHTTPClient(settings Settings, client *http.Client) (*http.Client, Monitor) {
	if client == nil {
		cl := *http.DefaultClient
		client = &cl
	}
	tr, _ := client.Transport.(*http.Transport)
	trcb := TransportWithCircuitBreaker(settings, tr)
	client.Transport = trcb
	return client, Monitor{Stater: trcb.breaker}
}

func TransportWithCircuitBreaker(
	settings Settings, rt http.RoundTripper,
) Transport {
	if settings.IsSuccessful == nil {
		settings.IsSuccessful = func(err error) bool {
			var ue *url.Error
			return errors.As(err, &ue)
		}
	}
	if rt == nil {
		rt = http.DefaultTransport
	}
	return Transport{
		RoundTripper: rt,
		breaker:      gobreaker.NewCircuitBreaker[*http.Response](settings.Settings),
	}
}

var (
	_ http.RoundTripper = Transport{}
	_ Stater            = Transport{}
)

type Transport struct {
	http.RoundTripper
	breaker *gobreaker.CircuitBreaker[*http.Response]
}

func (btr Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	return btr.breaker.Execute(func() (*http.Response, error) {
		return btr.RoundTripper.RoundTrip(req)
	})
}
func (btr Transport) State() gobreaker.State {
	return btr.breaker.State()
}

type (
	Stater  interface{ State() gobreaker.State }
	Monitor struct{ Stater }
)

func (bm Monitor) IsOpen() bool {
	if bm.Stater == nil {
		return false
	}
	return bm.Stater.State() == gobreaker.StateOpen
}
func NewBreakerMonitor(bs Stater) Monitor { return Monitor{Stater: bs} }

// Copyright 2025 Tamás Gulácsi. All rights reserved.

package httpcb

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/sony/gobreaker/v2"
)

type Settings struct {
	gobreaker.Settings
	*slog.Logger
}

func NewSettings(name string, bucketPeriod, timeout time.Duration, logger *slog.Logger) Settings {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return Settings{
		Logger: logger,
		Settings: gobreaker.Settings{
			Name:         name,
			BucketPeriod: bucketPeriod,
			Timeout:      timeout,
		},
	}
}
func (st Settings) IsZero() bool { return st.Name == "" }

func NewHTTPClient(settings Settings, client *http.Client) (*http.Client, Monitor) {
	if client == nil {
		cl := *http.DefaultClient
		client = &cl
	}
	trcb := NewTransport(settings, client.Transport)
	client.Transport = trcb
	return client, Monitor{Stater: trcb.breaker}
}

func NewTransport(
	settings Settings, rt http.RoundTripper,
) Transport {
	if settings.IsSuccessful == nil {
		settings.IsSuccessful = func(err error) bool {
			if err == nil {
				return true
			}
			var ue *url.Error
			return !errors.As(err, &ue)
		}
	}
	if settings.OnStateChange == nil && settings.Logger != nil {
		settings.OnStateChange = func(name string, from, to gobreaker.State) {
			settings.Logger.Warn("breaker changed state", "name", name, "from", from, "to", to)
		}
	}
	if rt == nil {
		rt = http.DefaultTransport
	}
	_ = rt.RoundTrip // panic on nil
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
func NewMonitor(bs Stater) Monitor { return Monitor{Stater: bs} }

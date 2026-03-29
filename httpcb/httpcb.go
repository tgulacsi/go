// Copyright 2025, 2026 Tamás Gulácsi. All rights reserved.

package httpcb

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-json-experiment/json/jsontext"
	"github.com/sony/gobreaker/v2"
)

type Settings struct {
	gobreaker.Settings
	Logger *slog.Logger
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

func (s Settings) MarshalJSONTo(enc *jsontext.Encoder) error {
	enc.WriteToken(jsontext.BeginObject)
	if s.Name != "" {
		enc.WriteToken(jsontext.String("Name"))
		enc.WriteToken(jsontext.String(s.Name))
	}
	if s.MaxRequests != 0 {
		enc.WriteToken(jsontext.String("MaxRequests"))
		enc.WriteToken(jsontext.Uint(uint64(s.MaxRequests)))
	}
	D := func(k string, v time.Duration) {
		if v == 0 {
			return
		}
		enc.WriteToken(jsontext.String(k))
		enc.WriteToken(jsontext.String(v.String()))
	}
	D("Interval", s.Interval)
	D("BucketPeriod", s.BucketPeriod)
	D("Timeout", s.Timeout)
	return enc.WriteToken(jsontext.EndObject)
}
func (s Settings) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	err := s.MarshalJSONTo(jsontext.NewEncoder(&buf))
	return buf.Bytes(), err
}

func (s *Settings) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	if tok.Kind() != jsontext.BeginObject.Kind() {
		return fmt.Errorf("wanted {, got %v", tok)
	}
	D := func(dest *time.Duration, v jsontext.Value) error {
		if len(v) == 0 {
			*dest = 0
			return nil
		}
		if v.Kind() == jsontext.KindNumber { // jsonv1 nanoseconds
			if u, err := strconv.ParseUint(string(v), 10, 64); err != nil {
				return err
			} else {
				*dest = time.Duration(u)
				return nil
			}
		}
		b := bytes.Trim(v, `"`)
		if len(b) == 0 {
			*dest = 0
			return nil
		}
		if b[0] == 'P' { // ISO8601 Period
			*dest, err = ParseDurationISO8601(b)
			return err
		}
		*dest, err = time.ParseDuration(string(b))
		return err
	}

	for {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}
		if tok.Kind() == jsontext.KindEndObject {
			break
		}
		if tok.Kind() != jsontext.KindString {
			return fmt.Errorf("wanted string, got %v", tok)
		}
		k := tok.String()
		switch k {
		case "Name":
			v, err := dec.ReadToken()
			if err != nil {
				return err
			}
			s.Name = v.String()
		case "MaxRequests":
			v, err := dec.ReadToken()
			if err != nil {
				return err
			}
			s.MaxRequests = uint32(v.Uint())
		default:
			v, err := dec.ReadValue()
			if err != nil {
				return err
			}
			switch k {
			case "Interval":
				err = D(&s.Interval, v)
			case "BucketPeriod":
				err = D(&s.BucketPeriod, v)
			case "Timeout":
				err = D(&s.Timeout, v)
			}
		}
		if err != nil {
			return err
		}
	}

	return nil
}
func (s *Settings) UnmarshalJSON(p []byte) error {
	return s.UnmarshalJSONFrom(jsontext.NewDecoder(bytes.NewReader(p)))
}

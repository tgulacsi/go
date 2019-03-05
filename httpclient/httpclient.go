/*
  Copyright 2019 Tamás Gulácsi

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/

// Package httpclient provides a retrying circuit-breaked http.Client.
package httpclient

import (
	"context"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/pkg/errors"
	"github.com/sony/gobreaker"
)

const (
	DefaultTimeout      = 10 * time.Second
	DefaultInterval     = 10 * time.Minute
	DefaultFailureRatio = 0.6
)

// New returns a *retryablehttp.Client, with the default http.Client, DefaultTimeout, DefaultInterval and DefaultFailureRatio.
func New(name string) *retryablehttp.Client {
	return NewWithClient(name, nil, DefaultTimeout, DefaultInterval, DefaultFailureRatio)
}

// NewWithClient returns a *retryablehttp.Client based on the given *http.Client.
// The accompanying circuit breaker is set with the given timeout and interval.
func NewWithClient(name string, cl *http.Client, timeout, interval time.Duration, failureRatio float64) *retryablehttp.Client {
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	if interval == 0 {
		interval = DefaultInterval
	}
	if failureRatio <= 0 {
		failureRatio = DefaultFailureRatio
	}
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:     name,
		Interval: interval,
		Timeout:  timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < 3 {
				return true
			}
			return float64(counts.TotalFailures)/float64(counts.Requests) <= failureRatio
		},
	})
	rc := retryablehttp.NewClient()
	if cl != nil {
		rc.HTTPClient = cl
	}
	rc.RetryWaitMin = timeout / 2
	rc.RetryWaitMax = interval
	rc.RetryMax = 10
	// CheckRetry specifies a policy for handling retries.
	// It is called following each request with the response and error values returned by the http.Client.
	// If CheckRetry returns false, the Client stops retrying and returns the response to the caller.
	// If CheckRetry returns an error, that error value is returned in lieu of the error from the request.
	// The Client will close any response body when retrying, but if the retry is aborted it is up to the CheckResponse callback
	// to properly close any response body before returning.
	rc.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		if cerr := errors.Cause(err); cerr == gobreaker.ErrOpenState || cerr == gobreaker.ErrTooManyRequests {
			return true, nil
		}
		return false, err
	}
	if true {
		{
			cl := *rc.HTTPClient
			cl.Transport = TransportWithBreaker{Tripper: cl.Transport, Breaker: GoBreaker{CircuitBreaker: cb}}
			rc.HTTPClient = &cl
		}
	}
	return rc
}

// NewRequest calls github.com/hashicorp/go-retryablehttp's NewRequest.
func NewRequest(method, URL string, body interface{}) (*retryablehttp.Request, error) {
	return retryablehttp.NewRequest(method, URL, body)
}

// Breaker is the interface for a circuit breaker.
type Breaker interface {
	// Execute runs the given request if the circuit breaker is closed or half-open states.
	// An error is instantly returned when the circuit breaker is tripped.
	Execute(fn func() (interface{}, error)) (interface{}, error)

	// Opened reports whether the breaker is opened at the moment.
	Opened() bool
}

// TransportWithBreaker shrink-wraps a http.RoundTripper with a circuit Breaker.
type TransportWithBreaker struct {
	Tripper http.RoundTripper
	Breaker Breaker
}

const waitCount = 10

// RoundTrip sends the request and returns the response, waiting the circuit breaker to be in a closed state.
func (t TransportWithBreaker) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := r.Context().Err(); err != nil {
		return nil, err
	}
	dl, ok := r.Context().Deadline()
	if ok && !dl.IsZero() {
		dl = dl.Add(-(time.Until(dl) >> 3)) // 12.5% margin
	} else {
		dl = time.Now().Add((waitCount*(waitCount) - 1) / 2)
	}
	for i := 0; i < waitCount; i++ {
		if !t.Breaker.Opened() {
			break
		}
		sleep := time.Duration(i) * time.Second
		if u := time.Until(dl); u <= 0 {
			break
		} else if u < sleep {
			sleep = u
		}
		time.Sleep(sleep)
	}

	res, err := t.Breaker.Execute(func() (interface{}, error) {
		return t.Tripper.RoundTrip(r)
	})

	if resp, ok := res.(*http.Response); ok {
		return resp, err
	}

	return nil, err
}

var _ = Breaker(GoBreaker{})

// GoBreaker adapts a *gobreaker.CircuitBreaker to the Breaker interface.
type GoBreaker struct {
	*gobreaker.CircuitBreaker
}

// Closed reports whether the circuit breaker is in opened state.
func (b GoBreaker) Opened() bool { return b.CircuitBreaker.State() == gobreaker.StateOpen }

/*
  Copyright 2019, 2022 Tamás Gulácsi

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

package soaphlp

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/kylewolfe/soaptrip"
	bp "github.com/tgulacsi/go/bufpool"
)

// DefaultLog is the logging function in use.
var DefaultLog = func(...interface{}) error { return nil }

var ErrBodyNotFound = errors.New("body not found")

// Caller is the client interface.
type Caller interface {
	Call(ctx context.Context, w io.Writer, method string, body io.Reader) (*xml.Decoder, error)
}

// WithLog returns the context with the "Log" value set to the given Log.
func WithLog(ctx context.Context, Log func(...interface{}) error) context.Context {
	return context.WithValue(ctx, logKey, Log)
}

type contextKey string

const logKey = contextKey("Log")

// NewClient returns a new client for the given endpoint.
func NewClient(endpointURL, soapActionBase string, cl *http.Client) Caller {
	if cl == nil {
		cl = http.DefaultClient
	}
	if cl.Transport == nil {
		cl.Transport = http.DefaultTransport
	}
	cl.Transport = soaptrip.New(cl.Transport)
	return &soapClient{
		Client:         cl,
		URL:            endpointURL,
		SOAPActionBase: soapActionBase,
		bufpool:        bp.New(1024),
	}
}

type soapClient struct {
	bufpool bp.Pool
	*http.Client
	URL            string
	SOAPActionBase string
}

var bufpool = bp.New(1024)

func FindBody(w io.Writer, r io.Reader) (*xml.Decoder, error) {
	buf := bufpool.Get()
	defer bufpool.Put(buf)

	d := xml.NewDecoder(io.TeeReader(r, buf))
	var n int
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				if buf.Len() == 0 {
					return nil, err
				}
				break
			}
			return nil, fmt.Errorf("%s: %w", buf.String(), err)
		}
		n++
		switch x := tok.(type) {
		case xml.StartElement:
			if x.Name.Local == "Body" &&
				(x.Name.Space == "" ||
					x.Name.Space == "http://schemas.xmlsoap.org/soap/envelope/" ||
					x.Name.Space == "http://www.w3.org/2003/05/soap-envelope") {
				start := d.InputOffset()
				if err = d.Skip(); err != nil {
					return nil, fmt.Errorf("%s: %w", buf.String(), err)
				}
				end := d.InputOffset()
				//Log("start", start, "end", end, "bytes", start, end, buf.Len())
				if _, err = w.Write(buf.Bytes()[start:end]); err != nil {
					return nil, err
				}
				// Must copy the bytes to a new slice to allow buf reuse!
				d := xml.NewDecoder(bytes.NewReader(append(
					make([]byte, 0, buf.Len()),
					buf.Bytes()...)))
				// Restart from the beginning, and consume n tokens (till the Skipped).
				for i := 0; i < n; i++ {
					d.Token()
				}
				return d, nil
			}
		}
	}
	return nil, fmt.Errorf("%s: %w", buf.String(), ErrBodyNotFound)
}

func (s soapClient) Call(ctx context.Context, w io.Writer, method string, body io.Reader) (*xml.Decoder, error) {
	if s.SOAPActionBase != "" {
		method = s.SOAPActionBase + "/" + method
	}
	return s.CallAction(ctx, w, method, body)
}
func (s soapClient) CallAction(ctx context.Context, w io.Writer, soapAction string, body io.Reader) (*xml.Decoder, error) {
	rc, err := s.CallActionRaw(ctx, soapAction, body)
	if err != nil {
		if rc != nil {
			rc.Close()
		}
		return nil, err
	}
	return FindBody(w, rc)
}
func (s soapClient) CallActionRaw(ctx context.Context, soapAction string, body io.Reader) (io.ReadCloser, error) {
	buf := s.bufpool.Get()
	defer s.bufpool.Put(buf)
	buf.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<Envelope xmlns="http://schemas.xmlsoap.org/soap/envelope/">
  <Body xmlns="http://schemas.xmlsoap.org/soap/envelope/">
`)
	_, err := io.Copy(buf, body)
	buf.WriteString("\n</Body></Envelope>")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", s.URL, bytes.NewReader(buf.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.URL, err)
	}
	req.Header.Set("Content-Length", strconv.Itoa(buf.Len()))
	req.Header.Set("SOAPAction", soapAction)
	req.Header.Set("Content-Type", "text/xml")
	Log := GetLog(ctx)
	Log("msg", "calling", "url", s.URL, "soapAction", soapAction, "body", buf.String())
	resp, err := s.Client.Do(req.WithContext(ctx))
	if err != nil {
		if resp != nil && resp.Body != nil {
			defer resp.Body.Close()
		}
		if urlErr, ok := err.(*url.Error); ok {
			if fault, ok := urlErr.Err.(*soaptrip.SoapFault); ok {
				return nil, &Fault{SoapFault: *fault}
			}
		}
		return nil, err
	}
	if resp.StatusCode > 299 {
		err := errors.New(resp.Status)
		b, _ := io.ReadAll(resp.Body)
		if len(b) == 0 {
			return nil, err
		}
		return nil, fmt.Errorf("%s: %w", string(b), err)
	}
	return resp.Body, nil
}

// GetLog returns the Log function from the Context.
func GetLog(ctx context.Context) func(keyvalue ...interface{}) error {
	if Log, _ := ctx.Value(logKey).(func(...interface{}) error); Log != nil {
		return Log
	}
	return DefaultLog
}

type Fault struct {
	soaptrip.SoapFault
}

func (f *Fault) Error() string            { return f.SoapFault.Error() }
func (f *Fault) FaultCode() string        { return f.SoapFault.FaultCode }
func (f *Fault) FaultString() string      { return f.SoapFault.FaultString }
func (f *Fault) Response() *http.Response { return f.SoapFault.Response }

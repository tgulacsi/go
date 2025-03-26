/*
  Copyright 2019, 2025 Tamás Gulácsi

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
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/UNO-SOFT/zlog/v2"
	bp "github.com/tgulacsi/go/bufpool"
	"github.com/valyala/quicktemplate"
)

var ErrBodyNotFound = errors.New("body not found")

// Caller is the client interface.
type Caller interface {
	Call(ctx context.Context, w io.Writer, method string, body io.Reader) (*xml.Decoder, error)
}

// WithLogger returns the context with the "Logger" value set to the given Log.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return zlog.NewSContext(ctx, logger)
}

// NewClient returns a new client for the given endpoint.
func NewClient(endpointURL, soapActionBase string, cl *http.Client) Caller {
	if cl == nil {
		cl = http.DefaultClient
	}
	if cl.Transport == nil {
		cl.Transport = http.DefaultTransport
	}
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

type envelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Header  string   `xml:",innerxml"`
}

func FindBody(w io.Writer, r io.Reader) (header string, d *xml.Decoder, err error) {
	buf := bufpool.Get()
	defer bufpool.Put(buf)

	d = xml.NewDecoder(io.TeeReader(r, buf))
	var env envelope
	var n int
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				if buf.Len() == 0 {
					return env.Header, nil, err
				}
				break
			}
			return env.Header, nil, fmt.Errorf("%s: %w", buf.String(), err)
		}
		n++
		switch x := tok.(type) {
		case xml.StartElement:
			if x.Name.Local == "Header" &&
				(x.Name.Space == "" ||
					x.Name.Space == "http://schemas.xmlsoap.org/soap/envelope/" ||
					x.Name.Space == "http://www.w3.org/2003/05/soap-envelope") {
				d.DecodeElement(&env, &x)
			} else if x.Name.Local == "Body" &&
				(x.Name.Space == "" ||
					x.Name.Space == "http://schemas.xmlsoap.org/soap/envelope/" ||
					x.Name.Space == "http://www.w3.org/2003/05/soap-envelope") {
				start := d.InputOffset()
				if err = d.Skip(); err != nil {
					return env.Header, nil, fmt.Errorf("%s: %w", buf.String(), err)
				}
				end := d.InputOffset()
				//Log("start", start, "end", end, "bytes", start, end, buf.Len())
				if _, err = w.Write(buf.Bytes()[start:end]); err != nil {
					return env.Header, nil, err
				}
				// Must copy the bytes to a new slice to allow buf reuse!
				d := xml.NewDecoder(bytes.NewReader(append(
					make([]byte, 0, buf.Len()),
					buf.Bytes()...)))
				// Restart from the beginning, and consume n tokens (till the Skipped).
				for i := 0; i < n; i++ {
					d.Token()
				}
				return env.Header, d, nil
			}
		}
	}
	return env.Header, nil, fmt.Errorf("%s: %w", buf.String(), ErrBodyNotFound)
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
	_, dec, err := FindBody(w, rc)
	return dec, err
}
func (s soapClient) CallActionRaw(ctx context.Context, soapAction string, body io.Reader) (io.ReadCloser, error) {
	buf := s.bufpool.Get()
	defer s.bufpool.Put(buf)
	buf.WriteString(xml.Header)
	buf.WriteString(`<Envelope xmlns="http://schemas.xmlsoap.org/soap/envelope/">
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
	logger := GetLogger(ctx)
	resp, err := s.Client.Do(req.WithContext(ctx))
	if err != nil {
		logger.Error("Do", "url", req.URL, "body", buf.String(), "error", err)
		if resp != nil && resp.Body != nil {
			defer resp.Body.Close()
		}

		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			if fault, ok := urlErr.Err.(*Fault); ok {
				return nil, fault
			}
		}
		return nil, err
	}
	if resp.StatusCode > 299 {
		logger.Error("Do", "url", req.URL, "status", resp.Status, "body", buf.String(), "error", err)
		err := errors.New(resp.Status)
		b, _ := io.ReadAll(resp.Body)
		if len(b) == 0 {
			return nil, err
		}
		return nil, fmt.Errorf("%s: %w", string(b), err)
	}
	if logger.Enabled(ctx, slog.LevelDebug) {
		logger.Debug("calling", "url", s.URL, "soapAction", soapAction, "body", buf.String())
	}
	return resp.Body, nil
}

// GetLogger returns the Log function from the Context.
func GetLogger(ctx context.Context) *slog.Logger {
	return zlog.SFromContext(ctx)
}

type Fault struct {
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Fault"`

	Code   string `xml:"faultcode,omitempty"`
	Reason string `xml:"faultstring,omitempty"`
	Actor  string `xml:"faultactor,omitempty"`
	Detail string `xml:"detail,omitempty"`

	Response *http.Response `xml:"-"`
}

func (f Fault) StreamXML(W *quicktemplate.Writer) {
	xml.NewEncoder(W.N()).Encode(f)
}

func (f Fault) WriteResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(struct {
		XMLName xml.Name `xml:"Envelope"`
		Fault   Fault
	}{Fault: f})
}

func Error(w http.ResponseWriter, err error) {
	Fault{Code: err.Error()}.WriteResponse(w)
}
func FaultFromError(err error) Fault {
	return Fault{Code: err.Error()}
}

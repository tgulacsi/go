package soaphlp

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/kylewolfe/soaptrip"
	"github.com/pkg/errors"
)

// Log is the logging function in use.
var Log = func(...interface{}) error { return nil }

var ErrBodyNotFound = errors.New("body not found")

// Caller is the client interface.
type Caller interface {
	Call(ctx context.Context, method string, body io.Reader) (*xml.Decoder, io.Closer, error)
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
	}
}

type soapClient struct {
	*http.Client
	URL            string
	SOAPActionBase string
}

var bufPool = sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 0, 1024)) }}

func FindBody(r io.Reader) (*xml.Decoder, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buf)
	buf.Reset()
	W := struct {
		io.Writer
	}{buf}
	d := xml.NewDecoder(io.TeeReader(r, W))
	for {
		tok, err := d.Token()
		if err != nil {
			return d, errors.Wrap(err, buf.String())
		}
		switch x := tok.(type) {
		case xml.StartElement:
			if (x.Name.Space == "" || x.Name.Space == "http://schemas.xmlsoap.org/soap/envelope/") && x.Name.Local == "Body" {
				W.Writer = ioutil.Discard // do not cache anymore
				return d, nil
			}
		}
	}
	return d, errors.Wrap(ErrBodyNotFound, buf.String())
}

func (s soapClient) Call(ctx context.Context, method string, body io.Reader) (*xml.Decoder, io.Closer, error) {
	if s.SOAPActionBase != "" {
		method = s.SOAPActionBase + "/" + method
	}
	return s.CallAction(ctx, method, body)
}
func (s soapClient) CallAction(ctx context.Context, soapAction string, body io.Reader) (*xml.Decoder, io.Closer, error) {
	rc, err := s.CallActionRaw(ctx, soapAction, body)
	if err != nil {
		if rc != nil {
			rc.Close()
		}
		return nil, nil, err
	}
	d, err := FindBody(rc)
	return d, rc, err
}
func (s soapClient) CallActionRaw(ctx context.Context, soapAction string, body io.Reader) (io.ReadCloser, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buf)
	buf.Reset()
	_, err := io.Copy(buf, io.MultiReader(
		strings.NewReader(`<?xml version="1.0" encoding="utf-8"?>
<Envelope xmlns="http://schemas.xmlsoap.org/soap/envelope/">
  <Body xmlns="http://schemas.xmlsoap.org/soap/envelope/">
`),
		body,
		strings.NewReader("\n</Body></Envelope>")))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", s.URL, bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Length", strconv.Itoa(buf.Len()))
	req.Header.Set("SOAPAction", soapAction)
	Log := GetLog(ctx)
	Log("msg", "calling", "url", s.URL, "soapAction", soapAction, "body", buf.String())
	resp, err := s.Client.Do(req.WithContext(ctx))
	if err != nil {
		if resp != nil && resp.Body != nil {
			defer resp.Body.Close()
		}
		if urlErr, ok := err.(*url.Error); ok {
			if fault, ok := urlErr.Err.(*soaptrip.SoapFault); ok {
				b, _ := ioutil.ReadAll(fault.Response.Body)
				return nil, errors.Wrapf(err, "%v: %v\n%s", fault.FaultCode, fault.FaultString, b)
			}
		}
		return nil, err
	}
	if resp.StatusCode > 299 {
		b, _ := ioutil.ReadAll(resp.Body)
		return nil, errors.Wrap(errors.New(resp.Status), string(b))
	}
	return resp.Body, nil
}

// GetLog returns the Log function from the Context.
func GetLog(ctx context.Context) func(keyvalue ...interface{}) error {
	if Log, _ := ctx.Value(logKey).(func(...interface{}) error); Log != nil {
		return Log
	}
	return Log
}

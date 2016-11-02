package soaphlp

import (
	"bytes"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"

	"github.com/kylewolfe/soaptrip"
	"github.com/pkg/errors"
)

// Log is the logging function in use.
var Log = func(...interface{}) error { return nil }

// Caller is the client interface.
type Caller interface {
	Call(ctx context.Context, method string, body io.Reader) (*xml.Decoder, io.Closer, error)
}

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

func (s soapClient) Call(ctx context.Context, method string, body io.Reader) (*xml.Decoder, io.Closer, error) {
	if s.SOAPActionBase != "" {
		method = s.SOAPActionBase + "/" + method
	}
	return s.CallAction(ctx, method, body)
}
func (s soapClient) CallAction(ctx context.Context, soapAction string, body io.Reader) (*xml.Decoder, io.Closer, error) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, io.MultiReader(
		strings.NewReader(`<?xml version="1.0" encoding="utf-8"?>
<Envelope xmlns="http://schemas.xmlsoap.org/soap/envelope/">
  <Body xmlns="http://schemas.xmlsoap.org/soap/envelope/">
`),
		body,
		strings.NewReader("\n</Body></Envelope>")))
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequest("POST", s.URL, bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Length", strconv.Itoa(buf.Len()))
	req.Header.Set("SOAPAction", soapAction)
	Log := GetLog(ctx)
	Log("msg", "calling", "url", s.URL, "soapAction", soapAction, "body", buf.String())
	resp, err := ctxhttp.Do(ctx, s.Client, req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		if urlErr, ok := err.(*url.Error); ok {
			if fault, ok := urlErr.Err.(*soaptrip.SoapFault); ok {
				b, _ := ioutil.ReadAll(fault.Response.Body)
				return nil, ioutil.NopCloser(bytes.NewReader(b)), errors.Wrapf(err, "%v: %v\n%s", fault.FaultCode, fault.FaultString, b)
			}
		}
		return nil, nil, err
	}

	d := xml.NewDecoder(resp.Body)
	for {
		tok, err := d.Token()
		if err != nil {
			return d, resp.Body, err
		}
		switch x := tok.(type) {
		case xml.StartElement:
			if x.Name.Space == "http://schemas.xmlsoap.org/soap/envelope/" && x.Name.Local == "Body" {
				return d, resp.Body, nil
			}
		}
	}
	return d, resp.Body, io.EOF
}

// GetLog returns the Log function from the Context.
func GetLog(ctx context.Context) func(keyvalue ...interface{}) error {
	if Log, _ := ctx.Value("Log").(func(...interface{}) error); Log != nil {
		return Log
	}
	return Log
}

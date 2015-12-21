package soaphlp

import (
	"bytes"
	"encoding/xml"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"gopkg.in/errgo.v1"

	"github.com/kylewolfe/soaptrip"
)

func NewClient(endpointURL, soapActionBase string, cl *http.Client) *soapClient {
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

func (s soapClient) Call(method string, body io.Reader) (*xml.Decoder, io.Closer, error) {
	if s.SOAPActionBase != "" {
		method = s.SOAPActionBase + "/" + method
	}
	return s.CallAction(method, body)
}
func (s soapClient) CallAction(soapAction string, body io.Reader) (*xml.Decoder, io.Closer, error) {
	var buf bytes.Buffer
	io.WriteString(&buf, `<?xml version="1.0" encoding="utf-8"?>
<Envelope xmlns="http://schemas.xmlsoap.org/soap/envelope/">
  <Body xmlns="http://schemas.xmlsoap.org/soap/envelope/">
`)
	io.Copy(&buf, body)
	io.WriteString(&buf, `
</Body></Envelope>`)

	req, err := http.NewRequest("POST", s.URL, bytes.NewReader(buf.Bytes()))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Length", strconv.Itoa(buf.Len()))
	req.Header.Set("SOAPAction", soapAction)
	log.Printf("calling %q (%q)\n%s", s.URL, soapAction, buf.String())
	resp, err := s.Client.Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		if urlErr, ok := err.(*url.Error); ok {
			if fault, ok := urlErr.Err.(*soaptrip.SoapFault); ok {
				b, _ := ioutil.ReadAll(fault.Response.Body)
				return nil, ioutil.NopCloser(bytes.NewReader(b)), errgo.Notef(err, "%v: %v\n%s", fault.FaultCode, fault.FaultString, b)
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

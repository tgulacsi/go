// Copyright 2015 Kyle Wolfe.

// Copied from https://github.com/kylewolfe/soaptrip/blame/master/soaptrip.go
package soaphlp

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// NewTranspport returns a new SoapRoundTripper from an existing http.RoundTripper
func NewTranspport(rt http.RoundTripper) http.RoundTripper {
	return &RoundTripper{rt}
}

// SoapRoundTripper is a wrapper for an existing http.RoundTripper
type RoundTripper struct {
	rt http.RoundTripper
}

// RoundTrip will call the original http.RoundTripper. Upon an error of the original RoundTripper it will return,
// otherwise it will copy the response body and attempt to parse it for a fault. If one is found a SoapFault
// will be returned as the error.
func (st *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// call original round tripper
	resp, err := st.rt.RoundTrip(req)

	// return on error
	if err != nil {
		return nil, err
	}

	// parse resp for soap faults
	err = ParseFault(resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (sf Fault) Error() string {
	return fmt.Sprintf("Code: '%s' String: '%s' Actor: '%s' Detail: '%s'", sf.Code, sf.Reason, sf.Actor, sf.Detail)
}

// ParseFault attempts to parse a Soap Fault from an http.Response. If a fault is found, it will return an error
// of type SoapFault, otherwise it will return nil
func ParseFault(resp *http.Response) error {
	var buf bytes.Buffer
	d := xml.NewDecoder(io.TeeReader(resp.Body, &buf))

	var start xml.StartElement
	fault := &Fault{Response: resp}
	var found bool
	depth := 0

	// iterate through the tokens
	for {
		tok, _ := d.Token()
		if tok == nil {
			break
		}

		// switch on token type
		switch t := tok.(type) {
		case xml.StartElement:
			start = t.Copy()
			depth++
			if depth > 2 { // don't descend beyond Envelope>Body>Fault
				break
			}
		case xml.EndElement:
			start = xml.StartElement{}
			depth--
		case xml.CharData:
			// https://www.techtarget.com/whatis/definition/SOAP-fault
			// fault was found, capture the values and mark as found
			switch strings.ToLower(start.Name.Local) {
			case "faultcode", "Code":
				found = true
				fault.Code = string(t)
			case "faultstring", "Reason":
				found = true
				fault.Reason = string(t)
			case "faultactor", "Role":
				found = true
				fault.Actor = string(t)
			case "detail", "Detail":
				found = true
				fault.Detail = string(t)
			}
		}
	}

	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(buf.Bytes()), resp.Body), resp.Body}

	if found {
		fault.Response = resp
		return fault
	}

	return nil
}

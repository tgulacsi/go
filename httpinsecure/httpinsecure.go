package httpinsecure

import (
	"crypto/tls"
	"net/http"

	"golang.org/x/net/http2"
)

var InsecureTransport = http.DefaultTransport.(*http.Transport).Clone()

func init() {
	tlsc := InsecureTransport.TLSClientConfig
	if tlsc == nil {
		tlsc = &tls.Config{InsecureSkipVerify: true} //nolint:gas
	} else {
		tlsc = tlsc.Clone()
		tlsc.InsecureSkipVerify = true //nolint:gas
	}
	http2.ConfigureTransport(InsecureTransport)
}

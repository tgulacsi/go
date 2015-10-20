/*
copyright 2015 tamás gulácsi

licensed under the apache license, version 2.0 (the "license");
you may not use this file except in compliance with the license.
you may obtain a copy of the license at

     http://www.apache.org/licenses/license-2.0

unless required by applicable law or agreed to in writing, software
distributed under the license is distributed on an "as is" basis,
without warranties or conditions of any kind, either express or implied.
see the license for the specific language governing permissions and
limitations under the license.
*/

// package handler provides some support structs for http.servehttp.
package handler

import (
	"net/http"

	"gopkg.in/inconshreveable/log15.v2"
)

var Log = log15.New("lib", "handler")

func init() {
	Log.SetHandler(log15.DiscardHandler())
}

type Handler func(w http.ResponseWriter, r *http.Request) error

type StatusError struct {
	Code int
	Err  error
}

func (se StatusError) Error() string {
	return se.Err.Error()
}

func (se StatusError) Status() int {
	return se.Code
}

type statuser interface {
	Status() int
}

type ErrHandler struct {
	Handler
}

// ServeHTTP allows our Handler type to satisfy http.Handler.
func (h ErrHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := h.Handler(w, r)
	if err == nil {
		return
	}
	if se, ok := err.(statuser); ok {
		// We can retrieve the status here and write out a specific
		// HTTP status code.
		Log.Error("HTTP error", "status", se.Status(), "error", err)
		http.Error(w, err.Error(), se.Status())
		return
	}
	// Any error types we don't specifically look out for default
	// to serving a HTTP 500
	Log.Error("HTTP", "error", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError),
		http.StatusInternalServerError)
}

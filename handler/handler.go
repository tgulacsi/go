/*
Copyright 2015 Tamás Gulácsi

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

// Package handler provides some support structs for http.ServeHTTP.
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

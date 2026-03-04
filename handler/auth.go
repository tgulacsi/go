// Copyright 2026 Tamás Gulácsi.
//
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"crypto/subtle"
	"errors"
	"net/http"
)

var ErrAuth = errors.New("authentication error")

// BasicAuth checks whether the given username and password equals with the declared.
func BasicAuth(username, password string, hndl http.Handler) http.Handler {
	bU, bP := []byte(username), []byte(password)
	return AuthHandler(func(r *http.Request) error {
		if u, p, ok := r.BasicAuth(); ok &&
			subtle.ConstantTimeCompare([]byte(u), bU) == 1 &&
			subtle.ConstantTimeCompare([]byte(p), bP) == 1 {
			return nil
		}
		return ErrAuth
	},
		hndl)
}

// AuthHandler calls check before serving hndl, and errors with the returned error.
func AuthHandler(check func(*http.Request) error, hndl http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := check(r); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		hndl.ServeHTTP(w, r)
	})
}

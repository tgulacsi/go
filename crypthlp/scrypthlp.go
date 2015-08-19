// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package crypthlp

import (
	"crypto/rand"
	"log"
	"sync"
	"time"

	"golang.org/x/crypto/scrypt"
)

var (
	nRatio   map[time.Duration]int
	nRatioMu sync.Mutex
)

// Key derives a key from the password, using scrypt.
// It tries to create the strongest key within the given time window.
func Key(password []byte, saltLen, keyLen int, timeout time.Duration,
) (salt []byte, key []byte, err error) {
	salt = make([]byte, saltLen)
	if _, err = rand.Read(salt); err != nil {
		return nil, nil, err
	}
	N := 16384
	nRatioMu.Lock()
	defer nRatioMu.Unlock()
	if nRatio == nil {
		nRatio = make(map[time.Duration]int)
	} else if n, ok := nRatio[timeout]; ok {
		N = n
	}
	for now := time.Now(); time.Since(now) < timeout; N <<= 1 {
		if key, err = scrypt.Key(password, salt, N, 8, 1, keyLen); err != nil {
			return
		}
	}
	nRatio[timeout] = N
	log.Printf("nRatio=%v", nRatio)
	return
}

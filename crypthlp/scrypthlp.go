// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package crypthlp

import (
	"crypto/rand"
	"encoding/json"
	"sync"
	"time"

	"golang.org/x/crypto/scrypt"
)

var (
	nRatio   map[time.Duration]int
	nRatioMu sync.Mutex
)

// Salt creates a new random salt with the given length.
func Salt(saltLen int) ([]byte, error) {
	salt := make([]byte, saltLen)
	_, err := rand.Read(salt)
	return salt, err
}

// GenKey derives a key from the password, using scrypt.
// It tries to create the strongest key within the given time window.
func GenKey(password []byte, saltLen, keyLen int, timeout time.Duration,
) (Key, error) {
	salt, err := Salt(saltLen)
	if err != nil {
		return Key{}, err
	}
	N := 16384 >> 1
	nRatioMu.Lock()
	defer nRatioMu.Unlock()
	if nRatio == nil {
		nRatio = make(map[time.Duration]int)
	} else if n, ok := nRatio[timeout]; ok {
		N = n
	}
	key := Key{Salt: salt, R: 8, P: 1, N: N}
	for now := time.Now(); time.Since(now) < timeout; {
		key.N <<= 1
		if key.Bytes, err = scrypt.Key(password, salt, key.N, key.R, key.P, keyLen); err != nil {
			return key, err
		}
	}
	nRatio[timeout] = key.N
	return key, nil
}

type Key struct {
	Bytes   []byte `json:"-"`
	Salt    []byte
	N, R, P int
}

func (key Key) String() string {
	type K struct {
		Bytes, Salt []byte
		N, R, P     int
	}
	k := K{Bytes: key.Bytes, Salt: key.Salt, N: key.N, R: key.R, P: key.P}
	b, err := json.Marshal(k)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func (key *Key) Populate(password []byte, keyLen int) error {
	var err error
	key.Bytes, err = scrypt.Key(password, key.Salt, key.N, key.R, key.P, keyLen)
	return err
}

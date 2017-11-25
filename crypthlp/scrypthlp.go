// Copyright 2017 Tamás Gulácsi. All rights reserved.
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

const minL2N = 14

var (
	durs   = make([]time.Duration, 0, 8)
	dursMu sync.Mutex
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
	key := Key{Salt: salt, R: 8, P: 1, L2N: minL2N}
	dursMu.Lock()
	defer dursMu.Unlock()
	for i, d := range durs {
		if d < timeout {
			key.L2N = minL2N + uint(i)
			continue
		}
		if d > timeout {
			key.L2N--
		}
		break
	}
	deadline := time.Now().Add(timeout)
	for now := time.Now(); now.Before(deadline); {
		if key.Bytes, err = scrypt.Key(password, salt, 1<<key.L2N, key.R, key.P, keyLen); err != nil {
			return key, err
		}
		now2 := time.Now()
		dur := now2.Sub(now)
		i := int(key.L2N - minL2N)
		if len(durs) <= i {
			if cap(durs) > i {
				durs = durs[:i+1]
			} else {
				durs = append(durs, make([]time.Duration, len(durs))...)
			}
		}
		durs[key.L2N-minL2N] = dur
		now = now2

		if now.Add(2 * dur).After(deadline) {
			break
		}
		key.L2N++
	}
	return key, nil
}

type Key struct {
	Bytes []byte `json:"-"`
	Salt  []byte
	L2N   uint
	R, P  int
}

func (key Key) String() string {
	k := struct {
		Bytes, Salt []byte
		L2N         uint
		R, P        int
	}(key)
	b, err := json.Marshal(k)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func (key *Key) Populate(password []byte, keyLen int) error {
	var err error
	key.Bytes, err = scrypt.Key(password, key.Salt, 1<<key.L2N, key.R, key.P, keyLen)
	return err
}

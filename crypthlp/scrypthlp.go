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
	dur16384   time.Duration
	dur16384Mu sync.Mutex
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
	key := Key{Salt: salt, R: 8, P: 1, L2N: 14}
	dur16384Mu.Lock()
	defer dur16384Mu.Unlock()
	if dur16384 == 0 {
		tim := time.Now()
		if key.Bytes, err = scrypt.Key(password, salt, 1<<14, key.R, key.P, keyLen); err != nil {
			return key, err
		}
		dur16384 = time.Since(tim)
	}
	n := int(int64(timeout)/int64(dur16384)) * 16384
	for key.L2N = 14; n > (1 << key.L2N); key.L2N++ {
	}
	key.L2N -= 2
	deadline := time.Now().Add(timeout)
	for {
		now := time.Now()
		if now.After(deadline) {
			break
		}
		key.L2N++
		if key.Bytes, err = scrypt.Key(password, salt, 1<<key.L2N, key.R, key.P, keyLen); err != nil {
			return key, err
		}
		dur := time.Since(now)
		if now.Add(dur + 2*dur).After(deadline) {
			break
		}
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
	type K struct {
		Bytes, Salt []byte
		L2N         uint
		R, P        int
	}
	k := K{Bytes: key.Bytes, Salt: key.Salt, L2N: key.L2N, R: key.R, P: key.P}
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

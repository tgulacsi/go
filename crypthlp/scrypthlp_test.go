// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package crypthlp_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/tgulacsi/go/crypthlp"
)

func TestSalt(t *testing.T) {
	salt, err := crypthlp.Salt(24)
	if err != nil {
		t.Fatal(err)
	}
	if len(salt) != 24 {
		t.Errorf("salt length mismatch: wanted 24, got %d.", len(salt))
	}
}

func TestKey(t *testing.T) {
	saltLen, keyLen, timeout := 24, 32, time.Second

	passphrase, err := crypthlp.Salt(keyLen)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	key, err := crypthlp.GenKey(passphrase, saltLen, keyLen, timeout)
	dur := time.Since(now)
	if err != nil {
		t.Fatal(err)
	}
	if len(key.Salt) != saltLen {
		t.Errorf("salt length mismatch: wanted 24, got %d.", len(key.Salt))
	}
	if len(key.Bytes) != keyLen {
		t.Errorf("key length mismatch: wanted %d, got %d", keyLen, len(key.Bytes))
	}
	if dur > 2*timeout {
		t.Errorf("timeout: wanted %s, got %s", timeout, dur)
	}
	t.Logf("key=%q", key)

	k := key.Bytes
	key.Bytes = nil
	if err = key.Populate(passphrase, keyLen); err != nil {
		t.Error(err)
	}
	if !bytes.Equal(key.Bytes, k) {
		t.Errorf("after populate, awaited\n\t%v,\n got \n\t%v.", k, key.Bytes)
	}
}

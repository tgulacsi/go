// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package crypthlp_test

import (
	"testing"
	"time"

	"github.com/tgulacsi/go/crypthlp"
)

func TestKey(t *testing.T) {
	saltLen, keyLen, timeout := 24, 32, time.Second

	now := time.Now()
	salt, key, err := crypthlp.Key([]byte("aaa"), saltLen, keyLen, timeout)
	dur := time.Since(now)
	if err != nil {
		t.Fatal(err)
	}
	if len(salt) != saltLen {
		t.Errorf("salt length mismatch: wanted %d, got %d", saltLen, len(salt))
	}
	if len(key) != keyLen {
		t.Errorf("key length mismatch: wanted %d, got %d", keyLen, len(key))
	}
	if dur > 2*timeout {
		t.Errorf("timeout: wanted %s, got %s", timeout, dur)
	}
}

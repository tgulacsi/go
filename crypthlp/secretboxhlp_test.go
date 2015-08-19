// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package crypthlp_test

import (
	"bytes"
	"io/ioutil"
	"testing"
	"time"

	"github.com/tgulacsi/go/crypthlp"
)

func TestSecretBox(t *testing.T) {
	passphrase, err := crypthlp.Salt(32)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	w, err := crypthlp.CreateWriter(&buf, passphrase, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write(passphrase); err != nil {
		t.Error(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("Written %d bytes (%q).", buf.Len(), buf.Bytes())

	_, r, err := crypthlp.OpenReader(bytes.NewReader(buf.Bytes()), passphrase)
	if err != nil {
		t.Fatal(err)
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, passphrase) {
		t.Errorf("wanted %q, got %q", passphrase, data)
	}
}

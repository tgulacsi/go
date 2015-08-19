// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package crypthlp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"time"

	"golang.org/x/crypto/nacl/secretbox"
)

func Open(filename string, passphrase []byte) (Key, io.Reader, error) {
	fh, err := os.Open(filename)
	if err != nil {
		return Key{}, nil, err
	}
	return OpenReader(fh, passphrase)
}

func OpenReader(r io.Reader, passphrase []byte) (Key, io.Reader, error) {
	dec := json.NewDecoder(r)
	var key Key
	if err := dec.Decode(&key); err != nil {
		return key, nil, err
	}
	if err := key.Populate(passphrase, 32); err != nil {
		return key, nil, err
	}
	box, err := ioutil.ReadAll(io.MultiReader(dec.Buffered(), r))
	if err != nil {
		return key, nil, err
	}
	box = box[1:] // trim \n of json.Encoder.Encode
	out := make([]byte, 0, len(box)-secretbox.Overhead)
	var (
		nonce [24]byte
		k     [32]byte
	)
	copy(nonce[:], key.Salt)
	copy(k[:], key.Bytes)
	data, ok := secretbox.Open(out, box, &nonce, &k)
	if !ok {
		return key, nil, errors.New("failed open box")
	}
	return key, bytes.NewReader(data), nil

}

var DefaultTimeout = 5 * time.Second

func Create(filename string, passphrase []byte, timeout time.Duration) (io.WriteCloser, error) {
	fh, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	return CreateWriter(fh, passphrase, timeout)
}

func CreateWriter(w io.Writer, passphrase []byte, timeout time.Duration) (io.WriteCloser, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	key, err := GenKey(passphrase, 24, 32, timeout)
	if err != nil {
		return nil, err
	}

	return key.CreateWriter(w)
}

func (key Key) CreateWriter(w io.Writer) (io.WriteCloser, error) {
	if err := json.NewEncoder(w).Encode(key); err != nil {
		return nil, err
	}
	return &secretWriter{key: key.Bytes, nonce: key.Salt, w: writeCloser{w}}, nil
}

type secretWriter struct {
	key, nonce []byte
	w          io.WriteCloser
	buf        bytes.Buffer
}

func (sw *secretWriter) Write(p []byte) (int, error) {
	return sw.buf.Write(p)
}
func (sw *secretWriter) Close() error {
	var key [32]byte
	var nonce [24]byte
	copy(key[:], sw.key)
	copy(nonce[:], sw.nonce)
	out := make([]byte, 0, sw.buf.Len()+secretbox.Overhead)
	_, err := sw.w.Write(secretbox.Seal(out, sw.buf.Bytes(), &nonce, &key))
	if err != nil {
		return err
	}
	return sw.w.Close()
}

type writeCloser struct {
	io.Writer
}

func (wc writeCloser) Close() error {
	if c, ok := wc.Writer.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

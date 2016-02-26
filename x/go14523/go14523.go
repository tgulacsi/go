package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sync"
)

func main() {
	fn := os.Args[0]

	{
		fh, err := os.Open(fn)
		if err != nil {
			log.Fatal(err)
		}
		rc := NewB64FilterReader(fh)
		r := bufio.NewReader(rc)
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r.Peek(512)
				if _, err := io.Copy(ioutil.Discard, r); err != nil {
					log.Fatal(err)
				}
			}()
		}
		wg.Wait()
		rc.Close()
		fh.Close()
	}

	return
}

const b64chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// NewB64Decoder returns a new filtering bae64 decoder.
func NewB64Decoder(enc *base64.Encoding, r io.Reader) io.Reader {
	return base64.NewDecoder(enc, NewB64FilterReader(NewB64FilterReader(r)))
}

// NewB64FilterReader returns a base64 filtering reader.
func NewB64FilterReader(r io.Reader) io.ReadCloser {
	return NewFilterReader(r, []byte(b64chars))
}

// NewFilterReader returns a reader which silently throws away bytes not in
// the okBytes slice.
func NewFilterReader(r io.Reader, okBytes []byte) io.ReadCloser {
	var okMap [256]bool
	for _, b := range okBytes {
		okMap[b] = true
	}
	pr, pw := io.Pipe()
	go func() {
		var length int64
		raw := make([]byte, 16<<10)
		filtered := make([]byte, cap(raw))
		for {
			n, readErr := r.Read(raw)
			if n == 0 && readErr == nil {
				continue
			}
			filtered = filtered[:n]
			i := 0
			for _, b := range raw[:n] {
				if okMap[b] {
					filtered[i] = b
					i++
				}
			}
			i, err := pw.Write(filtered[:i])
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			length += int64(i)
			if readErr == nil {
				continue
			}
			if readErr != io.EOF {
				pw.CloseWithError(err)
				return
			}
			if padding := int(length % 4); padding > 0 {
				if _, err := pw.Write(bytes.Repeat([]byte{'='}, 4-padding)); err != nil {
					pw.CloseWithError(err)
					return
				}
			}
			pw.CloseWithError(readErr)
			return
		}
	}()
	return pr
}

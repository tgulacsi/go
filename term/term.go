/*
Copyright 2015 Tamás Gulácsi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package term

import (
	"io"
	"os"
	"strings"

	"github.com/tgulacsi/go/text"
	"golang.org/x/text/encoding"
	"gopkg.in/inconshreveable/log15.v2/term"
)

var IsTTY = term.IsTty(os.Stdout.Fd())

// GetTTYEncoding returns the TTY encoding, or UTF-8 if not found.
func GetTTYEncoding() encoding.Encoding {
	enc := GetRawTTYEncoding()
	if enc != nil {
		return enc
	}
	return text.GetEncoding("UTF-8")
}

// GetRawTTYEncoding returns the TTY encoding, or nil if not found.
func GetRawTTYEncoding() encoding.Encoding {
	lang := os.Getenv("LANG")
	if lang == "" {
		return nil
	}
	if i := strings.IndexByte(lang, '.'); i >= 0 {
		return text.GetEncoding(lang[i+1:])
	}
	return nil
}

// MaskInOutTTY mask os.Stdin, os.Stdout, os.Stderr with the TTY encoding, if any.
func MaskInOutTTY() error {
	enc := GetRawTTYEncoding()
	if enc == nil {
		return nil
	}
	var err error
	if os.Stdin, err = MaskIn(os.Stdin, enc); err != nil {
		return err
	}
	return MaskStdoutErr(enc)
}

// MaskStdoutErr masks os.Stdout and os.Stderr.
func MaskStdoutErr(enc encoding.Encoding) error {
	var err error
	if os.Stdout, err = MaskOut(os.Stdout, enc); err != nil {
		return err
	}
	os.Stderr, err = MaskOut(os.Stderr, enc)
	return err
}

// MaskIn masks the input stream for Reads.
func MaskIn(in *os.File, enc encoding.Encoding) (*os.File, error) {
	pr, pw, err := os.Pipe()
	if err != nil {
		return in, err
	}
	// in -> pw -> pr
	go func() {
		defer in.Close()
		io.Copy(pw, text.NewReader(in, enc))

	}()
	return pr, nil
}

// MaskOut masks the output stream forWrites.
func MaskOut(out *os.File, enc encoding.Encoding) (*os.File, error) {
	pr, pw, err := os.Pipe()
	if err != nil {
		return out, err
	}
	// pw -> pr -> out
	go func() {
		defer out.Close()
		io.Copy(text.NewWriter(out, enc), pr)
	}()
	return pw, nil
}

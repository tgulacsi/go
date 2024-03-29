/*
Copyright 2017, 2023 Tamás Gulácsi

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

	"github.com/tgulacsi/go/iohlp"
	"golang.org/x/term"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
)

// IsTTY contains whether the stdout is a terminal.
var IsTTY = term.IsTerminal(int(os.Stdout.Fd()))

// GetTTYEncoding returns the TTY encoding, or UTF-8 if not found.
func GetTTYEncoding() encoding.Encoding {
	enc := GetRawTTYEncoding()
	if enc != nil {
		return enc
	}
	return encoding.Nop
}

// GetRawTTYEncoding returns the TTY encoding, or nil if not found.
func GetRawTTYEncoding() encoding.Encoding {
	lang := GetTTYEncodingName()
	if lang == "" {
		return nil
	}
	enc, err := htmlindex.Get(lang)
	if err != nil {
		panic(err)
	}
	return enc
}

// GetTTYEncodingName returns the TTY encoding's name, or empty if not found.
func GetTTYEncodingName() string {
	return GetLangEncodingName(os.Getenv("LANG"))
}

// GetLangEncodingName returns the encoding's name from the given LANG string.
func GetLangEncodingName(lang string) string {
	if i := strings.IndexByte(lang, '.'); i >= 0 {
		lang = lang[i+1:]
	}
	if len(lang) > 7 &&
		strings.EqualFold(lang[:7], "iso8859") {
		lang = lang[7:]
		for lang != "" && lang[0] == '-' || lang[0] == '_' {
			lang = lang[1:]
		}
		return "iso8859-" + lang
	}
	return strings.ToLower(lang)
}

// MaskInOutTTY mask os.Stdin, os.Stdout, os.Stderr with the TTY encoding, if any.
//
// WARNING! This uses os pipes, so kernel buffering may cut the tail!
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
//
// WARNING! This uses os pipes, so kernel buffering may cut the tail!
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
	iohlp.SetDirect(pr)
	iohlp.SetDirect(pw)
	// in -> pw -> pr
	go func() {
		defer in.Close()
		io.Copy(pw, enc.NewDecoder().Reader(in))

	}()
	return pr, nil
}

// MaskOut masks the output stream forWrites.
//
// WARNING! This uses os pipes, so kernel buffering may cut the tail!
func MaskOut(out *os.File, enc encoding.Encoding) (*os.File, error) {
	pr, pw, err := os.Pipe()
	if err != nil {
		return out, err
	}
	iohlp.SetDirect(pr)
	iohlp.SetDirect(pw)
	// pw -> pr -> out
	go func() {
		defer out.Close()
		io.Copy(enc.NewEncoder().Writer(out), pr)
	}()
	return pw, nil
}

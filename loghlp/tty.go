/*
Copyright 2014 Tamás Gulácsi

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

package loghlp

import (
	"os"

	"github.com/tgulacsi/go/term"
	"github.com/tgulacsi/go/text"
	"golang.org/x/text/encoding"
	"gopkg.in/inconshreveable/log15.v2"
)

// UseEncoding will use the given encoding for log15.StderrHandler.
// If enc is nil, GetRawTTYEncoding is used.
// If that returns nil, too, then nothing happens.
func UseEncoding(enc encoding.Encoding) {
	if enc == nil {
		if enc = term.GetRawTTYEncoding(); enc == nil {
			return
		}
	}
	logfmt := log15.LogfmtFormat()
	if term.IsTTY {
		logfmt = log15.TerminalFormat()
	}
	log15.StderrHandler = log15.StreamHandler(text.NewWriter(os.Stderr, enc), logfmt)
}

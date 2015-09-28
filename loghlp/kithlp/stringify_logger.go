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

package kithlp

import (
	"fmt"

	"github.com/go-kit/kit/log"
)

// StringifyLogger stringifies every value to make it printable by logfmt.
//
// Example:
//	Logger := log.LogfmtLogger(os.Stderr)
//	Logger = log.StringifyLogger{Logger}
type StringifyLogger struct {
	log.Logger
}

func (l StringifyLogger) Log(keyvals ...interface{}) error {
	for i := 1; i < len(keyvals); i += 2 {
		switch keyvals[i].(type) {
		case string, fmt.Stringer, fmt.Formatter:
		case error:
		default:
			keyvals[i] = StringWrap{Value: keyvals[i]}
		}
	}
	return l.Logger.Log(keyvals...)
}

var _ = fmt.Stringer(StringWrap{})

// StringWrap wraps the Value as a fmt.Stringer.
type StringWrap struct {
	Value interface{}
}

// String returns a string representation (%v) of the underlying Value.
func (sw StringWrap) String() string {
	return fmt.Sprintf("%v", sw.Value)
}

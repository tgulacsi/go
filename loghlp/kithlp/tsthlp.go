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
	"testing"

	"github.com/go-kit/kit/log"
	"gopkg.in/logfmt.v0"
	"gopkg.in/stack.v1"
)

func TestLogger(t testing.TB) log.Logger {
	return testLogger{t}
}

type testLogger struct {
	testing.TB
}

func (t testLogger) Log(keyvals ...interface{}) error {
	b, err := logfmt.MarshalKeyvals(append(keyvals, "stack", stack.Trace()[4:])...)
	if err != nil {
		t.TB.Log(fmt.Sprintf("%s: LOG_ERROR=%v", b, err))
		return err
	}
	t.TB.Log(string(b))
	return nil
}

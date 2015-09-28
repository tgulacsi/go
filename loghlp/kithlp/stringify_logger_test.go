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

package kithlp_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/tgulacsi/go/loghlp/kithlp"
)

func TestStringifyLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := log.NewLogfmtLogger(buf)
	logger = kithlp.StringifyLogger{logger}

	if err := logger.Log("hello", "world"); err != nil {
		t.Fatal(err)
	}
	if want, have := "hello=world\n", buf.String(); want != have {
		t.Errorf("want %#v, have %#v", want, have)
	}

	buf.Reset()
	if err := logger.Log("a", 1, "err", errors.New("error")); err != nil {
		t.Fatal(err)
	}
	if want, have := "a=1 err=error\n", buf.String(); want != have {
		t.Errorf("want %#v, have %#v", want, have)
	}

	buf.Reset()
	if err := logger.Log("std_map", map[int]int{1: 2}, "my_map", mymap{0: 0}); err != nil {
		t.Fatal(err)
	}
	if want, have := "std_map=map[1:2] my_map=special_behavior\n", buf.String(); want != have {
		t.Errorf("want %#v, have %#v", want, have)
	}
}

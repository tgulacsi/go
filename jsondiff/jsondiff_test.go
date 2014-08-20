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

package jsondiff

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/kylelemons/godebug/diff"
)

func TestPretty(t *testing.T) {
	var b bytes.Buffer
	for i, inout := range [][2]string{
		{"{}", "{}"},
		{`{"a":1}`, `{"a": 1
}`},
		{`{"m":{"a":1,"c":"b","b":[3,2,1]}}`, `{"m": {"a": 1,
    "b": [3,2,1],
    "c": "b"
  }
}`},
	} {
		m := make(map[string]interface{})
		if err := json.Unmarshal([]byte(inout[0]), &m); err != nil {
			t.Fatalf("%d. cannot unmarshal test input string: %v", i, err)
		}
		b.Reset()
		if err := Pretty(&b, m, ""); err != nil {
			t.Errorf("%d. Pretty: %v", i, err)
			continue
		}
		if inout[1] != b.String() {
			t.Errorf("%d. got %s\n\tawaited\n%s\n\tdiff\n%s", i, b.String(), inout[1],
				diff.Diff(inout[1], b.String()))
		}
	}
}

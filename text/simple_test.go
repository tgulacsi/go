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

package text

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestISO8859_1(t *testing.T) {
	enc := ISO8859_1
	for i, elt := range []struct {
		encoded []byte
		decoded string
	}{
		{[]byte("\xe1rv\xedz"), "árvíz"},
	} {
		b, err := ioutil.ReadAll(NewReader(bytes.NewReader(elt.encoded), enc))
		if err != nil {
			t.Errorf("%d. read: %v", i, err)
			continue
		}
		if string(b) != elt.decoded {
			t.Errorf("%d. decode mismatch: want %q, got %q", i, elt.decoded, b)
			continue
		}

		var buf bytes.Buffer
		w := NewWriter(&buf, enc)
		if w == nil {
			t.Errorf("%d nil writer for %v", i, enc)
		}
		if _, err = w.Write([]byte(elt.decoded)); err == nil {
			err = w.Close()
		}
		if err != nil {
			t.Errorf("%d. write: %v", i, err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), elt.encoded) {
			t.Errorf("%d. encode mismatch: want %q, got %q", i, elt.encoded, buf.Bytes())
		}
	}
}

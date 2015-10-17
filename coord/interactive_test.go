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

package coord

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInteractive(t *testing.T) {
	in := Interactive{}
	req, err := http.NewRequest("GET", "http://example.com/_koord", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp := httptest.NewRecorder()
	in.ServeHTTP(resp, req)
	if resp.Code != 200 {
		t.Errorf("simple GET: %d\n%s", resp.Code, resp.Body.String())
	}

	req, err = http.NewRequest("GET", "http://example.com/_koord/set?id=a&lat=%2B49.12&lng=-45.67", nil)
	resp = httptest.NewRecorder()
	in.ServeHTTP(resp, req)
	t.Logf("resp: %s", resp.Body.String())
}

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

package httpreq

import (
	"bytes"
	"io"
	"text/multipart"
	"net/http"
)

func ExampleCreateFormFile(url, name, contentType string, r io.Reader) error {
	// store entire content of the provided io.Reader in memory
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	// "upfile" will be the file's ID (field name)
	w, err := CreateFormFile(mw, "upfile", name, contentType)
	if err != nil {
		return err
	}
	if _, err = io.Copy(w, r); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return err
	}
	// this is essential: this dresses up our request properly as multipart/form-data
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.Client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return errors.New("bad response: " + resp.Status)
	}
	return nil
}

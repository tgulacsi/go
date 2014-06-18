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
	"mime/multipart"
	"strings"
	"io"
	"fmt"
	"net/textproto"
)

// CreatFormFile is like multipart.Writer.CreateFormFile, but allows the setting of Content-Type.
func CreateFormFile(w *multipart.Writer, fieldname, filename, contentType string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Type", contentType)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes(fieldname), escapeQuotes(filename)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return w.CreatePart(h)
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}


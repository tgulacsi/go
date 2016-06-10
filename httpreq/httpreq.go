/*
  Copyright 2013 Tamás Gulácsi

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

/*
Package httpreq for saving http.Request files
*/
package httpreq

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tgulacsi/go/temp"
)

// Log is discarded by default
var Log = func(...interface{}) error { return nil }

// ReadRequestOneFile reads the first file from the request (if multipart/),
// or returns the body if not
func ReadRequestOneFile(r *http.Request) (body io.ReadCloser, contentType string, status int, err error) {
	body = r.Body
	contentType = r.Header.Get("Content-Type")
	//Log("msg","ReadRequestOneFile", "ct", contentType)
	if !strings.HasPrefix(contentType, "multipart/") {
		// not multipart-form
		status = 200
		return
	}
	defer r.Body.Close()
	err = r.ParseMultipartForm(1 << 20)
	if err != nil {
		status, err = 405, errors.New("error parsing request as multipart-form: "+err.Error())
		return
	}
	if r.MultipartForm == nil || len(r.MultipartForm.File) == 0 {
		status, err = 405, errors.New("no files?")
		return
	}

Outer:
	for _, fileHeaders := range r.MultipartForm.File {
		for _, fileHeader := range fileHeaders {
			if body, err = fileHeader.Open(); err != nil {
				status, err = 405, fmt.Errorf("error opening part %q: %s", fileHeader.Filename, err)
				return
			}
			contentType = fileHeader.Header.Get("Content-Type")
			break Outer
		}
	}
	status = 200
	return
}

// ReadRequestFiles reads the files from the request, and calls ReaderToFile on them
func ReadRequestFiles(r *http.Request) (filenames []string, status int, err error) {
	defer r.Body.Close()
	err = r.ParseMultipartForm(1 << 20)
	if err != nil {
		status, err = 405, errors.New("cannot parse request as multipart-form: "+err.Error())
		return
	}
	if r.MultipartForm == nil || len(r.MultipartForm.File) == 0 {
		status, err = 405, errors.New("no files?")
		return
	}

	filenames = make([]string, 0, len(r.MultipartForm.File))
	var f multipart.File
	var fn string
	for _, fileHeaders := range r.MultipartForm.File {
		for _, fileHeader := range fileHeaders {
			if f, err = fileHeader.Open(); err != nil {
				status, err = 405, fmt.Errorf("error reading part %q: %s", fileHeader.Filename, err)
				return
			}
			//Log("msg","part", "filename", fileHeader.Filename)
			if fn, err = temp.ReaderToFile(f, fileHeader.Filename, ""); err != nil {
				f.Close()
				status, err = 500, fmt.Errorf("error saving %q: %s", fileHeader.Filename, err)
				return
			}
			f.Close()
			filenames = append(filenames, fn)
		}
	}
	if len(filenames) == 0 {
		status, err = 405, errors.New("no files??")
		return
	}
	status = 200
	return
}

// SendFile sends the given file as response
func SendFile(w http.ResponseWriter, filename, contentType string) error {
	fh, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fh.Close()
	fi, err := fh.Stat()
	if err != nil {
		return err
	}
	size := fi.Size()
	if _, err = fh.Seek(0, 0); err != nil {
		err = fmt.Errorf("error seeking in %v: %s", fh, err)
		http.Error(w, err.Error(), 500)
		return err
	}
	if contentType != "" {
		w.Header().Add("Content-Type", contentType)
	}
	w.Header().Add("Content-Length", fmt.Sprintf("%d", size))
	w.WriteHeader(200)
	Log("msg", "SendFile", "filename", filename, "length", size, "header", w.Header())
	fh.Seek(0, 0)
	if _, err = io.CopyN(w, fh, size); err != nil {
		err = fmt.Errorf("error sending file %q: %s", filename, err)
		Log("msg", "SendFile", "filename", filename, "error", err)
	}
	return err
}

// GetCombinedLogLine returns a CombinedLog - format of the request
// copied from https://github.com/gorilla/handlers/blob/master/handlers.go
func GetCombinedLogLine(r *http.Request, ts time.Time, status int, size int) string {
	username := "-"
	if r.URL.User != nil {
		if name := r.URL.User.Username(); name != "" {
			username = name
		}
	}

	return fmt.Sprintf("%s - %s [%s] \"%s %s %s\" %d %d \"%s\" \"%s\"",
		strings.Split(r.RemoteAddr, ":")[0],
		username,
		ts.Format("02/Jan/2006:15:04:05 -0700"),
		r.Method,
		r.URL.RequestURI(),
		r.Proto,
		status,
		size,
		r.Referer(),
		r.UserAgent(),
	)
}

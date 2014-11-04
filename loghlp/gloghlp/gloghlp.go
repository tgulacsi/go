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

// Package gloghlp is a wrap against glog flag set.
package gloghlp

import (
	"github.com/golang/glog"
	"gopkg.in/inconshreveable/log15.v2"
)

// GLogHandler returns a log15.Handler which logs using glog.
func GLogHandler() log15.Handler {
	return glogHandler{log15.LogfmtFormat()}
}

type glogHandler struct {
	fmt log15.Format
}

func (gl glogHandler) Log(r *log15.Record) error {
	b := gl.fmt.Format(r)
	s := string(b[:len(b)-1]) // strip \n
	switch r.Lvl {
	case log15.LvlCrit:
		glog.Fatal(s)
	case log15.LvlError:
		glog.Error(s)
	case log15.LvlWarn:
		glog.Warning(s)
	case log15.LvlInfo:
		glog.Info(s)
	default:
		glog.V(1).Info(s)
	}
	return nil
}


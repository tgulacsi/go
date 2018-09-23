/*
  Copyright 2017 Tamás Gulácsi

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

package pdf

import (
	"github.com/hhrutter/pdfcpu/pkg/api"
	"github.com/hhrutter/pdfcpu/pkg/pdfcpu"
)

// Log is used for logging.
var Log = func(...interface{}) error { return nil }

// MergeFiles merges the given sources into dest.
func MergeFiles(dest string, sources ...string) error {
	return api.Merge(sources, dest, pdfcpu.NewDefaultConfiguration())
}

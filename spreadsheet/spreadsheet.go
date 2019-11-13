/*
Copyright 2019 Tamás Gulácsi

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

package spreadsheet

import "io"

// Writer writes the spreadsheet consisting of the sheets created
// with NewSheet. The write finishes when Close is called.
type Writer interface {
	io.Closer
	NewSheet(name string, cols []Column) (Sheet, error)
}

// Sheet should be Closed when finished.
type Sheet interface {
	io.Closer
	AppendRow(values ...interface{}) error
}

// Style is a style for a column/row/cell.
type Style struct {
	// FontBold is true if the font is bold
	FontBold bool
	// Format is the number format
	Format string
}

// Column contains the Name of the column and header's style and column's style.
type Column struct {
	Name           string
	Header, Column Style
}

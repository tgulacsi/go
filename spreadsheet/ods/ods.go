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

package ods

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"net/http"
	"os"
	"sync"

	qt "github.com/valyala/quicktemplate"
	errors "golang.org/x/xerrors"

	"github.com/rakyll/statik/fs"
	_ "github.com/tgulacsi/go/spreadsheet/ods/statik"
)

//go:generate statik -f -src assets
//go:generate qtc

var qtMu sync.Mutex

// AcquireWriter wraps the given io.Writer to be usable with quicktemplates.
func AcquireWriter(w io.Writer) *qt.Writer {
	qtMu.Lock()
	W := qt.AcquireWriter(w)
	qtMu.Unlock()
	return W
}

// ReleaseWriter returns the *quicktemplate.Writer to the pool.
func ReleaseWriter(W *qt.Writer) { qtMu.Lock(); qt.ReleaseWriter(W); qtMu.Unlock() }

// Table or sheet.
type Table struct {
	Name     string
	Style    string
	ColCount int
	Heading  Row
}

// Row with style.
type Row struct {
	Style string
	Cells []Cell
}

// Cell with style, type and value.
type Cell struct {
	Style string
	Type  ValueType
	Value string
}

// ValueType is the cell's value's type.
type ValueType uint8

func (v ValueType) String() string {
	switch v {
	case 'f':
		return "float"
	case 'd':
		return "date"
	default:
		return "string"
	}
}

const (
	// FloatType for numerical data
	FloatType = ValueType('f')
	// DateType for dates
	DateType = ValueType('d')
	// StringType for everything else
	StringType = ValueType('s')
)

var statikFS http.FileSystem

func init() {
	var err error
	if statikFS, err = fs.New(); err != nil {
		panic(err)
	}
}

// NewWriter returns a content writer and a zip closer for an ods file.
func NewWriter(w io.Writer) (*ODSWriter, error) {
	zw := zip.NewWriter(w)
	if err := fs.Walk(statikFS, "/", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		b, err := fs.ReadFile(statikFS, path)
		if err != nil {
			return errors.Errorf("%s %s: %w", path, info, err)
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return errors.Errorf("%s: %w", path, err)
		}
		hdr.Method = zip.Deflate
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}); err != nil {
		zw.Close()
		return nil, errors.Errorf("Walk: %w", err)
	}

	bw, err := zw.Create("content.xml")
	if err != nil {
		zw.Close()
		return nil, err
	}
	W := AcquireWriter(bw)
	StreamBeginSheets(W)

	return &ODSWriter{qtWriter: W, zipWriter: zw}, nil
}

// ODSWriter writes content.xml of ODS zip.
type ODSWriter struct {
	qtWriter  *qt.Writer
	zipWriter *zip.Writer
}

func (ow *ODSWriter) QTWriter() *qt.Writer { return ow.qtWriter }

// Close the ODSWriter.
func (ow *ODSWriter) Close() error {
	if ow == nil || ow.qtWriter == nil {
		return nil
	}
	StreamEndSheets(ow.qtWriter)
	ow.qtWriter = nil
	err := ow.zipWriter.Close()
	ow.zipWriter = nil
	return err
}

// Style information - generated from content.xml with github.com/miek/zek/cmd/zek.
type Style struct {
	XMLName         xml.Name `xml:"style"`
	Name            string   `xml:"name,attr"`
	Family          string   `xml:"family,attr"`
	MasterPageName  string   `xml:"master-page-name,attr"`
	DataStyleName   string   `xml:"data-style-name,attr"`
	TableProperties struct {
		Display     string `xml:"display,attr"`
		WritingMode string `xml:"writing-mode,attr"`
	} `xml:"table-properties"`
	TextProperties struct {
		FontWeight           string `xml:"font-weight,attr"`
		FontStyle            string `xml:"font-style,attr"`
		TextPosition         string `xml:"text-position,attr"`
		TextLineThroughType  string `xml:"text-line-through-type,attr"`
		TextLineThroughStyle string `xml:"text-line-through-style,attr"`
		TextUnderlineType    string `xml:"text-underline-type,attr"`
		TextUnderlineStyle   string `xml:"text-underline-style,attr"`
		TextUnderlineWidth   string `xml:"text-underline-width,attr"`
		Display              string `xml:"display,attr"`
		TextUnderlineColor   string `xml:"text-underline-color,attr"`
		TextUnderlineMode    string `xml:"text-underline-mode,attr"`
		FontSize             string `xml:"font-size,attr"`
		Color                string `xml:"color,attr"`
		FontFamily           string `xml:"font-family,attr"`
	} `xml:"text-properties"`
	TableRowProperties struct {
		RowHeight           string `xml:"row-height,attr"`
		UseOptimalRowHeight string `xml:"use-optimal-row-height,attr"`
	} `xml:"table-row-properties"`
	TableColumnProperties struct {
		ColumnWidth           string `xml:"column-width,attr"`
		UseOptimalColumnWidth string `xml:"use-optimal-column-width,attr"`
	} `xml:"table-column-properties"`
	TableCellProperties struct {
		BackgroundColor          string `xml:"background-color,attr"`
		BorderTop                string `xml:"border-top,attr"`
		BorderBottom             string `xml:"border-bottom,attr"`
		BorderLeft               string `xml:"border-left,attr"`
		BorderRight              string `xml:"border-right,attr"`
		DiagonalBlTr             string `xml:"diagonal-bl-tr,attr"`
		DiagonalTlBr             string `xml:"diagonal-tl-br,attr"`
		VerticalAlign            string `xml:"vertical-align,attr"`
		WrapOption               string `xml:"wrap-option,attr"`
		ShrinkToFit              string `xml:"shrink-to-fit,attr"`
		WritingMode              string `xml:"writing-mode,attr"`
		GlyphOrientationVertical string `xml:"glyph-orientation-vertical,attr"`
		CellProtect              string `xml:"cell-protect,attr"`
		RotationAlign            string `xml:"rotation-align,attr"`
		RotationAngle            string `xml:"rotation-angle,attr"`
		PrintContent             string `xml:"print-content,attr"`
		DecimalPlaces            string `xml:"decimal-places,attr"`
		TextAlignSource          string `xml:"text-align-source,attr"`
		RepeatContent            string `xml:"repeat-content,attr"`
	} `xml:"table-cell-properties"`
	ParagraphProperties struct {
		WritingModeAutomatic string `xml:"writing-mode-automatic,attr"`
		MarginLeft           string `xml:"margin-left,attr"`
	} `xml:"paragraph-properties"`
}

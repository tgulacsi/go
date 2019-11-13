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

package xlsx

import (
	"fmt"
	errors "golang.org/x/xerrors"
	"io"
	"strings"
	"time"

	"github.com/360EntSecGroup-Skylar/excelize/v2"
	"github.com/tgulacsi/go/spreadsheet"
)

var _ = (spreadsheet.Writer)((*XLSXWriter)(nil))

type XLSXWriter struct {
	xl     *excelize.File
	w      io.Writer
	sheets []string
	styles map[string]int
}

type XLSXSheet struct {
	Name string

	xl  *excelize.File
	row int64
}

func NewWriter(w io.Writer) *XLSXWriter {
	return &XLSXWriter{w: w, xl: excelize.NewFile()}
}

func (xlw *XLSXWriter) Close() error {
	xl, w := xlw.xl, xlw.w
	xlw.xl, xlw.w = nil, nil
	if xl == nil || w == nil {
		return nil
	}
	_, err := xl.WriteTo(w)
	return err
}
func (xlw *XLSXWriter) NewSheet(name string, columns []spreadsheet.Column) (spreadsheet.Sheet, error) {
	xlw.sheets = append(xlw.sheets, name)
	if len(xlw.sheets) == 1 { // first
		xlw.xl.SetSheetName("Sheet1", name)
	} else {
		xlw.xl.NewSheet(name)
	}
	var hasHeader bool
	for i, c := range columns {
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return nil, err
		}
		if s := xlw.getStyle(c.Column); s != 0 {
			if err = xlw.xl.SetColStyle(name, col, s); err != nil {
				return nil, err
			}
		}
		if s := xlw.getStyle(c.Header); s != 0 {
			if err = xlw.xl.SetCellStyle(name, col+"1", col+"1", s); err != nil {
				return nil, err
			}
		}
		if c.Name != "" {
			hasHeader = true
			if err = xlw.xl.SetCellStr(name, col+"1", c.Name); err != nil {
				return nil, err
			}
		}
	}
	xls := &XLSXSheet{xl: xlw.xl}
	if hasHeader {
		xls.row++
	}
	return xls, nil
}

func (xlw XLSXWriter) getStyle(style spreadsheet.Style) int {
	if !style.FontBold && style.Format == "" {
		return 0
	}
	k := fmt.Sprintf("%b\t%s", style.FontBold, style.Format)
	s, ok := xlw.styles[k]
	if ok {
		return s
	}
	var buf strings.Builder
	buf.WriteByte('{')
	if style.FontBold {
		buf.WriteString(`{"font":{"bold":true}}`)
	}
	if style.Format != "" {
		if buf.Len() > 1 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, `{"custom_number_format":%q}`, style.Format)
	}
	buf.WriteByte('}')
	s, err := xlw.xl.NewStyle(buf.String())
	if err != nil {
		panic(errors.Errorf("%s: %w", err))
	}
	xlw.styles[k] = s
	return s
}

func (xls *XLSXSheet) Close() error { return nil }
func (xls *XLSXSheet) AppendRow(values ...interface{}) error {
	xls.row++
	for i, v := range values {
		axis, err := excelize.CoordinatesToCellName(i, int(xls.row))
		if err != nil {
			return err
		}
		isNil := v == nil
		if !isNil {
			if t, ok := v.(time.Time); ok {
				if isNil = t.IsZero(); !isNil {
					if err = xls.xl.SetCellStr(xls.Name, axis, t.Format("2006-01-02")); err != nil {
						return err
					}
					continue
				}
			}
		}
		if isNil {
			continue
		}
		if err = xls.xl.SetCellValue(xls.Name, axis, v); err != nil {
			return err
		}
	}
	return nil
}

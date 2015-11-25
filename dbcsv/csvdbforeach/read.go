// Copyright 2011-2015, Tamás Gulácsi.
// All rights reserved.
// For details, see the LICENSE file.

package main

import (
	"encoding/csv"
	"io"
	"strings"
	"time"

	"github.com/extrame/xls"
	"github.com/tealeg/xlsx"
	"gopkg.in/errgo.v1"
)

const (
	dateFormat     = "20060102"
	dateTimeFormat = "20060102150405"
)

var timeReplacer = strings.NewReplacer(
	"yyyy", "2006",
	"yy", "06",
	"dd", "02",
	"d", "2",
	"mmm", "Jan",
	"mmss", "0405",
	"ss", "05",
	"hh", "15",
	"h", "3",
	"mm:", "04:",
	":mm", ":04",
	"mm", "01",
	"am/pm", "pm",
	"m/", "1/",
	".0", ".9999",
)

func readXLSXFile(rows chan<- Row, filename string, sheetIndex int) error {
	xlFile, err := xlsx.OpenFile(filename)
	if err != nil {
		return errgo.Notef(err, "open %q", filename)
	}
	sheetLen := len(xlFile.Sheets)
	switch {
	case sheetLen == 0:
		return errgo.New("This XLSX file contains no sheets.")
	case sheetIndex >= sheetLen:
		return errgo.Newf("No sheet %d available, please select a sheet between 0 and %d\n", sheetIndex, sheetLen-1)
	}
	sheet := xlFile.Sheets[sheetIndex]
	n := 0
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		vals := make([]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			numFmt := cell.GetNumberFormat()
			if strings.Contains(numFmt, "yy") || strings.Contains(numFmt, "mm") || strings.Contains(numFmt, "dd") {
				goFmt := timeReplacer.Replace(numFmt)
				dt, err := time.Parse(goFmt, cell.String())
				if err != nil {
					return errgo.Notef(err, "parse %q as %q (from %q)", cell.String(), goFmt, numFmt)
				}
				vals = append(vals, dt.Format(dateFormat))
			} else {
				vals = append(vals, cell.String())
			}
		}
		rows <- Row{Line: n, Values: vals}
		n++
	}
	return nil
}

func readXLSFile(rows chan<- Row, filename string, charset string, sheetIndex int) error {
	wb, err := xls.Open(filename, charset)
	if err != nil {
		return errgo.Notef(err, "open %q", filename)
	}
	sheet := wb.GetSheet(sheetIndex)
	if sheet == nil {
		return errgo.Newf("This XLS file does not contain sheet no %d!", sheetIndex)
	}
	var maxWidth int
	for n, row := range sheet.Rows {
		if row == nil {
			continue
		}
		vals := make([]string, maxWidth)
		for _, col := range row.Cols {
			if len(vals) <= int(col.LastCol()) {
				maxWidth = int(col.LastCol()) + 1
				vals = append(vals, make([]string, maxWidth-len(vals))...)
			}
			off := int(col.FirstCol())
			for i, s := range col.String(wb) {
				vals[off+i] = s
			}
		}
		rows <- Row{Line: int(n), Values: vals}
	}
	return nil
}

func readCSV(rows chan<- Row, r io.Reader, delim string) error {
	cr := csv.NewReader(r)
	cr.Comma = ([]rune(delim))[0]
	n := 0
	for {
		row, err := cr.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		rows <- Row{Line: n, Values: row}
		n++
	}
	return nil
}

type Row struct {
	Line   int
	Values []string
}

// Copyright 2017 Tamás Gulácsi. All rights reserved.

package dbcsv

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"

	"github.com/extrame/xls"
	"github.com/pkg/errors"
	"github.com/tealeg/xlsx"
)

var DefaultEncoding = encoding.Replacement

func init() {
	encName := os.Getenv("LANG")
	if i := strings.IndexByte(encName, '.'); i >= 0 {
		if enc, err := htmlindex.Get(encName[i+1:]); err == nil {
			DefaultEncoding = enc
		}
	}
}

type FileType string

const (
	Unknown = FileType("")
	Csv     = FileType("csv")
	Xls     = FileType("xls")
	XlsX    = FileType("xlsx")
)

func DetectReaderType(r io.Reader, fileName string) (FileType, error) {
	// detect file type
	var b [4]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return Unknown, err
	}
	if bytes.Equal(b[:], []byte{0xd0, 0xcf, 0x11, 0xe0}) { // OLE2
		return Xls, nil
	} else if bytes.Equal(b[:], []byte{0x50, 0x4b, 0x03, 0x04}) { //PKZip, so xlsx
		return XlsX, nil
	} else if bytes.Equal(b[:1], []byte{'"'}) { // CSV
		return Csv, nil
	}
	switch filepath.Ext(fileName) {
	case ".xls":
		return Xls, nil
	case ".xlsx":
		return XlsX, nil
	default:
		return Csv, nil
	}
}

type Config struct {
	Type          FileType
	Sheet, Skip   int
	Delim         string
	Charset       string
	ColumnsString string
}

func (cfg Config) Encoding() (encoding.Encoding, error) {
	if cfg.Charset == "" {
		return DefaultEncoding, nil
	}
	return htmlindex.Get(cfg.Charset)
}

func (cfg Config) Columns() ([]int, error) {
	if cfg.ColumnsString == "" {
		return nil, nil
	}
	columns := make([]int, 0, strings.Count(cfg.ColumnsString, ",")+1)
	for _, x := range strings.Split(cfg.ColumnsString, ",") {
		i, err := strconv.Atoi(x)
		if err != nil {
			return columns, errors.Wrap(err, x)
		}
		columns = append(columns, i-1)
	}
	return columns, nil
}

func (cfg Config) ReadRows(rows chan<- Row, fileName string) error {
	dst := rows
	if cfg.Skip != 0 {
		inter := make(chan Row, 1)
		dst = inter
		go func() {
			defer close(inter)
			for i := 0; i < cfg.Skip; i++ {
				<-inter
			}
			for row := range inter {
				rows <- row
			}
		}()
	}

	switch cfg.Type {
	case Xls:
		return ReadXLSFile(dst, fileName, cfg.Charset, cfg.Sheet)
	case XlsX:
		return ReadXLSXFile(dst, fileName, cfg.Sheet)
	}
	enc, err := cfg.Encoding()
	if err != nil {
		return err
	}
	fh, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer fh.Close()
	r := transform.NewReader(fh, enc.NewDecoder())
	return ReadCSV(dst, r, cfg.Delim)
}

const (
	//dateFormat = "2006-01-02 15:04:05"

	DateFormat     = "20060102"
	DateTimeFormat = "20060102150405"
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

func ReadXLSXFile(rows chan<- Row, filename string, sheetIndex int) error {
	xlFile, err := xlsx.OpenFile(filename)
	if err != nil {
		return errors.Wrapf(err, "open %q", filename)
	}
	sheetLen := len(xlFile.Sheets)
	switch {
	case sheetLen == 0:
		return errors.New("This XLSX file contains no sheets.")
	case sheetIndex >= sheetLen:
		return errors.New(fmt.Sprintf("No sheet %d available, please select a sheet between 0 and %d\n", sheetIndex, sheetLen-1))
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
					return errors.Wrapf(err, "parse %q as %q (from %q)", cell.String(), goFmt, numFmt)
				}
				vals = append(vals, dt.Format(DateFormat))
			} else {
				vals = append(vals, cell.String())
			}
		}
		rows <- Row{Line: n, Values: vals}
		n++
	}
	return nil
}

func ReadXLSFile(rows chan<- Row, filename string, charset string, sheetIndex int) error {
	wb, err := xls.Open(filename, charset)
	if err != nil {
		return errors.Wrapf(err, "open %q", filename)
	}
	sheet := wb.GetSheet(sheetIndex)
	if sheet == nil {
		return errors.New(fmt.Sprintf("This XLS file does not contain sheet no %d!", sheetIndex))
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

func ReadCSV(rows chan<- Row, r io.Reader, delim string) error {
	if delim == "" {
		br := bufio.NewReader(r)
		b, _ := br.Peek(1024)
		r = br
		b = bytes.Map(
			func(r rune) rune {
				if r == '"' || unicode.IsDigit(r) || unicode.IsLetter(r) {
					return -1
				}
				return r
			},
			b,
		)
		for len(b) > 1 && b[0] == ' ' {
			b = b[1:]
		}
		s := []rune(string(b))
		if len(s) > 4 {
			s = s[:4]
		}
		delim = string(s[:1])
		log.Printf("Non-alphanum characters are %q, so delim is %q.", s, delim)
	}
	cr := csv.NewReader(r)

	cr.Comma = ([]rune(delim))[0]
	cr.LazyQuotes = true
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

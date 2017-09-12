// Copyright 2017 Tamás Gulácsi. All rights reserved.

package dbcsv

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
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
	typ           FileType
	Sheet, Skip   int
	Delim         string
	Charset       string
	ColumnsString string
	encoding      encoding.Encoding
	columns       []int
	fileName      string
}

func (cfg *Config) Encoding() (encoding.Encoding, error) {
	if cfg.encoding != nil {
		return cfg.encoding, nil
	}
	if cfg.Charset == "" {
		return DefaultEncoding, nil
	}
	var err error
	cfg.encoding, err = htmlindex.Get(cfg.Charset)
	return cfg.encoding, err
}

func (cfg *Config) Columns() ([]int, error) {
	if cfg.ColumnsString == "" {
		return nil, nil
	}
	if cfg.columns != nil {
		return cfg.columns, nil
	}
	cfg.columns = make([]int, 0, strings.Count(cfg.ColumnsString, ",")+1)
	for _, x := range strings.Split(cfg.ColumnsString, ",") {
		i, err := strconv.Atoi(x)
		if err != nil {
			return cfg.columns, errors.Wrap(err, x)
		}
		cfg.columns = append(cfg.columns, i-1)
	}
	return cfg.columns, nil
}

func (cfg *Config) Type(fileName string) (FileType, error) {
	if cfg.typ != Unknown {
		return cfg.typ, nil
	}
	fh, err := os.Open(fileName)
	if err != nil {
		return Unknown, err
	}
	defer fh.Close()
	cfg.typ, err = DetectReaderType(fh, fileName)
	return cfg.typ, err
}

func (cfg *Config) ReadRows(ctx context.Context, rows chan<- Row, fileName string) (err error) {
	defer func() {
		if err != nil {
			log.Printf("ReadRows(%q): %v", fileName, err)
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	columns, err := cfg.Columns()
	if err != nil {
		return err
	}

	if fileName == "-" || fileName == "" {
		panic(fileName)
		if cfg.fileName != "" {
			fileName = cfg.fileName
		}
		fh, err := ioutil.TempFile("", "ReadRows-")
		if err != nil {
			return err
		}
		cfg.fileName = fh.Name()
		fileName = cfg.fileName
		defer fh.Close()
		//defer os.Remove(fileName)
		if _, err := io.Copy(fh, os.Stdin); err != nil {
			return err
		}
		if err := fh.Close(); err != nil {
			return err
		}
	}

	typ, err := cfg.Type(fileName)
	if err != nil {
		return err
	}
	switch typ {
	case Xls:
		return ReadXLSFile(ctx, rows, fileName, cfg.Charset, cfg.Sheet, columns, cfg.Skip)
	case XlsX:
		return ReadXLSXFile(ctx, rows, fileName, cfg.Sheet, columns, cfg.Skip)
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
	return ReadCSV(ctx, rows, r, cfg.Delim, columns, cfg.Skip)
}

const (
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

func ReadXLSXFile(ctx context.Context, rows chan<- Row, filename string, sheetIndex int, columns []int, skip int) error {
	defer close(rows)
	if err := ctx.Err(); err != nil {
		return err
	}
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
	var need map[int]bool
	if columns != nil {
		need = make(map[int]bool, len(columns))
		for _, i := range columns {
			need[i] = true
		}
	}
	for i, row := range sheet.Rows {
		if i < skip {
			continue
		}
		if row == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		vals := make([]string, 0, len(row.Cells))
		for i, cell := range row.Cells {
			if need != nil && !need[i] {
				continue
			}
			s := cell.String()
			if s == "" {
				vals = append(vals, "")
				continue
			}
			numFmt := cell.GetNumberFormat()
			if !(strings.Contains(numFmt, "yy") || strings.Contains(numFmt, "mm") || strings.Contains(numFmt, "dd")) {
				vals = append(vals, s)
				continue
			}

			goFmt := timeReplacer.Replace(numFmt)
			dt, err := time.Parse(goFmt, s)
			if err != nil {
				return errors.Wrapf(err, "parse %q as %q (from %q)", s, goFmt, numFmt)
			}
			vals = append(vals, dt.Format(DateFormat))
		}
		select {
		case rows <- Row{Line: n, Values: vals}:
		case <-ctx.Done():
			return ctx.Err()
		}
		n++
	}
	return nil
}

func ReadXLSFile(ctx context.Context, rows chan<- Row, filename string, charset string, sheetIndex int, columns []int, skip int) error {
	defer close(rows)
	if err := ctx.Err(); err != nil {
		return err
	}
	wb, err := xls.Open(filename, charset)
	if err != nil {
		return errors.Wrapf(err, "open %q", filename)
	}
	sheet := wb.GetSheet(sheetIndex)
	if sheet == nil {
		return errors.New(fmt.Sprintf("This XLS file does not contain sheet no %d!", sheetIndex))
	}
	var need map[int]bool
	if columns != nil {
		need = make(map[int]bool, len(columns))
		for _, i := range columns {
			need[i] = true
		}
	}
	var maxWidth int
	for n := 0; n < int(sheet.MaxRow); n++ {
		row := sheet.Row(n)
		if n < skip {
			continue
		}
		if row == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		vals := make([]string, 0, maxWidth)
		off := row.FirstCol()
		if len(vals) <= row.LastCol() {
			maxWidth = row.LastCol() + 1
			vals = append(vals, make([]string, maxWidth-len(vals))...)
		}

		for j := off; j < row.LastCol(); j++ {
			if need != nil && !need[int(j)] {
				continue
			}
			vals[j] = row.Col(j)
		}
		select {
		case rows <- Row{Line: int(n), Values: vals}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func ReadCSV(ctx context.Context, rows chan<- Row, r io.Reader, delim string, columns []int, skip int) error {
	defer close(rows)
	if err := ctx.Err(); err != nil {
		return err
	}
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
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	n := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		row, err := cr.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		n++
		if n < skip {
			continue
		}
		if columns != nil {
			r2 := make([]string, len(columns))
			for i, j := range columns {
				r2[i] = row[j]
			}
			row = r2
		}
		select {
		case rows <- Row{Line: n - 1, Values: row}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

type Row struct {
	Line   int
	Values []string
}

// vim: set noet fileencoding=utf-8:

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
	"unicode"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"

	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/extrame/xls"
	"github.com/pkg/errors"
)

var DefaultEncoding = encoding.Replacement
var UnknownSheet = errors.New("unknown sheet")

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
	file          *os.File
	permanent     bool
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

func (cfg *Config) Type() (FileType, error) {
	if cfg.typ != Unknown {
		return cfg.typ, nil
	}
	var err error
	cfg.typ, err = DetectReaderType(cfg.file, cfg.fileName)
	if err == nil {
		_, err = cfg.file.Seek(0, 0)
	}
	return cfg.typ, err
}

func (cfg *Config) OpenVolatile(fileName string) error {
	cfg.fileName = fileName
	if fileName == "-" || fileName == "" {
		cfg.file, cfg.permanent = os.Stdin, false
		return nil
	}
	var err error
	cfg.file, err = os.Open(fileName)
	if fi, statErr := cfg.file.Stat(); statErr != nil || !fi.Mode().IsRegular() {
		cfg.permanent = false
	}
	return err
}
func (cfg *Config) Open(fileName string) error {
	slurp := fileName == "-" || fileName == ""
	cfg.permanent = true
	if slurp {
		cfg.file, fileName = os.Stdin, "-"
	} else {
		var err error
		if cfg.file, err = os.Open(fileName); err != nil {
			return errors.Wrap(err, "open "+fileName)
		}
		fi, err := cfg.file.Stat()
		if err != nil {
			cfg.file.Close()
			return errors.Wrap(err, "stat "+fileName)
		}
		slurp = !fi.Mode().IsRegular()
	}
	var err error
	if slurp {
		fh, tmpErr := ioutil.TempFile("", "ReadRows-")
		if tmpErr != nil {
			return tmpErr
		}
		defer fh.Close()
		fileName = fh.Name()
		defer os.Remove(fileName)
		log.Printf("Copying into temporary file %q...", fileName)
		if _, err = io.Copy(fh, cfg.file); err != nil {
			return errors.Wrap(err, "copy into "+fh.Name())
		}
		if err = fh.Close(); err != nil {
			return errors.Wrap(err, "close "+fh.Name())
		}
		if cfg.file, err = os.Open(fileName); err != nil {
			return errors.Wrap(err, "open "+fileName)
		}
	}
	cfg.fileName = fileName
	_, err = cfg.Type()
	return errors.Wrap(err, "type "+cfg.fileName)
}

func (cfg *Config) Close() error {
	fh := cfg.file
	cfg.file, cfg.fileName, cfg.typ = nil, "", Unknown
	if fh != nil {
		return fh.Close()
	}
	return nil
}

func (cfg *Config) ReadRows(ctx context.Context, fn func(string, Row) error) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}
	columns, err := cfg.Columns()
	if err != nil {
		return err
	}

	if cfg.permanent {
		if _, err = cfg.file.Seek(0, 0); err != nil {
			return err
		}
	}
	switch cfg.typ {
	case Xls:
		return ReadXLSFile(ctx, fn, cfg.fileName, cfg.Charset, cfg.Sheet, columns, cfg.Skip)
	case XlsX:
		return ReadXLSXFile(ctx, fn, cfg.fileName, cfg.Sheet, columns, cfg.Skip)
	}
	enc, err := cfg.Encoding()
	if err != nil {
		return err
	}
	r := transform.NewReader(cfg.file, enc.NewDecoder())
	return ReadCSV(ctx, func(row Row) error { return fn(cfg.fileName, row) }, r, cfg.Delim, columns, cfg.Skip)
}

const (
	DateFormat     = "20060102"
	DateTimeFormat = "20060102150405"
)

func ReadXLSXFile(ctx context.Context, fn func(string, Row) error, filename string, sheetIndex int, columns []int, skip int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	xlFile, err := excelize.OpenFile(filename)
	if err != nil {
		return errors.Wrapf(err, "open %q", filename)
	}
	sheetName := xlFile.GetSheetName(sheetIndex)
	if sheetName == "" {
		return errors.Wrap(UnknownSheet, strconv.Itoa(sheetIndex))
	}
	n := 0
	var need map[int]bool
	if len(columns) != 0 {
		need = make(map[int]bool, len(columns))
		for _, i := range columns {
			need[i] = true
		}
	}
	for i, row := range xlFile.GetRows(sheetName) {
		if i < skip {
			continue
		}
		if row == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := fn(sheetName, Row{Line: n, Values: row}); err != nil {
			return err
		}
		n++
	}
	return nil
}

func ReadXLSFile(ctx context.Context, fn func(string, Row) error, filename string, charset string, sheetIndex int, columns []int, skip int) error {
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
	if len(columns) != 0 {
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
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := fn(sheet.Name, Row{Line: int(n), Values: vals}); err != nil {
			return err
		}
	}
	return nil
}

func ReadCSV(ctx context.Context, fn func(Row) error, r io.Reader, delim string, columns []int, skip int) error {
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
		default:
		case <-ctx.Done():
			return ctx.Err()
		}
		if err := fn(Row{Line: n - 1, Values: row}); err != nil {
			return err
		}
	}
	return nil
}

type Row struct {
	Line   int
	Values []string
}

// vim: set noet fileencoding=utf-8:

// Copyright 2018 Tamás Gulácsi
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package pdfreport

import (
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/jung-kurt/gofpdf"
	errors "golang.org/x/xerrors"

	_ "github.com/tgulacsi/go/pdfreport/statik"
	"github.com/tgulacsi/statik/fs"
)

//go:generate sh -c "set -x; if [ -e $GOPATH/src/go.googlesource.com/image ]; then cd $GOPATH/src/go.googlesource.com/image && git pull; else mkdir -p $GOPATH/src/go.googlesource.com && cd $GOPATH/src/go.googlesource.com && git clone https://go.googlesource.com/image; fi"
//go:generate go build -o ./makefont github.com/jung-kurt/gofpdf/makefont
//go:generate mkdir -p assets/font
//go:generate sh -c "./makefont --embed --enc=$GOPATH/src/github.com/jung-kurt/gofpdf/font/cp1250.map --dst=./assets/font $GOPATH/src/go.googlesource.com/image/font/gofont/ttfs/*.ttf"
//go:generate go get github.com/tgulacsi/statik
//go:generate statik -f -src=assets

type Report struct {
	*gofpdf.Fpdf
	html          gofpdf.HTMLBasicType
	Encode        func(string) string
	FontSize, Ht  float64
	Bottom, Width float64
	Sans, Mono    string

	fontLoaders map[fontName]fontLoader
}

func (pdf *Report) Println(text string) error {
	pdf.html.Write(pdf.Ht, pdf.Encode(text))
	pdf.Ln(pdf.Ht + pdf.Ht/2)
	return pdf.Error()
}

func (pdf *Report) DefTable(keyvals ...string) error {
	//mLeft, _, _, _ := pdf.GetMargins()
	pdf.SetFillColor(230, 230, 230)
	for i := 0; i < len(keyvals); i += 2 {
		y := pdf.GetY()
		pdf.SetFont(pdf.Sans, "B", pdf.FontSize)
		pdf.MultiCell(45, 2*pdf.Ht, pdf.Encode(keyvals[i+0]), "1", "", i%4 == 0)
		pdf.SetFont(pdf.Sans, "", pdf.FontSize)
		pdf.SetXY(70, y)
		pdf.MultiCell(0, 2*pdf.Ht, pdf.Encode(keyvals[i+1]), "1", "", i%4 == 0)
	}
	return nil
}

func (pdf *Report) NewTable(names []string, sizes []int, headingSize, bodySize float64) Table {
	origSize, _ := pdf.GetFontSize()
	widths := make([]float64, len(names))
	if n := len(names) - len(sizes); n > 0 {
		sizes = append(sizes, make([]int, n)...)
	}
	width := pdf.Width
	var z int
	for i, s := range sizes {
		if s == 0 {
			z++
			continue
		}
		widths[i] = float64(s) * pdf.Width / 100
		width -= widths[i]
	}
	if z > 0 {
		w := width / float64(z)
		for i, f := range widths {
			if f == 0 {
				widths[i] = w
			}
		}
	}

	pdf.SetFillColor(230, 230, 230)
	size := headingSize
	if size == 0 {
		size = pdf.FontSize
	}
	pdf.SetFont(pdf.Sans, "B", size)
	y := pdf.GetY()
	for i, nm := range names {
		x := pdf.GetX()
		pdf.MultiCell(widths[i], 2*pdf.Ht, pdf.Encode(nm), "1", "", true)
		if i != len(names)-1 {
			pdf.SetXY(x+widths[i], y)
		}
	}
	size = bodySize
	if size == 0 {
		size = pdf.FontSize
	}
	pdf.SetFont(pdf.Sans, "", size)
	return Table{report: pdf, widths: widths, bodySize: size, origSize: origSize}
}

type Table struct {
	report             *Report
	Ht                 float64
	widths             []float64
	bodySize, origSize float64
}

func (t Table) Row(values []string) {
	if n := len(t.widths) - len(values); n > 0 {
		values = append(values, make([]string, n)...)
	}
	y := t.report.GetY()
	t.report.SetFontSize(t.bodySize)
	if y+2*t.report.Ht > t.report.Bottom {
		t.report.AddPage()
	}
	for i, v := range values {
		x := t.report.GetX()
		t.report.MultiCell(t.widths[i], 2*t.report.Ht, t.report.Encode(v), "1", "", false)
		if i != len(values)-1 {
			t.report.SetXY(x+t.widths[i], y)
		}
	}
	t.report.SetFontSize(t.origSize)
}

func (pdf *Report) Heading(level int, title string) error {
	pdf.Ln(pdf.Ht)
	pdf.SetFont(pdf.Sans, "U", float64(24-level*4))
	pdf.Fpdf.Write(float64(24-level*2), pdf.Encode(strings.TrimSpace(title))+"\n")
	pdf.SetFont(pdf.Sans, "", pdf.FontSize)

	return nil
}

var fonts map[fontName]fontLoader

type ReportParams struct {
	HeaderLogoLeft, HeaderLogoRight Image
	Title, Subject, Author, Creator string
	Footer                          string
}
type Image struct {
	io.Reader
	ImageType string
}

func NewReport(params ReportParams) *Report {
	pdf := &Report{Fpdf: gofpdf.New("P", "mm", "A4", ""), FontSize: 12}
	pdf.Ht = pdf.PointConvert(pdf.FontSize)
	pdf.html = pdf.HTMLBasicNew()
	pdf.Encode = pdf.UnicodeTranslatorFromDescriptor("cp1250")
	pdf.Sans = "Arial"
	pdf.Mono = "Courier"

	pdf.fontLoaders = make(map[fontName]fontLoader, len(fonts))
	for f, loader := range fonts {
		if strings.IndexByte(f.Name, '-') < 0 {
			pdf.Sans = f.Name
		}
		if strings.HasSuffix(f.Name, "-Mono") {
			pdf.Mono = f.Name
		}
		pdf.fontLoaders[f] = loader
	}

	pdf.SetFont(pdf.Sans, "", pdf.FontSize)
	if pdf.Err() {
		panic(pdf.Error())
	}
	pdf.AliasNbPages("")

	pdf.SetMargins(25, 25, 25)
	pdf.SetAutoPageBreak(true, 30)
	pdf.SetAuthor(params.Author, true)
	pdf.SetCreator(params.Creator, true)
	pdf.SetSubject(params.Subject, true)
	pdf.SetTitle(params.Title, true)
	mLeft, mTop, mRight, mBottom := pdf.GetMargins()
	pWidth, pHeight := pdf.GetPageSize()
	pdf.Width = pWidth - mLeft - mRight
	pdf.Bottom = pHeight - mBottom
	pCenterX, pCenterY := (pWidth-mLeft-mRight)/2+mLeft, (pHeight-mTop-mBottom)/2+mTop
	_ = pCenterY
	pdf.SetHeaderFunc(func() {
		if params.HeaderLogoLeft.Reader != nil {
			r, opts := params.HeaderLogoLeft.Reader, gofpdf.ImageOptions{ImageType: params.HeaderLogoLeft.ImageType, ReadDpi: true}
			if opts.ImageType == "" {
				opts.ImageType = "PNG"
				opts.ReadDpi = true
			}
			fn := "logo_left." + strings.ToLower(opts.ImageType)
			pdf.RegisterImageOptionsReader(fn, opts, r)
			pdf.ImageOptions(fn, mLeft, 10, 30, 0, false, opts, 0, "")
		}
		pdf.SetFontSize(16)
		// Calculate width of title and position
		wd := pdf.GetStringWidth(params.Title) + 6
		pdf.SetXY(pCenterX-wd/2, mTop-14)
		// Title
		pdf.Fpdf.WriteAligned(0, 10, pdf.Encode(params.Title), "C")

		if params.HeaderLogoRight.Reader != nil {
			r, opts := params.HeaderLogoRight.Reader, gofpdf.ImageOptions{ImageType: params.HeaderLogoRight.ImageType, ReadDpi: true}
			if opts.ImageType == "" {
				opts.ImageType = "PNG"
				opts.ReadDpi = true
			}
			fn := "logo_right." + strings.ToLower(opts.ImageType)
			pdf.RegisterImageOptionsReader(fn, opts, r)
			pdf.ImageOptions(fn, pWidth-mRight-15, 10, 15, 0, false, opts, 0, "")
		}
		pdf.SetXY(mLeft, mTop+pdf.Ht/2)
	})
	pdf.SetFooterFunc(func() {
		if params.Footer == "" {
			return
		}
		pdf.SetFontSize(10)
		wd := pdf.GetStringWidth(params.Footer)
		pdf.SetXY(pCenterX-wd/2, -25)
		pdf.Fpdf.WriteAligned(0, 10, pdf.Encode(params.Footer), "C")
	})

	if pdf.Err() {
		panic(pdf.Error())
	}

	return pdf
}

func (pdf *Report) SetFont(nm string, style string, size float64) {
	f := fontName{
		Name:   nm,
		Bold:   strings.IndexByte(style, 'B') >= 0,
		Italic: strings.IndexByte(style, 'I') >= 0,
	}
	if load := pdf.fontLoaders[f]; load != nil {
		delete(pdf.fontLoaders, f)
		load(pdf)
	}
	pdf.Fpdf.SetFont(nm, f.Style(), size)
}

func (pdf *Report) Output(w io.Writer) error {
	if wc, ok := w.(io.WriteCloser); ok {
		return pdf.Fpdf.OutputAndClose(wc)
	}
	return pdf.Fpdf.Output(w)
}
func (pdf *Report) Write(p []byte) (int, error) {
	pdf.Fpdf.Write(pdf.Ht, pdf.Encode(string(p)))
	return len(p), pdf.Error()
}

var statikFS http.FileSystem

func init() {
	var err error
	statikFS, err = fs.New()
	if err != nil {
		panic(err)
	}
	if fonts, err = newFontRepo("/font"); err != nil {
		panic(err)
	}
}

func getStatikFile(name string) ([]byte, error) {
	return fs.ReadFile(statikFS, name)
}

func newFontRepo(assetDir string) (map[fontName]fontLoader, error) {
	repo := make(map[fontName]fontLoader)
	err := fs.Walk(statikFS,
		assetDir,
		func(afn string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fn := path.Base(afn)
			if !(strings.HasPrefix(fn, "Go-") && strings.HasSuffix(fn, ".json")) {
				return nil
			}
			fJson, err := getStatikFile(afn)
			if err != nil {
				return errors.Errorf("%s: %w", fn, err)
			}
			zfn := strings.TrimSuffix(afn, ".json") + ".z"
			fZ, err := getStatikFile(zfn)
			if err != nil {
				return errors.Errorf("%s: %w", zfn, err)
			}
			nm := strings.TrimSuffix(fn, ".json")
			var f fontName
			if i := strings.Index(nm, "-Bold"); i >= 0 {
				nm = nm[:i] + nm[i+5:]
				f.Bold = true
			}
			if i := strings.Index(nm, "-Italic"); i >= 0 {
				nm = nm[:i] + nm[i+7:]
				f.Italic = true
			}
			if i := strings.Index(nm, "-Regular"); i >= 0 {
				nm = nm[:i] + nm[i+8:]
			}
			f.Name = nm

			repo[f] = func(pdf interface {
				AddFontFromBytes(string, string, []byte, []byte)
			}) {
				pdf.AddFontFromBytes(f.Name, f.Style(), fJson, fZ)
			}
			return nil
		},
	)
	return repo, err
}

type fontLoader func(interface {
	AddFontFromBytes(string, string, []byte, []byte) //nolint:megacheck
})

type fontName struct {
	Name         string
	Bold, Italic bool
}

func (f fontName) Style() string {
	var a [2]byte
	b := a[:0]
	if f.Bold {
		b = append(b, 'B')
	}
	if f.Italic {
		b = append(b, 'I')
	}
	return string(b)
}

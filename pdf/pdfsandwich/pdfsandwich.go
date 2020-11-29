/*
	process OCR on a pdf file;
	requires:
		- tesseract [depending on imagemagick and ghostscript]
		- unpaper
		- hocr2pdf (from ExactImage)

	(C) 2010-2018 Tobias Elze, modified for tesseract Heinrich Schwietering 2012
	patch for -rgb option contributed by James Cort
	patch for a parallel computing problem with Tesseract 4, Dominique Meeus, 2018

	Translate to Go, Tamás Gulácsi, 2020
*/

// SPDX-License-Identifier:	GPL-2.0-or-later

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/tgulacsi/go/globalctx"
	"golang.org/x/sync/errgroup"
)

var Version = "v0.1.7"

// default binary names:
var (
	globalTempDir string
	unpaper       = "unpaper"
	identify      = "identify"
	convert       = "convert"
	tesseract     = "tesseract"
	pdfinfo       = "pdfinfo"
	pdfunite      = "pdfunite"
	hocr2pdf      = "hocr2pdf"
	gs            = "gs"
)

// global flags:
var (
	verbose, quiet bool
)

// print output, if verbose option is set (default)
var logger = log.New(ioutil.Discard, "", log.LstdFlags)

func main() {
	if err := Main(); err != nil {
		logger.Fatalf("%+v", err)
	}
}

func makeTempFile(ext string) (string, error) {
	fh, err := ioutil.TempFile(globalTempDir, "pdfsandwich-*"+ext)
	if err != nil {
		return "", err
	}
	fh.Close()
	os.Remove(fh.Name())
	return fh.Name(), nil
}

// execute command cmd and print it's invocation line (if verbose is set):
func run(ctx context.Context, crash bool, prog string, args ...string) error {
	cmd := exec.CommandContext(ctx, prog, args...)
	logger.Print(cmd.Args)
	if err := cmd.Run(); err != nil {
		if crash {
			logger.Fatalf("%v: %+v", cmd.Args, err)
		}
		logger.Printf("%v: %+v", cmd.Args, err)
		return fmt.Errorf("%v: %w", cmd.Args, err)
	}
	return nil
}

// return number of pages of a PDF file; needs pdfinfo:
func numberOfPages(ctx context.Context, filename string) (int, error) {
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, pdfinfo, filename)
	cmd.Stdin = nil
	cmd.Stderr = os.Stderr
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("%v: %w", cmd.Args, err)
	}
	for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
		if bytes.Contains(line, []byte("Pages:")) {
			return strconv.Atoi(string(bytes.Map(func(r rune) rune {
				if '0' <= r && r <= '9' {
					return r
				}
				return -1
			}, line)))
		}
	}
	return 0, fmt.Errorf("Error: pdfinfo could not determine number of pages. Check the pdf input file.\n")
}

// process OCR on pdf file infile and save the results to outfile:
func ProcessOCR(ctx context.Context,
	infile, outfile string,
	firstPage, lastPage int,
	resolution int,
	rgb, gray bool,
	nThreads int,
	language string,
	convertopts []string,
	tessopts []string,
	hocropts []string,
	preprocess bool,
	unpaperopts []string,
	debug bool,
	enforcehocr2pdf bool,
	pageWidthHeight []int,
	maxpixels int,
) error {
	// let hocr_resolution = Str.global_replace (Str.regexp "^\\(.+\\)x.*$") "\\1" resolution in
	hocrResolution := fmt.Sprintf("%d", resolution)
	if nThreads < 1 {
		nThreads = 1
	}
	if nThreads > 1 {
		logger.Printf("Parallel processing with %d threads started. Processing page order may differ from original page order.\n", nThreads)
	}

	type todo struct {
		Page    int
		PDFName string
	}

	pages := make(chan todo, nThreads)
	grp, grpCtx := errgroup.WithContext(ctx)
	for i := 0; i < nThreads; i++ {
		ctx := grpCtx
		grp.Go(func() error {
			for todo := range pages {
				currPage, pdfname := todo.Page, todo.PDFName
				picExt := ".pbm"
				if rgb {
					picExt = ".ppm"
				} else if gray {
					picExt = ".pgm"
				}
				tmppicfile, err := makeTempFile(picExt)
				if err != nil {
					return err
				}
				tmptessinpfile, err := makeTempFile(".tif")
				if err != nil {
					return err
				}
				tmpocrfile, err := makeTempFile("")
				if err != nil {
					return err
				}
				tmpcolfigfile, err := makeTempFile("_col.png")
				if err != nil {
					return err
				}
				tmprescaled_infile, err := makeTempFile("_rescaled.pdf")
				if err != nil {
					return err
				}
				tmpunpaperfile, err := makeTempFile("_unpaper" + picExt)
				if err != nil {
					return err
				}
				if !debug {
					defer func() {
						for _, nm := range []string{tmpocrfile + ".pdf", tmprescaled_infile, tmptessinpfile, tmppicfile, tmpocrfile, tmpcolfigfile, tmpunpaperfile} {
							os.Remove(nm)
						}
					}()
				}
				if !quiet {
					logger.Printf("Processing page %d\n", currPage)
				}
				// get original height and width:
				cmd := exec.CommandContext(ctx, identify, "-format", "%w %h", fmt.Sprintf("%s[%d]", infile, currPage-1))
				var buf bytes.Buffer
				cmd.Stdout = &buf
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("%v: %w", cmd.Args, err)
				}
				lines := bytes.SplitN(buf.Bytes(), []byte("\n"), 2)
				parts := bytes.SplitN(lines[0], []byte(" "), 2)
				origWidth, err := strconv.Atoi(string(parts[0]))
				if err != nil {
					logger.Printf("%+v", err)
				}
				origHeight, err := strconv.Atoi(string(parts[1]))
				if err != nil {
					logger.Printf("%+v", err)
				}
				if origHeight == 0 || origWidth == 0 {
					logger.Printf("Warning: could not determine page size; defaulting to A4.")
					origHeight, origWidth = 842, 595
				}

				height, width := origHeight, origWidth
				if len(pageWidthHeight) == 2 {
					height, width = pageWidthHeight[1], pageWidthHeight[1]
				}

				// downscaling if resolution too large (requires gs):
				newHeight, newWidth := height, width
				fm := float64(maxpixels)
				fr := float64(resolution) / 72
				pixels := fr * fr * float64(width) * float64(height)
				if pixels > fm {
					mpix := fm / (fr * fr)
					k := float64(width) / float64(height)
					newWidth = int(math.Sqrt(k * mpix))
					newHeight = int(math.Sqrt(mpix) / math.Sqrt(k))
					logger.Printf(
						"\n\nWARNING: page size (%dx%d) of page %d together with resolution %d yields very large file which exceeds parameter maxpixels. Most probably, the input file was accidentally generated in an inappropriately hight resolution.\nThe input file is scaled down to %dx%d pixels instead.\nIf such a large input file is really required, set the command line option -maxpixels greater than %.0f instead.\n\n",
						width, height, currPage, resolution, newWidth, newHeight, pixels)
				}
				var convertInFile string
				if !(newHeight == origHeight && newWidth == origWidth) {
					page := strconv.Itoa(currPage)
					if err := run(ctx, true, gs, "-q", "-dNOPAUSE", "-dBATCH", "-sDEVICE=pdfwrite", "-dFirstPage="+page, "-dLastPage="+page,
						fmt.Sprintf("-dDEVICEWIDTHPOINTS=%d", newWidth), fmt.Sprintf("-dDEVICEHEIGHTPOINTS=%d", newHeight), "-dPDFFitPage", "-o", tmprescaled_infile, infile,
					); err != nil {
						return err
					}
					convertInFile = tmprescaled_infile
				} else {
					convertInFile = fmt.Sprintf("%s[%d]", infile, currPage-1)
				}

				convoptstmt := []string{"-type", "Bilevel", "-density"}
				if rgb || gray {
					convoptstmt = []string{"-depth", "8", "-background", "white", "-flatten", "-alpha", "Off", "-density"}
					if gray {
						convoptstmt = append(append(make([]string, 0, 2+len(convoptstmt)), "-colorspace", "gray"), convoptstmt...)
					}
				}
				if err := run(ctx, true, convert, append(append(append(append(make([]string, 0, 2+len(convertopts)+len(convoptstmt)+3),
					"-units", "PixelsPerInch"), convertopts...), convoptstmt...),
					fmt.Sprintf("%dx%d", resolution, resolution), convertInFile, tmppicfile)...,
				); err != nil {
					return err
				}
				preprocOutput := tmppicfile
				if preprocess {
					cmd := exec.CommandContext(ctx, unpaper, append(append(append(make([]string, 0, 1+len(unpaperopts)+2), "--overwrite"), unpaperopts...), tmppicfile, tmpunpaperfile)...)
					if verbose {
						cmd.Stderr = os.Stderr
					}
					if err := cmd.Run(); err != nil {
						return err
					}
					preprocOutput = tmpunpaperfile
				}

				// convert preprocessing output file to tif in order to ensure correct resolution and size:
				if err := run(ctx, true, convert, "-units", "PixelsPerInch", "-density", fmt.Sprintf("%dx%d", resolution, resolution), preprocOutput, tmptessinpfile); err != nil {
					return err
				}
				tessinputfile := tmptessinpfile

				// test if tesseract can output pdf files:
				if err := run(ctx, true, tesseract, append(append(append(make([]string, 0, 2+len(tessopts)+3), tessinputfile, tmpocrfile), tessopts...), "-l", language, "pdf")...); err != nil {
					return err
				}

				if !enforcehocr2pdf {
					if _, err := os.Stat(tmpocrfile + ".pdf"); err == nil {
						if verbose {
							logger.Printf("OCR pdf generated. Renaming output file to %q", pdfname)
						}
						if err = os.Rename(tmpocrfile+".pdf", pdfname); err != nil {
							return err
						}
					} else {
						if !quiet {
							logger.Printf("Tesseract was unable to produce a pdf output file. Possibly, version of tesseract is prior to 3.03 and cannot output pdf yet. Using hocr2pdf instead.")
						}
						if err := run(ctx, true, tesseract, append(append(append(make([]string, 0, 2+len(tessopts)+3), tessinputfile, tmpocrfile), tessopts...), "-l", language, "hocr")...); err != nil {
							return err
						}
						hocrinputfile := tmpocrfile + ".hocr"
						if _, err := os.Stat(tmpocrfile + ".html"); err == nil {
							hocrinputfile = tmpocrfile + ".html"
						}
						fh, err := os.Open(hocrinputfile)
						if err != nil {
							return err
						}
						cmd := exec.CommandContext(ctx, hocr2pdf, append(append(make([]string, 0, len(hocropts)+6), hocropts...), "-r", hocrResolution, "-i", tessinputfile, "-o", pdfname)...)
						cmd.Stdin = fh
						cmd.Stderr = os.Stderr
						err = cmd.Run()
						fh.Close()
						if !debug {
							os.Remove(hocrinputfile)
						}
						if err != nil {
							return err
						}
					}
				}
			}
			return nil
		})
	}

	pdfFiles := make([]string, 0, lastPage-firstPage+1)
	for i := firstPage; i <= lastPage; i++ {
		p := todo{Page: i}
		var err error
		if p.PDFName, err = makeTempFile(".pdf"); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case pages <- p:
			pdfFiles = append(pdfFiles, p.PDFName)
		}
	}
	close(pages)
	if !debug {
		defer func() {
			for _, nm := range pdfFiles {
				os.Remove(nm)
			}
		}()
	}

	if err := grp.Wait(); err != nil {
		return err
	}

	logger.Printf("OCR done. Writing %q", outfile)

	tmpoutfile := pdfFiles[0]
	if len(pdfFiles) > 1 {
		var err error
		tmpoutfile, err = makeTempFile(".pdf")
		if err != nil {
			return err
		}
		if err := run(ctx, true, pdfunite, append(append(make([]string, 0, len(pdfFiles)+1), pdfFiles...), tmpoutfile)...); err != nil {
			return err
		}
	}
	return os.Rename(tmpoutfile, outfile)
}

func Main() error {
	var (
		outputfile                                 string
		firstPage, lastPage, resolution, maxPixels = 1, 0, 300, 17415167
		lang                                       = "eng"
		rgb, gray                                  bool
		preprocess                                 = true
		unpaperopts                                stringsFlag
		layout                                     = "none"
		grayfilter                                 bool
		hocropts, convertopts, tessopts            stringsFlag
		nThreads                                   int
		debug                                      bool
		enforcehocr2pdf                            bool
		pagesize                                   = "original"
		// determins the number of threads tesseract may use for each page; if this is set to more than 1 it currently clashes with tesseract 4:
		ompThreadLimit = 1
	)

	fs := flag.NewFlagSet("pdfsandwich", flag.ContinueOnError)
	fs.StringVar(&convert, "convert", convert, "name of convert binary")
	fs.Var(&convertopts, "coo", "additional convert options; specify as many times as options: -coo -normalize -coo -black-threshold -coo 75%; call convert --help or man convert for all convert options")
	fs.BoolVar(&debug, "debug", false, "keep all temporary files in /tmp (for debugging)")
	fs.BoolVar(&enforcehocr2pdf, "enforcehocr2pdf", false, "use hocr2pdf even if tesseract >= 3.03")
	fs.IntVar(&firstPage, "first_page", firstPage, "number of page to start OCR from")
	fs.BoolVar(&grayfilter, "grayfilter", grayfilter, "enable unpaper's gray filter; further options can be set by -unpo")
	fs.BoolVar(&gray, "gray", gray, "use grayscale for images (default: black and white); will be overridden by use of rgb")
	fs.StringVar(&gs, "gs", gs, "name of gs binary; optional, only required for resizing")
	fs.StringVar(&hocr2pdf, "hocr2pdf", hocr2pdf, "name of hocr2pdf binary; ignored for tesseract >= 3.03 unless option -enforcehocr2pdf is set")
	fs.Var(&hocropts, "hoo", "additional hocr2pdf options")
	fs.StringVar(&identify, "identify", identify, "name of identify binary")
	fs.IntVar(&lastPage, "last_page", lastPage, "number of page up to which to process OCR")
	fs.StringVar(&lang, "lang", lang, "language of the text; option to tesseract e.g: eng, deu, deu-frak, fra, rus, swe, spa, ita, ...; see option -list_langs; multiple languages may be specified, separated by plus characters.")
	fs.StringVar(&layout, "layout", layout, "-layout { single | double | none } : layout of the scanned pages; requires unpaper; single: one page per sheet; double: two pages per sheet; none: no auto-layout")
	flagListLangs := fs.Bool("list-langs", false, "list currently available languages and exit")
	fs.IntVar(&maxPixels, "maxpixels", maxPixels, "maximal number of pixels allowed for input file; if (resolution/72)^2 *width*height > maxpixels then scale page of input file down prior to OCR so that page size in pixels corresponds to maxpixels")
	flagNoImage := fs.Bool("noimage", false, "do not place the image over the text (requires hocr2pdf; ignored without -enforcehocr2pdf option)")
	flagNoPreprocess := fs.Bool("nopreproc", !preprocess, "do not preprocess with unpaper")
	fs.IntVar(&nThreads, "nthreads", runtime.NumCPU(), "number of parallel threads")
	fs.StringVar(&outputfile, "o", outputfile, "output file; default: inputfile_ocr.pdf (if extension is different from .pdf, original extension is kept)")
	fs.IntVar(&ompThreadLimit, "omp_thread_limit", ompThreadLimit, "number of threads tesseract may use for each page; values greater than 1 may cause tesseract >=4 to hang up")
	fs.StringVar(&pagesize, "pagesize", pagesize, "set page size of output pdf (requires ghostscript); original: same as input file; NUMxNUM: width x height in pixel (e.g. for A4: -pagesize 595x842)")
	fs.StringVar(&pdfinfo, "pdfinfo", pdfinfo, "name of pdfinfo binary")
	fs.StringVar(&pdfunite, "pdfunite", pdfunite, "name of pdfunite binary")
	fs.IntVar(&resolution, "resolution", resolution, "resolution (dpi) used for OCR")
	fs.BoolVar(&rgb, "rgb", rgb, "use RGB color space for images (default: black and white); use with care: causes problems with some color spaces")
	flagSloppyText := fs.Bool("sloppy-text", false, "sloppily place text, group words, do not draw single glyphs; ignored for tesseract >= 3.03 unless option -enforcehocr2pdf is set")
	fs.StringVar(&tesseract, "tesseract", tesseract, "name of tesseract binary")
	fs.Var(&tessopts, "tesso", "additional tesseract options")
	fs.StringVar(&unpaper, "unpaper", unpaper, "name of unpaper binary")
	fs.Var(&unpaperopts, "unpo", "additional unpaper options")
	fs.BoolVar(&quiet, "quiet", quiet, "suppress output")
	fs.BoolVar(&verbose, "verbose", verbose, "produce more output")
	flagVersion := fs.Bool("version", false, "print version and quit")

	fs.Parse(os.Args[1:])

	logger = log.New(os.Stderr, "", log.LstdFlags)
	ctx, cancel := globalctx.Wrap(context.Background())
	defer cancel()
	var tesseractLangs []string
	{
		cmd := exec.CommandContext(ctx, tesseract, "--list-langs")
		b, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %w", cmd.Args, err)
		}
		tesseractLangs = strings.Split(string(b), "\n")
		var found bool
		for _, lang := range tesseractLangs {
			if found = lang == "eng"; found {
				break
			}
		}
		if !found {
			logger.Printf("Warning: tesseract option --list-langs not implemented. Cannot check languages. Make sure you have all necessary tesseract language packages installed.")
		}
	}

	if *flagListLangs {
		fmt.Println(tesseractLangs)
		return nil
	}
	if *flagVersion {
		fmt.Printf("pdfsandwich version %s\n", Version)
		return nil
	}
	if *flagNoImage {
		hocropts = append(hocropts, "-n")
	}
	if *flagSloppyText {
		hocropts = append(hocropts, "-s")
	}

	preprocess = !*flagNoPreprocess

	// generate global temporary directory:
	var err error
	if globalTempDir, err = ioutil.TempDir("", "pdfsandwich_tmp"); err != nil {
		return err
	}
	if !debug {
		defer os.RemoveAll(globalTempDir)
	}

	//check if binary bin exists (in search path):
	binaries := []string{convert, tesseract, gs, pdfunite}
	if enforcehocr2pdf {
		binaries = append(binaries, hocr2pdf)
	}
	if !grayfilter {
		unpaperopts = append(unpaperopts, "--no-grayfilter")
	}
	unpaperopts = append(unpaperopts, "--layout", layout)

	if preprocess {
		binaries = append(binaries, unpaper)
	}
	for _, bin := range binaries {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("could not find program %q. Make sure this program exists and can be found in your search path.\nUse command line options to specify a custom binary.", bin)
		}
	}

	argFilename := fs.Arg(0)
	if outputfile == "" {
		ext := filepath.Ext(argFilename)
		outputfile = argFilename[:len(ext)] + "_ocr" + ext
	}
	if outputfile == argFilename {
		outputfile += "_ocr"
	}
	if fh, err := os.Open(argFilename); err == nil {
		fh.Close()
	} else {
		return fmt.Errorf("open %q: %w", argFilename, err)
	}
	logger.Printf("pdfsandwich version: %s", Version)
	if verbose {
		// check versions of external programs:
		run(ctx, false, convert, "-version")
		if preprocess {
			run(ctx, false, unpaper, "-V")
		}
		if enforcehocr2pdf {
			run(ctx, false, hocr2pdf, "-h")
		}
		for _, nm := range []string{tesseract, gs, pdfinfo, pdfunite} {
			run(ctx, false, nm, "-v")
		}
	}

	var pageWidthHeight []int
	if i := strings.IndexByte(pagesize, 'x'); i > 0 {
		width, err := strconv.Atoi(pagesize[:i])
		if err != nil {
			return err
		}
		height, err := strconv.Atoi(pagesize[i+1:])
		if err != nil {
			return err
		}
		pageWidthHeight = []int{width, height}
	}

	// check if requested language is supported by tesseract:
	{
		langlist := strings.Split(lang, "+")
		for _, l := range langlist {
			var found bool
			for _, s := range tesseractLangs {
				if found = l == s; found {
					break
				}
			}
			if !found {
				return fmt.Errorf("language %q not supported by tesseract - make sure that the respective tesseract language package is installed", l)
			}
		}
	}

	inputfile := argFilename
	logger.Printf("Input file: %q", inputfile)
	logger.Printf("Output file: %q", outputfile)

	npages, err := numberOfPages(ctx, inputfile)
	if err != nil {
		return err
	}
	logger.Printf("Number of pages in inputfile: %d", npages)
	if firstPage < 1 || firstPage > npages {
		return fmt.Errorf("Value %d is invalid as first-page", firstPage)
	}
	if lastPage < 1 {
		lastPage = npages
	}
	if lastPage < firstPage || lastPage > npages {
		return fmt.Errorf("Value %d is invalid as last-page", lastPage)
	}
	if nThreads < 1 {
		nThreads = runtime.NumCPU()
	}
	// precede tesseract call with a thread limit to prevent hang ups for tesseract >=4:
	os.Setenv("OMP_THREAD_LIMIT", strconv.Itoa(ompThreadLimit))

	if err := ProcessOCR(ctx,
		inputfile,
		outputfile,
		firstPage, lastPage,
		resolution,
		rgb,
		gray,
		nThreads,
		lang,
		convertopts, tessopts, hocropts,
		preprocess, unpaperopts,
		debug,
		enforcehocr2pdf,
		pageWidthHeight,
		maxPixels); err != nil {
		return err
	}
	logger.Printf("%q generated", outputfile)
	return nil
}

type stringsFlag []string

func (ss stringsFlag) String() string      { return fmt.Sprintf("%v", ([]string)(ss)) }
func (ss *stringsFlag) Set(s string) error { *ss = append(*ss, s); return nil }

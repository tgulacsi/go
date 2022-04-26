// Copyright 2013, 2022 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package i18nmail

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/go-logr/logr"
	"github.com/sloonz/go-qprintable"
	"github.com/tgulacsi/go/iohlp"
)

// MaxWalkDepth is the maximum depth Walk will descend.
const (
	MaxWalkDepth  = 32
	bodyThreshold = 1 << 20
)

var (
	logger = logr.Discard()

	// CheckEncoding is true if we should check Base64 encodings
	CheckEncoding = true

	// SaveBadInput is true if we should save bad input
	SaveBadInput = false

	// ErrStopWalk shall be returned by the TodoFunc to stop the walk silently.
	ErrStopWalk = errors.New("stop the walk")
)

func SetLogger(lgr logr.Logger) { logger = lgr }

// TodoFunc is the type of the function called by Walk and WalkMultipart.
type TodoFunc func(mp MailPart) error

// sequence is a global sequence for numbering mail parts.
var sequence uint64

func nextSeq() uint64 {
	return atomic.AddUint64(&sequence, 1)
}
func nextSeqInt() int {
	return int(nextSeq() % uint64(1<<31))
}

// HashKeyName is the header key name for the hash
const HashKeyName = "X-HashOfFullMessage"

// MailPart is part of a mail or multipart message.
type MailPart struct {
	// Body of the part.
	Body *io.SectionReader
	// MediaType is the parsed media type.
	MediaType map[string]string
	// Header of the mail part.
	Header textproto.MIMEHeader
	// Parent of this part.
	Parent *MailPart
	// ContenType for the part.
	ContentType string
	// Level is the depth level.
	Level int
	// Seq is a sequence number
	Seq int
}

// String returns some string representation of the part.
func (mp MailPart) String() string {
	pseq := -1
	if mp.Parent != nil {
		pseq = mp.Parent.Seq
	}
	return fmt.Sprintf("%d:::{%s %s %s}", pseq, mp.ContentType, mp.MediaType, mp.Header)
}

// Spawn returns a descendant of the MailPart (Level+1, Parent=*mp, next sequence).
func (mp *MailPart) Spawn() MailPart {
	return MailPart{Parent: mp, Level: mp.Level + 1, Seq: nextSeqInt()}
}

// DecoderFunc is a type of a decoder (io.Reader wrapper)
type DecoderFunc func(io.Reader) io.Reader

// MakeSectionReader reads the reader and returns the byte slice.
//
// If the read length is below the threshold, then the bytes are read into memory;
// otherwise, a temp file is created, and mmap-ed.
func MakeSectionReader(r io.Reader, threshold int) (*io.SectionReader, error) {
	return iohlp.MakeSectionReader(r, threshold)
}

// WalkMessage walks over the parts of the email, calling todo on every part.
// The part.Body given to todo is reused, so read if you want to use it!
//
// By default this is recursive, except dontDescend is true.
func WalkMessage(msg *mail.Message, todo TodoFunc, dontDescend bool, parent *MailPart) error {
	msg.Header = DecodeHeaders(msg.Header)
	ct, params, decoder, err := getCT(msg.Header)
	if decoder != nil {
		msg.Body = decoder(msg.Body)
	}
	logger.V(1).Info("Walk message", "headers", msg.Header)
	if err != nil {
		return fmt.Errorf("WalkMail: %w", err)
	}
	if ct == "" {
		ct = "message/rfc822"
	}
	var level int
	if parent != nil {
		level = parent.Level
	}
	child := MailPart{
		ContentType: ct, MediaType: params,
		Header: textproto.MIMEHeader(msg.Header),
		Parent: parent,
		Level:  level + 1,
		Seq:    nextSeqInt(),
	}
	//fmt.Println("WM", child.Seq, "ct", child.ContentType)
	if hsh := msg.Header.Get("X-Hash"); hsh != "" && child.Header.Get(HashKeyName) == "" {
		child.Header.Add(HashKeyName, hsh)
	}
	child.Body, err = MakeSectionReader(msg.Body, bodyThreshold)
	if err != nil {
		return err
	}
	//debugf("message sequence=%d content-type=%q params=%v", child.Seq, ct, params)
	if strings.HasPrefix(ct, "multipart/") {
		if err = WalkMultipart(child, todo, dontDescend); err != nil {
			return fmt.Errorf("multipart: %w", err)
		}
		return nil
	}
	child.Body.Seek(0, 0)
	return todo(child)
}

// Walk over the parts of the email, calling todo on every part.
//
// By default this is recursive, except dontDescend is true.
func Walk(part MailPart, todo TodoFunc, dontDescend bool) error {
	h := sha1.New()
	if _, err := io.Copy(h, part.Body); err != nil {
		return err
	}
	part.Body.Seek(0, 0)
	msg, err := mail.ReadMessage(io.MultiReader(
		part.Body,
		bytes.NewReader([]byte("\r\n\r\n")),
	))
	hsh := base64.URLEncoding.EncodeToString(h.Sum(nil))
	if err != nil {
		b := make([]byte, 2048)
		n, _ := part.Body.ReadAt(b, 0)
		logger.Error(err, "ReadAndHashMessage", "message", string(b[:n]))
		return fmt.Errorf("WalkMail: %w", err)
	}
	if hsh != "" {
		msg.Header["X-Hash"] = []string{hsh}
	}
	// force a new SectionReader
	part.Body = io.NewSectionReader(part.Body, 0, part.Body.Size())
	return WalkMessage(msg, todo, dontDescend, &part)
}

// WalkMultipart walks a multipart/ MIME parts, calls todo on every part
// mp.Body is reused, so read if you want to use it!
//
// By default this is recursive, except dontDescend is true.
func WalkMultipart(mp MailPart, todo TodoFunc, dontDescend bool) error {
	boundary := mp.MediaType["boundary"]
	if boundary == "" {
		ct, params, _, ctErr := getCT(mp.Header)
		if ctErr != nil {
			return fmt.Errorf("getCT(%v): %w", mp.Header, ctErr)
		}
		if boundary = params["boundary"]; boundary != "" {
			mp.ContentType = ct
			mp.MediaType = params
		}
	}
	parts := multipart.NewReader(
		io.MultiReader(mp.Body, strings.NewReader("\r\n")),
		boundary)
	mp.Body = io.NewSectionReader(mp.Body, 0, mp.Body.Size())
	logger.Info("WalkMultipart", "seq", mp.Seq, "ct", mp.ContentType, "media", mp.MediaType)
	var err error
	var i int
	for {
		var part *multipart.Part
		if part, err = parts.NextPart(); err != nil {
			break
		}
		i++
		part.Header = DecodeHeaders(part.Header)
		var ct string
		ct, params, decoder, ctErr := getCT(part.Header)
		if ctErr != nil {
			return fmt.Errorf("%d.getCT(%v): %w", i, part.Header, ctErr)
		}
		body := io.Reader(part)
		if decoder != nil {
			body = decoder(part)
		}
		child := MailPart{
			ContentType: ct, MediaType: params,
			Header: part.Header,
			Parent: &mp,
			Level:  mp.Level + 1,
			Seq:    nextSeqInt(),
		}
		//fmt.Println(i, child.Seq, child.Header.Get("Content-Type"))
		child.Header.Add(HashKeyName, mp.Header.Get(HashKeyName))
		if child.Body, err = MakeSectionReader(body, bodyThreshold); err != nil {
			return err
		}
		if isMultipart := strings.HasPrefix(ct, "multipart/"); !dontDescend &&
			(isMultipart && child.MediaType["boundary"] != "" ||
				strings.HasPrefix(ct, "message/")) {
			if isMultipart {
				err = WalkMultipart(child, todo, dontDescend)
			} else {
				err = Walk(child, todo, dontDescend)
			}
			if err != nil {
				data := make([]byte, 1024)
				if n, readErr := child.Body.ReadAt(data, 0); readErr == io.EOF && n == 0 {
					err = nil
				} else {
					err = fmt.Errorf("descending data=%s: %w", data[:n], err)
					break
				}
			}
			child.Body.Seek(0, 0)
		} else {
			fn := part.FileName()
			if fn != "" {
				fn = HeadDecode(fn)
			}
			if fn == "" {
				ext, _ := mime.ExtensionsByType(child.ContentType)
				fn = fmt.Sprintf("%d.%d%s", child.Level, child.Seq, append(ext, ".dat")[0])
			}
			child.Header.Add("X-FileName", safeFn(fn, true))
			if err = todo(child); err != nil {
				return fmt.Errorf("todo(%q): %w", fn, err)
			}
		}
	}
	var eS string
	if err != nil {
		eS = err.Error()
		if err != io.EOF && !(strings.HasSuffix(eS, "EOF") || strings.Contains(eS, "multipart: expecting a new Part")) {
			logger.Error(err, "reading parts")
			var a [16 << 10]byte
			n, _ := mp.Body.ReadAt(a[:], 0)
			return fmt.Errorf("reading parts [media=%v body=%q]: %w", mp.MediaType, string(a[:n]), err)
		}
	}
	return nil
}

// returns the content-type, params and a decoder for the body of the multipart
func getCT(
	header map[string][]string,
) (
	contentType string,
	params map[string]string,
	decoder func(io.Reader) io.Reader,
	err error,
) {
	decoder = func(r io.Reader) io.Reader {
		return r
	}
	hdr := mail.Header(header)
	contentType = hdr.Get("Content-Type")
	//infof("getCT ct=%q", contentType)
	if contentType == "" {
		return
	}
	var nct string
	nct, params, err = mime.ParseMediaType(contentType)
	//infof("getCT mediaType=%v; %v (%+v)", nct, params, err)
	if err != nil {
		// Guess from filename's extension
		cd := hdr.Get("Content-Disposition")
		var ok bool
		if cd != "" {
			cd, cdParams, _ := mime.ParseMediaType(cd)
			if params == nil {
				params = cdParams
			} else {
				for k, v := range cdParams {
					if _, occupied := params[k]; !occupied {
						params[k] = v
					}
				}
			}
			if cd != "" {
				if ext := filepath.Ext(cdParams["filename"]); ext != "" {
					if nct = mime.TypeByExtension(ext); nct == "" {
						nct = "application/octet-stream"
					}
				}
				ok = true
			}
		}
		if !ok {
			err = fmt.Errorf("cannot parse Content-Type %s: %w", contentType, err)
			return
		}
		err = nil
		if nct == "" {
			nct = "application/octet-stream"
		}
	}
	contentType = nct
	te := strings.ToLower(hdr.Get("Content-Transfer-Encoding"))
	switch te {
	case "", "7bit", "8bit", "binary":
		// https://stackoverflow.com/questions/25710599/content-transfer-encoding-7bit-or-8-bit
	case "base64":
		decoder = func(r io.Reader) io.Reader {
			//return &b64ForceDecoder{Encoding: base64.StdEncoding, r: r}
			//return B64FilterReader(r, base64.StdEncoding)
			return NewB64Decoder(base64.StdEncoding, r)
		}
	case "quoted-printable":
		decoder = func(r io.Reader) io.Reader {
			br := bufio.NewReaderSize(r, 1024)
			first, _ := br.Peek(1024)
			enc := qprintable.BinaryEncoding
			if len(first) > 0 {
				enc = qprintable.DetectEncoding(string(first))
			}
			return qprintable.NewDecoder(enc, br)
		}
	default:
		logger.Info("unknown", "transfer-encoding", te)
	}
	return
}

// HashBytes returns a hash (sha1 atm) for the given bytes
func HashBytes(data []byte) string {
	h := sha1.New()
	_, _ = h.Write(data)
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

func safeFn(fn string, maskPercent bool) string {
	fn = url.QueryEscape(
		strings.Replace(strings.Replace(fn, "/", "-", -1),
			`\`, "-", -1))
	if maskPercent {
		fn = strings.Replace(fn, "%", "!P!", -1)
	}
	return fn
}

// DecodeHeaders decodes the headers.
func DecodeHeaders(hdr map[string][]string) map[string][]string {
	for k, vv := range hdr {
		for i, v := range vv {
			vv[i] = HeadDecode(v)
		}
		hdr[k] = vv
	}
	return hdr
}

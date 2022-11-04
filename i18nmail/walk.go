// Copyright 2013, 2022 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package i18nmail

import (
	"bufio"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"

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
type TodoFunc func(mp mailPart) error

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

// mailPart is part of a mail or multipart message.
type mailPart struct {
	entity *message.Entity
	body   *io.SectionReader
	Header textproto.MIMEHeader
	// MediaType is the parsed media type.
	MediaType map[string]string
	// Parent of this part.
	Parent *mailPart
	// ContenType for the part.
	ContentType string
	// Level is the depth level.
	Level int
	// Seq is a sequence number
	Seq int
}

func NewMailPart(r io.Reader) (mailPart, error) {
	var mp mailPart
	msg, err := message.Read(r)
	if err != nil {
		return mp, err
	}
	return mp.WithEntity(msg)
}

// Entity returns the part as a *message.Entity
func (mp mailPart) Entity() *message.Entity {
	return mp.entity
}

// String returns some string representation of the part.
func (mp mailPart) String() string {
	pseq := -1
	if mp.Parent != nil {
		pseq = mp.Parent.Seq
	}
	return fmt.Sprintf("%d:::{%s %s %s}", pseq, mp.ContentType, mp.MediaType, mp.Header)
}

// Spawn returns a descendant of the mailPart (Level+1, Parent=*mp, next sequence).
func (mp mailPart) Spawn() mailPart {
	return mailPart{Parent: &mp, Level: mp.Level + 1, Seq: nextSeqInt()}
}
func (mp mailPart) GetBody() *io.SectionReader {
	return io.NewSectionReader(mp.body, 0, mp.body.Size())
}
func (mp mailPart) WithEntity(entity *message.Entity) (mailPart, error) {
	mp.entity = entity
	hdr := mp.entity.Header
	ct, params, decoder, err := getCTdecoder(hdr.Get("Content-Type"), hdr.Get("Content-Disposition"), hdr.Get("Content-Transfer-Encoding"))
	if err != nil {
		return mp, err
	}
	if ct == "" {
		ct = "message/rfc822"
	}
	mp.ContentType, mp.MediaType = ct, params
	hsh := sha512.New512_224()
	mp.entity.Body = io.TeeReader(mp.entity.Body, hsh)
	if decoder != nil {
		mp.entity.Body = decoder(mp.entity.Body)
	}
	body, err := MakeSectionReader(mp.entity.Body, bodyThreshold)
	if err != nil {
		return mp, err
	}
	var a [sha512.Size224]byte

	mp.body, mp.entity.Body = body, io.NewSectionReader(body, 0, body.Size())
	if !mp.entity.Header.Has(HashKeyName) {
		mp.entity.Header.Set(HashKeyName, base64.URLEncoding.EncodeToString(hsh.Sum(a[:0])))
	}

	fields := mp.entity.Header.Fields()
	m := make(map[string][]string, fields.Len())
	for fields.Next() {
		k := textproto.CanonicalMIMEHeaderKey(fields.Key())
		s, err := fields.Text()
		if err != nil {
			b, _ := fields.Raw()
			s = string(b)
		}
		m[k] = append(m[k], s)
	}
	mp.Header = textproto.MIMEHeader(m)
	return mp, err
}

func (mp mailPart) FileName() string {
	_, params, _ := mp.entity.Header.ContentDisposition()
	return params["filename"]
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
func WalkMessage(msg *message.Entity, todo TodoFunc, dontDescend bool, parent *mailPart) error {
	var child mailPart
	if parent != nil {
		child = parent.Spawn()
	} else {
		child.Level = 1
		child.Seq = nextSeqInt()
	}
	var err error
	if child, err = child.WithEntity(msg); err != nil {
		return err
	}
	//debugf("message sequence=%d content-type=%q params=%v", child.Seq, ct, params)
	if strings.HasPrefix(child.ContentType, "multipart/") {
		if err = WalkMultipart(child, todo, dontDescend); err != nil {
			return fmt.Errorf("WalkMultipart(seq=%d, ct=%q): %w", child.Seq, child.ContentType, err)
		}
		return nil
	}
	return todo(child)
}

// Walk over the parts of the email, calling todo on every part.
//
// By default this is recursive, except dontDescend is true.
func Walk(part mailPart, todo TodoFunc, dontDescend bool) error {
	return part.entity.Walk(func(path []int, entity *message.Entity, err error) error {
		if err != nil {
			return err
		}
		mp, err := part.Spawn().WithEntity(entity)
		if err != nil {
			return err
		}
		return todo(mp)
	})
}

// WalkMultipart walks a multipart/ MIME parts, calls todo on every part
// mp.Body is reused, so read if you want to use it!
//
// By default this is recursive, except dontDescend is true.
func WalkMultipart(mp mailPart, todo TodoFunc, dontDescend bool) error {
	parts := mp.entity.MultipartReader()
	var err error
	var i int
	for {
		var part *message.Entity
		if part, err = parts.NextPart(); err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			} else {
				err = fmt.Errorf("NextPart: %w", err)
			}
			break
		}
		i++
		child, entErr := mailPart{
			Parent: &mp,
			Level:  mp.Level + 1,
			Seq:    nextSeqInt(),
		}.WithEntity(part)
		if entErr != nil {
			err = entErr
			break
		}
		if isMultipart := strings.HasPrefix(child.ContentType, "multipart/"); !dontDescend &&
			(isMultipart && child.MediaType["boundary"] != "" ||
				strings.HasPrefix(child.ContentType, "message/")) {
			if isMultipart {
				err = WalkMultipart(child, todo, dontDescend)
			} else {
				err = Walk(child, todo, dontDescend)
			}
			if err != nil {
				data := make([]byte, 1024)
				if n, readErr := child.body.ReadAt(data, 0); readErr == io.EOF && n == 0 {
					err = nil
				} else {
					err = fmt.Errorf("descending data=%s: %w", data[:n], err)
					break
				}
			}
			child.body.Seek(0, 0)
		} else {
			fn := child.FileName()
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
	if err != nil && !errors.Is(err, io.EOF) {
		eS = err.Error()
		if !(strings.HasSuffix(eS, "EOF") || strings.Contains(eS, "multipart: expecting a new Part")) {
			logger.Error(err, "reading parts")
			var a [16 << 10]byte
			n, _ := mp.body.ReadAt(a[:], 0)
			return fmt.Errorf("reading parts [media=%v body=%q]: %w", mp.MediaType, string(a[:n]), err)
		}
	}
	return nil
}

// returns the content-type, params and a decoder for the body of the multipart
func getCTdecoder(
	Type, contentDisposition, contentTransferEncoding string,
) (
	contentType string,
	params map[string]string,
	decoder func(io.Reader) io.Reader,
	err error,
) {
	decoder = func(r io.Reader) io.Reader {
		return r
	}
	contentType = Type
	//infof("getCT ct=%q", contentType)
	if contentType == "" {
		return
	}
	var nct string
	nct, params, err = mime.ParseMediaType(contentType)
	//infof("getCT mediaType=%v; %v (%+v)", nct, params, err)
	if err != nil {
		// Guess from filename's extension
		cd := contentDisposition
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
	te := strings.ToLower(contentTransferEncoding)
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

// HashBytes returns a hash (sha512_224 atm) for the given bytes
func HashBytes(data []byte) string {
	h := sha512.New512_224()
	_, _ = h.Write(data)
	var a [sha512.Size224]byte
	return base64.URLEncoding.EncodeToString(h.Sum(a[:0]))
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

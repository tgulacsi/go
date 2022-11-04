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
	"strings"
	"sync/atomic"

	"github.com/emersion/go-message"
	tp "github.com/emersion/go-message/textproto"

	// charsets
	_ "github.com/emersion/go-message/charset"

	"github.com/go-logr/logr"
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

// SetLogger sets the global logger
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
	entity *message.Entity
	body   *io.SectionReader
	Header textproto.MIMEHeader
	// MediaType is the parsed media type.
	MediaType map[string]string
	// Parent of this part.
	Parent *MailPart
	// ContenType for the part.
	ContentType string
	// Level is the depth level.
	Level int
	// Seq is a sequence number
	Seq int
}

// NewMailPart parses the io.Reader as a full email message and returns it as a MailPart.
func NewMailPart(r io.Reader) (MailPart, error) {
	var mp MailPart
	msg, err := message.Read(r)
	if err != nil {
		return mp, err
	}
	return mp.WithEntity(msg)
}

// Entity returns the part as a *message.Entity
func (mp MailPart) Entity() *message.Entity {
	return mp.entity
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
func (mp MailPart) Spawn() MailPart {
	return MailPart{Parent: &mp, Level: mp.Level + 1, Seq: nextSeqInt()}
}

// GetBody returns an intact *io.SectionReader of the body.
func (mp MailPart) GetBody() *io.SectionReader {
	return io.NewSectionReader(mp.body, 0, mp.body.Size())
}

// NewEntity returns a new *message.Entity from the header map and body readers.
func NewEntity(header map[string][]string, body io.Reader) (*message.Entity, error) {
	hdr := message.HeaderFromMap(header)
	return message.New(fixHeader(hdr), body)
}

// NewEntityFromReaders returns a new *message.Entity from the header and body readers.
func NewEntityFromReaders(header, body io.Reader) (*message.Entity, error) {
	hdr, err := tp.ReadHeader(bufio.NewReader(io.LimitReader(header, 1<<20)))
	if err != nil {
		return nil, err
	}
	return message.New(fixHeader(message.Header{Header: hdr}), body)
}

func fixHeader(hdr message.Header) message.Header {
	if cte := hdr.Get("Content-Transfer-Encoding"); cte != "" && strings.ToLower(cte) != cte {
		hdr.Set("Content-Transfer-Encoding", cte)
	}
	return hdr
}

// WithReader returns a MailPart parsing the io.Reader as a full email.
func (mp MailPart) WithReader(r io.Reader) (MailPart, error) {
	entity, err := message.Read(r)
	if err != nil {
		return mp, err
	}
	return mp.WithEntity(entity)
}

// WithBody replaces only the body part.
func (mp MailPart) WithBody(r io.Reader) (MailPart, error) {
	entity := *mp.entity
	entity.Body = r
	return mp.WithEntity(&entity)
}

// WithEntity populates MailPart with the parsed *message.Entity.
func (mp MailPart) WithEntity(entity *message.Entity) (MailPart, error) {
	mp.entity = entity
	hdr := mp.entity.Header
	ct, params, err := hdr.ContentType()
	if err != nil {
		return mp, err
	}
	if ct == "" {
		ct = "message/rfc822"
	}
	mp.ContentType, mp.MediaType = ct, params
	hsh := sha512.New512_224()
	mp.entity.Body = io.TeeReader(mp.entity.Body, hsh)
	body, err := MakeSectionReader(mp.entity.Body, bodyThreshold)
	if err != nil {
		return mp, fmt.Errorf("part %s: %w", mp, err)
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

// FileName returns the file name from the Content-Disposition header.
func (mp MailPart) FileName() string {
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
func WalkMessage(msg *message.Entity, todo TodoFunc, dontDescend bool, parent *MailPart) error {
	var child MailPart
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
func Walk(part MailPart, todo TodoFunc, dontDescend bool) error {
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
func WalkMultipart(mp MailPart, todo TodoFunc, dontDescend bool) error {
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
		child, entErr := MailPart{
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

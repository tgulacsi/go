// Copyright 2013 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package i18nmail

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
	"github.com/sloonz/go-qprintable"
	"github.com/tgulacsi/go/temp"
)

const MaxWalkDepth = 32

var (
	// Debugf prints debug logs. Nil walue prints nothing.
	Debugf func(string, ...interface{})

	// Infof prints informative logs. Nil walue prints nothing.
	Infof func(string, ...interface{})

	// CheckEncoding is true if we should check Base64 encodings
	CheckEncoding = true

	// SaveBadInput is true if we should save bad input
	SaveBadInput = false

	// ErrStop
	ErrStopWalk = errors.New("Stop the walk")
)

// TodoFunc is the type of the function called by Walk and WalkMultipart.
type TodoFunc func(mp MailPart) error

func debugf(pattern string, args ...interface{}) {
	if Debugf == nil {
		return
	}
	Debugf(pattern, args...)
}
func infof(pattern string, args ...interface{}) {
	if Infof == nil {
		return
	}
	Infof(pattern, args...)
}

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
	// ContenType for the part.
	ContentType string
	// MediaType is the parsed media type.
	MediaType map[string]string
	// Header of the mail part.
	Header textproto.MIMEHeader
	// Body of the part.
	Body io.Reader
	// Parent of this part.
	Parent *MailPart
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

// Walk over the parts of the email, calling todo on every part.
// The part.Body given to todo is reused, so read if you want to use it!
//
// By default this is recursive, except dontDescend is true.
func Walk(part MailPart, todo TodoFunc, dontDescend bool) error {
	var (
		msg *mail.Message
		hsh string
	)
	br, e := temp.NewReadSeeker(part.Body)
	if e != nil {
		return e
	}
	defer func() { _ = br.Close() }()
	if msg, hsh, e = ReadAndHashMessage(br); e != nil {
		if p, _ := br.Seek(0, 2); p == 0 {
			infof("empty body!")
			return nil
		}
		br.Seek(0, 0)
		b := make([]byte, 4096)
		n, _ := io.ReadAtLeast(br, b, 2048)
		infof("ReadAndHashMessage: %v\n%s", e, string(b[:n]))
		return errors.Wrapf(e, "WalkMail")
	}
	msg.Header = DecodeHeaders(msg.Header)
	ct, params, decoder, e := getCT(msg.Header)
	if decoder != nil {
		msg.Body = decoder(msg.Body)
	}
	debugf("Walk message hsh=%s headers=%q level=%d", hsh, msg.Header, part.Level)
	if e != nil {
		return errors.Wrapf(e, "WalkMail")
	}
	if ct == "" {
		ct = "message/rfc822"
	}
	child := MailPart{ContentType: ct, MediaType: params,
		Header: textproto.MIMEHeader(msg.Header),
		Body:   msg.Body,
		Parent: &part,
		Level:  part.Level + 1,
		Seq:    nextSeqInt()}
	if hsh != "" {
		child.Header.Add("X-Hash", hsh)
	}
	if child.Header.Get(HashKeyName) == "" {
		child.Header.Add(HashKeyName, hsh)
	}
	//debugf("message sequence=%d content-type=%q params=%v", child.Seq, ct, params)
	if strings.HasPrefix(ct, "multipart/") {
		if e = WalkMultipart(child, todo, dontDescend); e != nil {
			return errors.Wrapf(e, "multipart")
		}
		return nil
	}
	if e = todo(child); e != nil {
		return e
	}
	return nil
}

// WalkMultipart walks a multipart/ MIME parts, calls todo on every part
// mp.Body is reused, so read if you want to use it!
//
// By default this is recursive, except dontDescend is true.
func WalkMultipart(mp MailPart, todo TodoFunc, dontDescend bool) error {
	parts := multipart.NewReader(io.MultiReader(mp.Body, strings.NewReader("\r\n")), mp.MediaType["boundary"])
	part, e := parts.NextPart()
	var (
		decoder DecoderFunc
		body    io.Reader
		params  map[string]string
		ct      string
	)
	for i := 1; e == nil; i++ {
		part.Header = DecodeHeaders(part.Header)
		if ct, params, decoder, e = getCT(part.Header); e != nil {
			return errors.Wrapf(e, "%d.getCT(%v)", i, part.Header)
		}
		if decoder != nil {
			body = decoder(part)
		} else {
			body = part
		}
		child := MailPart{ContentType: ct, MediaType: params,
			Body:   body,
			Header: part.Header,
			Parent: &mp,
			Level:  mp.Level + 1,
			Seq:    nextSeqInt()}
		child.Header.Add(HashKeyName, mp.Header.Get(HashKeyName))
		if !dontDescend && strings.HasPrefix(ct, "multipart/") {
			if e = WalkMultipart(child, todo, dontDescend); e != nil {
				br := bufio.NewReader(body)
				child.Body = br
				data, _ := br.Peek(1024)
				if len(data) == 0 { // EOF
					e = nil
					break
				}
				return errors.Wrapf(e, fmt.Sprintf("descending data=%s", data))
			}
		} else if !dontDescend && strings.HasPrefix(ct, "message/") {
			if e = Walk(child, todo, dontDescend); e != nil {
				br := bufio.NewReader(body)
				child.Body = br
				data, _ := br.Peek(1024)
				return errors.Wrapf(e, fmt.Sprintf("descending data=%s", data))
			}
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
			if e = todo(child); e != nil {
				return errors.Wrapf(e, "todo(%q)", fn)
			}
		}

		part, e = parts.NextPart()
	}
	var eS string
	if e != nil {
		eS = e.Error()
	}
	if e != nil && e != io.EOF && !(strings.HasSuffix(eS, "EOF") || strings.Contains(eS, "multipart: expecting a new Part")) {
		infof("ERROR reading parts: %v", e)
		return errors.Wrapf(e, "reading parts")
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
	contentType = mail.Header(header).Get("Content-Type")
	if contentType == "" {
		return
	}
	var nct string
	nct, params, err = mime.ParseMediaType(contentType)
	if err != nil {
		err = errors.Wrapf(err, "cannot parse Content-Type %s", contentType)
		return
	}
	contentType = nct
	te := strings.ToLower(mail.Header(header).Get("Content-Transfer-Encoding"))
	switch te {
	case "":
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
		infof("unknown transfer-encoding %q", te)
	}
	return
}

// HashBytes returns a hash (sha1 atm) for the given bytes
func HashBytes(data []byte) string {
	h := sha1.New()
	_, _ = h.Write(data)
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

var bufPool = sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 4096)) }}

// ReadAndHashMessage reads message and hashes it by the way
func ReadAndHashMessage(r io.Reader) (*mail.Message, string, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)
	h := sha1.New()
	m, e := mail.ReadMessage(io.MultiReader(
		io.TeeReader(r, buf),
		bytes.NewReader([]byte("\r\n\r\n")),
	))
	if e != nil && m == nil {
		infof("ERROR ReadMessage: %v", e)
		return nil, "", e
	}
	h.Write(bytes.TrimSpace(buf.Bytes()))
	return m, base64.URLEncoding.EncodeToString(h.Sum(nil)), nil
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

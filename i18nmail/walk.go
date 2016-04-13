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
	"sync/atomic"

	"github.com/rs/xlog"
	"github.com/sloonz/go-qprintable"
	"github.com/tgulacsi/go/temp"
	"gopkg.in/errgo.v1"
)

const MaxWalkDepth = 32

var (
	// Logger is the base logger, can be swapped - defaults to NopLogger.
	Log = xlog.Logger(xlog.NopLogger)

	// CheckEncoding is true if we should check Base64 encodings
	CheckEncoding = true

	// SaveBadInput is true if we should save bad input
	SaveBadInput = false

	// ErrStop
	ErrStopWalk = errgo.New("Stop the walk")
)

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

func errIsStopWalk(err error) bool { return err == ErrStopWalk }

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
			Log.Warn("empty body!")
			return nil
		}
		br.Seek(0, 0)
		b := make([]byte, 4096)
		n, _ := io.ReadAtLeast(br, b, 2048)
		Log.Warnf("ReadAndHashMessage: %v\n%s", e, string(b[:n]))
		return errgo.Notef(e, "WalkMail")
	}
	msg.Header = DecodeHeaders(msg.Header)
	ct, params, decoder, e := getCT(msg.Header)
	if decoder != nil {
		msg.Body = decoder(msg.Body)
	}
	Log.Infof("Walk message hsh=%s headers=%q level=%d", hsh, msg.Header, part.Level)
	if e != nil {
		return errgo.Notef(e, "WalkMail")
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
	//Log.Debugf("message sequence=%d content-type=%q params=%v", child.Seq, ct, params)
	if strings.HasPrefix(ct, "multipart/") {
		if e = WalkMultipart(child, todo, dontDescend); e != nil {
			return errgo.Notef(e, "multipart")
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
			return errgo.Notef(e, "%d.getCT(%v)", i, part.Header)
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
				data, _ := br.Peek(1024)
				if len(data) == 0 { // EOF
					e = nil
					break
				}
				return errgo.NoteMask(e, fmt.Sprintf("descending data=%s", data), errIsStopWalk)
			}
		} else if !dontDescend && strings.HasPrefix(ct, "message/") {
			if e = Walk(child, todo, dontDescend); e != nil {
				br := bufio.NewReader(body)
				data, _ := br.Peek(1024)
				return errgo.NoteMask(e, fmt.Sprintf("descending data=%s", data), errIsStopWalk)
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
				return errgo.Notef(e, "todo(%q)", fn)
			}
		}

		part, e = parts.NextPart()
	}
	var eS string
	if e != nil {
		eS = e.Error()
	}
	if e != nil && e != io.EOF && !(strings.HasSuffix(eS, "EOF") || strings.Contains(eS, "multipart: expecting a new Part")) {
		Log.Errorf("reading parts: %v", e)
		return errgo.NoteMask(e, "reading parts", errIsStopWalk)
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
		err = errgo.Newf("cannot parse Content-Type %s: %s", contentType, err)
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
		Log.Warnf("unknown transfer-encoding %q", te)
	}
	return
}

// HashBytes returns a hash (sha1 atm) for the given bytes
func HashBytes(data []byte) string {
	h := sha1.New()
	_, _ = h.Write(data)
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

// ReadAndHashMessage reads message and hashes it by the way
func ReadAndHashMessage(r io.Reader) (*mail.Message, string, error) {
	h := sha1.New()
	m, e := mail.ReadMessage(io.TeeReader(
		io.MultiReader(r, bytes.NewReader([]byte("\r\n\r\n"))),
		h))
	if e != nil && m == nil {
		Log.Errorf("ReadMessage: %v", e)
		return nil, "", e
	}
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

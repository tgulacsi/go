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
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/sloonz/go-qprintable"
	"github.com/tgulacsi/go/temp"
	"gopkg.in/errgo.v1"
	"gopkg.in/kit.v0/log"
	"gopkg.in/kit.v0/log/levels"
)

var (
	// Logger is the base logger, can be swapped - defaults to NopLogger.
	Logger = new(log.SwapLogger)

	// CheckEncoding is true if we should check Base64 encodings
	CheckEncoding = true

	// SaveBadInput is true if we should save bad input
	SaveBadInput = false

	// logger is the package-level logger.
	logger = levels.New(log.NewContext(Logger).With("lib", "i18nmail"))
)

func init() {
	Logger.Swap(log.NewNopLogger())
}

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
	br, e := temp.NewReadSeeker(part.Body)
	if e != nil {
		return e
	}
	defer func() { _ = br.Close() }()
	msg, hsh, e := ReadAndHashMessage(br)
	if e != nil {
		return errgo.Notef(e, "WalkMail")
	}
	ct, params, decoder, e := getCT(msg.Header)
	logger.Info("msg", "Walk message", "hsh", hsh, "headers", msg.Header)
	if e != nil {
		return errgo.Notef(e, "WalkMail")
	}
	if ct == "" {
		ct = "message/rfc822"
	}
	child := MailPart{ContentType: ct, MediaType: params,
		Header: textproto.MIMEHeader(msg.Header),
		Body:   msg.Body, Parent: &part,
		Level: part.Level + 1,
		Seq:   nextSeqInt()}
	child.Header.Add("X-Hash", hsh)
	if child.Header.Get(HashKeyName) == "" {
		child.Header.Add(HashKeyName, hsh)
	}
	logger.Debug("msg", "message", "sequence", child.Seq, "content-type", ct, "params", params)
	if strings.HasPrefix(ct, "multipart/") {
		return WalkMultipart(child, todo, dontDescend)
	}
	if !dontDescend && strings.HasPrefix(ct, "message/") { //mail
		if decoder != nil {
			child.Body = decoder(child.Body)
		}
		if e = Walk(child, todo, dontDescend); e != nil {
			return errgo.Notef(e, "WalkMail descending")
		}
		return nil
	}
	//simple
	if decoder != nil {
		child.Body = decoder(child.Body)
	}
	if e = todo(child); e != nil {
		return errgo.Notef(e, "todo")
	}
	return nil
}

// WalkMultipart walks a multipart/ MIME parts, calls todo on every part
// mp.Body is reused, so read if you want to use it!
//
// By default this is recursive, except dontDescend is true.
func WalkMultipart(mp MailPart, todo TodoFunc, dontDescend bool) error {
	parts := multipart.NewReader(mp.Body, mp.MediaType["boundary"])
	part, e := parts.NextPart()
	var (
		decoder DecoderFunc
		body    io.Reader
		params  map[string]string
		ct      string
	)
	for i := 1; e == nil; i++ {
		if ct, params, decoder, e = getCT(part.Header); e != nil {
			return e
		}
		//log.Printf("part ct=%s decoder=%v", ct, decoder)
		if decoder != nil {
			body = decoder(part)
		} else {
			body = part
		}
		child := MailPart{ContentType: ct, MediaType: params, Body: body,
			Header: part.Header, Parent: &mp,
			Level: mp.Level + 1, Seq: nextSeqInt()}
		child.Header.Add(HashKeyName, mp.Header.Get(HashKeyName))
		logger.Debug("msg", "multipart", "sequence", child.Seq, "content-type", ct, "params", params)
		if !dontDescend && strings.HasPrefix(ct, "multipart/") {
			if e = WalkMultipart(child, todo, dontDescend); e != nil {
				br := bufio.NewReader(body)
				data, _ := br.Peek(1024)
				return fmt.Errorf("error while descending: %s\ndata=%s", e, data)
			}
		} else if !dontDescend && strings.HasPrefix(ct, "message/") {
			if e = Walk(child, todo, dontDescend); e != nil {
				br := bufio.NewReader(body)
				data, _ := br.Peek(1024)
				return fmt.Errorf("error while descending: %s\ndata=%s", e, data)
			}
		} else {
			child.Header.Add("X-FileName", safeFn(HeadDecode(part.FileName()), true))
			if e = todo(child); e != nil {
				return e
			}
		}

		part, e = parts.NextPart()
	}
	if e != nil && e != io.EOF && !strings.HasSuffix(e.Error(), " EOF") {
		return fmt.Errorf("error reading parts: %v", e)
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
			r = B64FilterReader(r, nil)
			if CheckEncoding {
				raw, e := ioutil.ReadAll(r)
				if e != nil {
					logger.Warn("msg", "cannot read data", "error", e)
					return bytes.NewReader(nil)
				}
				decoded := make([]byte, base64.StdEncoding.DecodedLen(len(raw)))
				n, e := base64.StdEncoding.Decode(decoded, raw)
				if e != nil {
					bad := string(raw[:min(20, len(raw))])
					txt := e.Error()
					q := strings.LastIndex(txt, " ")
					if q >= 0 {
						p, e2 := strconv.Atoi(txt[q+1:])
						if e2 == nil {
							p = max(0, p-4)
							q = min(len(raw), p+4)
							bad = string(raw[p:q])
						}
					}
					logger.Error("msg", "base64 decoding", "raw", bad, "error", e)
				}
				// K-MT9461
				return bytes.NewReader(decoded[:n])
			}
			return base64.NewDecoder(base64.StdEncoding, r)
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
		logger.Warn("msg", "unknown transfer-encoding", "transfer-encoding", te)
	}
	return
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

// strips directory information from the filename (both windows and unix)
func basename(fn string) string {
	if i := strings.LastIndexAny(fn, `/\`); i >= 0 {
		return fn[i+1:]
	}
	return fn
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
	m, e := mail.ReadMessage(io.TeeReader(r, h))
	if e != nil {
		return nil, "", e
	}
	return m, base64.URLEncoding.EncodeToString(h.Sum(nil)), nil
}

const b64chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// B64Filter is a decoding base64 filter
type B64Filter struct {
	n         int
	decodeMap [256]byte
	r         io.Reader
}

// B64FilterReader wraps the reader for decoding base64
func B64FilterReader(r io.Reader, decoder *base64.Encoding) io.Reader {
	f := B64Filter{r: r}
	for i := 0; i < len(f.decodeMap); i++ {
		f.decodeMap[i] = 0xFF
	}
	for i := 0; i < len(b64chars); i++ {
		f.decodeMap[b64chars[i]] = byte(i)
	}
	if decoder != nil {
		return base64.NewDecoder(decoder, &f)
	}
	return &f
}

// decodes Base64-encoded stream as reading
func (f *B64Filter) Read(b []byte) (int, error) {
	n, err := f.r.Read(b)
	if err != nil {
		if err == io.EOF && f.n%4 != 0 {
			miss := 4 - (f.n % 4)
			for i := 0; i < miss; i++ {
				b[n+i] = '='
			}
			f.n += miss
			return miss, nil
		}
		return n, err
	}
	for i := 0; i < n; i++ {
		if b[i] == '\r' || b[i] == '\n' || b[i] == '=' {
			continue
		}
		if c := f.decodeMap[b[i]]; c == 0xFF {
			logger.Warn("msg", "invalid char: "+fmt.Sprintf("%c(%d) @ %d", b[i], b[i], f.n+i))
			b[i] = '\n'
		}
	}
	f.n += n
	return n, err
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

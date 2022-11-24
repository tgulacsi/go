// Copyright 2013, 2022 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package i18nmail

import (
	"bytes"
	"compress/gzip"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-logr/logr/testr"
)

func TestMailAddress(t *testing.T) {
	for i, str := range [][3]string{
		[3]string{"=?iso-8859-2?Q?Bogl=E1rka_Tak=E1cs?= <tbogi77@gmail.com>",
			"Boglárka Takács", "<tbogi77@gmail.com>"},
		[3]string{"=?iso-8859-2?Q?K=E1r_Oszt=E1ly_=28kar=40kobe=2Ehu=29?= <kar@kobe.hu>",
			"Kár_Osztály kar@kobe.hu", "<kar@kobe.hu>"},
	} {
		mh := make(map[string][]string, 1)
		k := strconv.Itoa(i)
		mh[k] = []string{str[0]}
		mailHeader := mail.Header(mh)
		if addr, err := mailHeader.AddressList(k); err == nil {
			t.Errorf("address for %s: %q", k, addr)
		} else {
			t.Logf("error parsing address %s(%q): %s", k, mailHeader.Get(k), err)
		}
		if addr, err := ParseAddress(str[0]); err != nil {
			t.Errorf("error parsing address %q: %s", str[0], err)
		} else {
			t.Logf("address for %q: %q <%s>", str[0], addr.Name, addr.Address)
		}
	}
}
func TestWalk(t *testing.T) {
	logger = testr.New(t)
	b := make([]byte, 1024)
	dis, err := os.ReadDir("testdata")
	if len(dis) == 0 && err != nil {
		t.Fatal(err)
	}
	for _, di := range dis {
		fn := di.Name()
		tcName := strings.TrimSuffix(fn, ".gz")
		t.Run(tcName, func(t *testing.T) {
			var buf bytes.Buffer
			fh, err := os.Open(filepath.Join("testdata", fn))
			if err != nil {
				t.Fatal(err)
			}
			gr, err := gzip.NewReader(fh)
			if err != nil {
				fh.Close()
				t.Fatal(err)
			}
			_, err = io.Copy(&buf, gr)
			fh.Close()
			if err != nil {
				t.Fatal(err)
			}

			if true {
				msg, err := mail.ReadMessage(bytes.NewReader(buf.Bytes()))
				if err != nil {
					t.Fatal("read message:", err)
				}
				if ct := msg.Header.Get("Content-Type"); ct != "" {
					mediaType, params, err := mime.ParseMediaType(ct)
					if err != nil {
						t.Fatal("ct:", ct, ":", err)
					}
					t.Logf("ct=%q mt=%q params=%v", ct, mediaType, params)
					if strings.HasPrefix(mediaType, "multipart/") {
						mr := multipart.NewReader(msg.Body, params["boundary"])
						for {
							p, err := mr.NextRawPart()
							if err == io.EOF {
								break
							}
							if err != nil {
								t.Log("ERROR", err)
								break
							}
							slurp, err := io.ReadAll(p)
							p.Close()
							if err != nil {
								t.Log("slurp part:", err)
							}
							t.Logf("Part %q: %d\n", p.Header.Get("Content-Type"), len(slurp))
						}
					}
				}
			}

			mp := MailPart{Body: io.NewSectionReader(bytes.NewReader(buf.Bytes()), 0, int64(buf.Len()))}
			if err := Walk(mp,
				func(mp MailPart) error {
					body := mp.GetBody()
					n, err := body.Read(b[:cap(b)])
					if err != nil {
						panic(err)
					}
					if _, nextErr := io.Copy(io.Discard, body); nextErr != nil {
						logger.Error(nextErr, "next error")
						if err == nil {
							err = nextErr
						}
					}
					if err != nil || n == 0 {
						t.Errorf("%q %d/%d. read body of: %v", tcName, mp.Level, mp.Seq, err)
					}
					t.Logf("\n--- %q %d/%d. part ---\n%q %#v\n%s", tcName, mp.Level, mp.Seq, mp.ContentType, mp.MediaType, mp.Header)
					t.Log(strconv.Quote(string(b[:n])))
					return nil
				},
				false,
			); err != nil {
				fn := filepath.Join(os.TempDir(), tcName+".eml")
				os.WriteFile(fn, buf.Bytes(), 0400)
				t.Logf("Written %q", fn)
				t.Errorf("ERROR %q. walk: %v", tcName, err)
			}
		})
	}
}

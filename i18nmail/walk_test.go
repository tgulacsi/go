// Copyright 2013 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package i18nmail

import (
	"net/mail"
	"strconv"
	"testing"

	"github.com/tgulacsi/go/loghlp/tsthlp"
)

func TestMailAddress(t *testing.T) {
	Log.SetHandler(tsthlp.TestHandler(t))

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

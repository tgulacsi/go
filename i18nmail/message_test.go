// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package i18nmail

import (
	"strings"
	"testing"
)

var splitTest = [][2]string{
	[2]string{"=?utf-8?B?KE0yQikgUmU6IEvDoXJzesOhbTogMTQwNjk0LzEgVURXNDI5IFtbSzk5NjU3?= =?utf-8?Q?6-963815]]?=",
		"=?utf-8?B?KE0yQikgUmU6IEvDoXJzesOhbTogMTQwNjk0LzEgVURXNDI5IFtbSzk5NjU3?=|=?utf-8?Q?6-963815]]?="},
	[2]string{"=?iso-8859-2?Q?partner_lev._:_135944/1_, _RKO-870___=FAj_GPS_kooridn=E1ta_?= =?iso-8859-2?Q?=E9s_fuvar_lev=E9l_sz=FCks=E9ges?=",
		"=?iso-8859-2?Q?partner_lev._:_135944/1_, _RKO-870___=FAj_GPS_kooridn=E1ta_?=|=?iso-8859-2?Q?=E9s_fuvar_lev=E9l_sz=FCks=E9ges?="},
}

func TestSplit(t *testing.T) {
	Debugf = t.Logf
	defer func() {
		Debugf = nil
	}()
	for i, tup := range splitTest {
		result := strings.Join(
			strings.FieldsFunc(tup[0], new(splitter).fieldsFunc),
			"|")
		if result != tup[1] {
			t.Errorf("%d. data mismatch: awaited\n\t%s\n != got\n\t%s\n",
				i, tup[1], result)
		}
		t.Logf("%d. head: %s", i, result)
	}
}

var headDecodeTests = [][2]string{
	[2]string{"=?iso-8859-2?Q?partner_lev._:_135944/1_, _RKO-870___=FAj_GPS_kooridn=E1ta_?= =?iso-8859-2?Q?=E9s_fuvar_lev=E9l_sz=FCks=E9ges?=",
		"partner lev. : 135944/1 ,  RKO-870   új GPS kooridnáta és fuvar levél szükséges"},
	[2]string{"=?utf-8?B?KE0yQikgUmU6IEvDoXJzesOhbTogMTQwNjk0LzEgVURXNDI5IFtbSzk5NjU3?= =?utf-8?Q?6-963815]]?=",
		"(M2B) Re: Kárszám: 140694/1 UDW429 [[K996576-963815]]"},
	[2]string{"=?utf-8?b?RXNlZMOpa2Vzc8OpZ2kgw6lydGVzw610xZEgKExERzU4OSwgMTA5MjMxNTgp?=\n  =?utf-8?q?_=5B=5BS10923158-3772089=5D=5D?=",
		"Esedékességi értesítő (LDG589, 10923158) [[S10923158-3772089]]"},
}

func TestHeadDecode(t *testing.T) {
	for i, tup := range headDecodeTests {
		result := HeadDecode(tup[0])
		if result != tup[1] {
			t.Errorf("%d. data mismatch: awaited\n\t%s\n%v != got\n\t%s\n%v",
				i, tup[1], []byte(tup[1]), result, []byte(result))
		}
		t.Logf("%d. head: %s", i, result)
	}
}

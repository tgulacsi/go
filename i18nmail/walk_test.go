// Copyright 2013 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package i18nmail

import (
	"io/ioutil"
	"net/mail"
	"strconv"
	"strings"
	"testing"

	"github.com/tgulacsi/go/loghlp/kithlp"
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
	Logger.Swap(kithlp.TestLogger(t))
	if err := Walk(MailPart{Body: strings.NewReader(`Received: from mesmtp1.kobe.hu (192.168.1.55) by BUDSEXCH03.kobe.hu
 (192.168.1.38) with Microsoft SMTP Server id 14.3.123.3; Tue, 29 Sep 2015
 11:56:08 +0200
Received: from mail.neosoft.hu ([195.228.75.176])  by esmtp1.kobe.hu with
 ESMTP; 29 Sep 2015 13:34:45 +0200
Received: by mail.neosoft.hu (Postfix, from userid 1001)	id 77A81A5262; Tue,
 29 Sep 2015 11:56:07 +0200 (CEST)
From: Mail Delivery Subsystem <postmaster@mail.neosoft.hu>
To: =?utf-8?B?SGliYSBFbGxlbsWRcg==?= <hibae@kobe.hu>
Subject:
 =?utf-8?B?W1NVU1BFQ1RFRCBTUEFNXSBSZWplY3RlZDogRXNlZMOpa2Vzc8OpZ2kgw6ly?=
 =?utf-8?B?dGVzw610xZEgKExERzU4OSwgMTA5MjMxNTgpPyA9P3UuLi4=?=
Thread-Topic:
 =?utf-8?B?W1NVU1BFQ1RFRCBTUEFNXSBSZWplY3RlZDogRXNlZMOpa2Vzc8OpZ2kgw6ly?=
 =?utf-8?B?dGVzw610xZEgKExERzU4OSwgMTA5MjMxNTgpPyA9P3UuLi4=?=
Thread-Index: AQHQ+p0QOlqzX9MlVEyhhhIthgOmPQ==
Date: Tue, 29 Sep 2015 11:56:08 +0200
Message-ID: <dovecot-1443520567-445469-0@mail.neosoft.hu>
Content-Language: hu-HU
X-MS-Exchange-Organization-AuthAs: Anonymous
X-MS-Exchange-Organization-AuthSource: BUDSEXCH03.kobe.hu
X-MS-Has-Attach: yes
X-Auto-Response-Suppress: All
X-MS-TNEF-Correlator:
Content-Type: multipart/mixed;
	boundary="_003_dovecot14435205674454690mailneosofthu_"
MIME-Version: 1.0

--_003_dovecot14435205674454690mailneosofthu_
Content-Type: text/plain; charset="utf-8"
Content-ID: <BFB21492F4971A4C9D40A1A1A0242017@kobe.hu>
Content-Transfer-Encoding: base64

WW91ciBtZXNzYWdlIHRvIDxlcmlrYUBldXJvcGFja2luZ2NlbnRlci5odT4gd2FzIGF1dG9tYXRp
Y2FsbHkgcmVqZWN0ZWQ6DQpRdW90YSBleGNlZWRlZCAobWFpbGJveCBmb3IgdXNlciBpcyBmdWxs
KQ==

--_003_dovecot14435205674454690mailneosofthu_
Content-Type: message/delivery-status; name="ATT00001"
Content-Description: ATT00001
Content-Disposition: attachment; filename="ATT00001"; size=185;
	creation-date="Tue, 29 Sep 2015 09:56:08 GMT";
	modification-date="Tue, 29 Sep 2015 09:56:08 GMT"
Content-ID: <01D505059E45164C872358E1C14A0EF7@kobe.hu>
Content-Transfer-Encoding: base64

UmVwb3J0aW5nLU1UQTogZG5zOyBtYWlsLm5lb3NvZnQuaHUNCkZpbmFsLVJlY2lwaWVudDogcmZj
ODIyOyBlcmlrYUBldXJvcGFja2luZ2NlbnRlci5odQ0KQWN0aW9uOiBmYWlsZWQNClN0YXR1czog
NS4yLjINCg==

--_003_dovecot14435205674454690mailneosofthu_
Content-Type: message/rfc822
Content-Disposition: attachment;
	creation-date="Tue, 29 Sep 2015 09:56:08 GMT";
	modification-date="Tue, 29 Sep 2015 09:56:08 GMT"
Content-ID: <EF8E60DE1F7FA84F92EF00E393A7E8D9@kobe.hu>

Return-Path: <hibae@kobe.hu>
Delivered-To: erika@europackingcenter.hu
Received: from esmtp1.kobe.hu (esmtp1.kobe.hu [213.253.192.42])	by
 mail.neosoft.hu (Postfix) with ESMTP id 45DAEA5252	for
 <erika@europackingcenter.hu>; Tue, 29 Sep 2015 11:56:07 +0200 (CEST)
X-IPAS-Result: A2AQcwCXXwpW/wwCqMBIFhkBg15pgmSnN4JUCgGBIoUqhAGBBocFhiaBfQEBAQEBAYELhDcXbw8TCxcCBCQmERIBiDOZS50sj3aEWwwgiQKEK4Jlg1uBQwWBJJROgkuBYIFSiGOENoMXhTmMbIJ0HIFUcYkfAQEB
Received: from unknown (HELO budsexch01.kobe.hu) ([192.168.2.12])  by
 esmtp1.kobe.hu with ESMTP; 29 Sep 2015 13:34:44 +0200
Received: from asprodp6.kobe.hu (192.168.1.66) by BUDSEXCH01.kobe.hu
 (192.168.1.30) with Microsoft SMTP Server id 14.3.195.1; Tue, 29 Sep 2015
 11:56:05 +0200
Date: Tue, 29 Sep 2015 09:54:13 +0000
To: <erika@europackingcenter.hu>
From: <admin@kobe.hu>
Subject: =?utf-8?b?RXNlZMOpa2Vzc8OpZ2kgw6lydGVzw610xZEgKExERzU4OSwgMTA5MjMxNTgp?=
 =?utf-8?q?_=5B=5BS10923158-3772089=5D=5D?=
Message-ID: <6b1c59ac-3478-4eec-b922-34ea6b39c6b6@BUDSEXCH01.kobe.hu>
Content-Type: text/plain
MIME-Version: 1.0



--_003_dovecot14435205674454690mailneosofthu_--
`)},
		func(mp MailPart) error {
			b, err := ioutil.ReadAll(mp.Body)
			if err != nil {
				t.Errorf("read body of %d/%d: %v", mp.Level, mp.Seq, err)
			}
			t.Logf("part %d/%d %q %#v\n%s\n%s", mp.Level, mp.Seq, mp.ContentType, mp.MediaType, mp.Header, b)
			return nil
		},
		false,
	); err != nil {
		t.Errorf("walk: %v", err)
	}
}

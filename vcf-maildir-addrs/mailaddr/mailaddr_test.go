package mailaddr

import (
	"fmt"
	"net/mail"
	"strings"
	"testing"
)

func TestParseAddressList(t *testing.T) {
	for k, v := range map[string]struct {
		In   string
		Want []mail.Address
	}{
		"naked":       {In: "tamas@gulacsi.eu", Want: []mail.Address{{Address: "tamas@gulacsi.eu"}}},
		"angled":      {In: "<tamas@gulacsi.eu>", Want: []mail.Address{{Address: "tamas@gulacsi.eu"}}},
		"named":       {In: "\"Tamás Gulácsi\" <tamas@gulacsi.eu>", Want: []mail.Address{{Name: "Tamás Gulácsi", Address: "tamas@gulacsi.eu"}}},
		"nakeNamed":   {In: "Tamás Gonosz Gulácsi tamas@gulacsi.eu <tamas@gulacsi.eu>", Want: []mail.Address{{Name: "Tamás Gonosz Gulácsi tamas_at_gulacsi.eu", Address: "tamas@gulacsi.eu"}}},
		"singleQuote": {In: "'Tamás Gulácsi' <tamas@gulacsi.eu>", Want: []mail.Address{{Name: "'Tamás Gulácsi'", Address: "tamas@gulacsi.eu"}}},

		"long": {In: "Adler György <gyorgyadler@t-online.hu>, Adlerné S. Anikó <adlerne@freemail.hu>, \"Anna Marietta\" <marietta.anna62@gmail.com>, \"Attila Toldi\" <cafelutyo@gmail.com>, \"Balatoni Henrik\" <balatonihenrik@gmail.com>, Balázsi Győző <GYOZO.BALAZSI@hotmail.com>, Bedros Róbert <drbedros@drbedros.hu>, Blazsovits Éva <info@sasadiovi.hu>, Bóna Eszter <mihuxsi@gmail.com>, Bóna Eszter párja <benebator@gmail.com>, Bőr Tamás Szécsi Julianna <dermavet@t-online.hu>, \"Czibula Katalin\" <czibula.katalin@gmail.com>, Csapó Zita <csapo@daruline.hu>, Cseh Tiborné <tiborne.cseh@groupama.hu>, Csilling László <laszlo@csilling.com>, Eszter Molnár <eszter0316@gmail.com>, Gábor Szomolányi <szomolg@gmail.com>, Gack Józsefné <gack.jozsefne@datatrain.hu>, Gulácsi Tamás <gt-siofok@gthomas.eu>, \"Hermann Tibor\" <hermann@stanctechnik.hu>, \"Hermann Tibor\" <info@stanctechnik.hu>, <ikalman@msn.com>, Ildikó Kószó <koszoildi10@gmail.com>, Jex Gábor <JexG@modusz.hu>, Kálmán Mónika <kalman.monika@otpeletjaradek.hu>, Kaló Orsolya <kaloors@gmail.com>, \"kaman karoly\" <karoly.kaman@gmail.com>, \"Kis Alice\" <alice@oraesekszerhaz.t-online.hu>, \"Klara Schneider\" <schneider.klarika@gmail.com>, Kollárik Éva <enagyova@yahoo.com>, \"Koszta Veronika\" <verula21@gmail.com>, Kovács István <istkov@freemail.hu>, Kováts András <andras.kovats@keletterv.hu>, Kuglics József <nbd6970@gmail.com>, Lassan György <lassan.gyorgy20@gmail.com>, Leitner György <gyorgy.l.leitner@gmail.com>, Magyari László <magyari@daruline.hu>, Márta T.Vajda <tothvajda.marta@gmail.com>, Melinda Tóth <t.rezmelinda@gmail.com>, Mórocz-Schmitz Irén <irmoschmitz@gmail.com>, Nagy Gáborné Nagy Tamás <ng0130@gmail.com>, Nagy Tamás <nt71@citromail.hu>, Nagy Zoltán <magdolna.nagyne@gmail.com>, Nyikus Éva <evi@csilling.com>, \"ordog.istvan\" <ordog.istvan@upcmail.hu>, OTP Életjáradék <info@otpeletjaradek.hu>, Perneczky Tamás t. perneczky@gmail. com <t.perneczky@gmail.com>, Pethes Nóra <tariek@t-online.hu>, \"Pethes Olga\" <pethes.olga@chello.hu>, Rácz Zsuzsanna <razsuzsanna@gmail.com>, Ruff János <Ruff@iveco-levantex.hu>, <seri@seri.hu>, Skivraha Éva <skivrahae@t-online.hu>, Stefán György <stgeorge108@gmail.com>, Szabó Ervin <szaboervin@gmail.com>, Szabó Tímea <timi2141@freemail.hu>, Szabó Zoltán <szabo@sysinforg.hu>, \"Szalay Judit\" <judit@szalay-family.hu>, Szécsi Julianna <szjulcsi123@gmail.com>, Szente János <52jszente@gmail.com>, Tamás Miklós <elnok@citytaxi.hu>, Toldi Imréné <toldiimre@t-online.hu>, Toth Tamás <totht.tamas66@gmail.com>, Urbancsok Tamás <t.urbancsok@gmail.com>, \"Urbancsok Zsuzsanna\" <urbancsokzsuzsi@gmail.com>, Varga József B67 <vargajp63@gmail.com>, Vida László <vida.bt@citromail.hu>, Vödrös Anikó <vodros.aniko@gmail.com>, 'Lajos Gyócsi' <lui6567@gmail.com>",
			Want: []mail.Address{{Name: "Adler György", Address: "gyorgyadler@t-online.hu"}, {Name: "Adlerné S. Anikó", Address: "adlerne@freemail.hu"}, {Name: "Anna Marietta", Address: "marietta.anna62@gmail.com"}, {Name: "Attila Toldi", Address: "cafelutyo@gmail.com"}, {Name: "Balatoni Henrik", Address: "balatonihenrik@gmail.com"}, {Name: "Balázsi Győző", Address: "GYOZO.BALAZSI@hotmail.com"}, {Name: "Bedros Róbert", Address: "drbedros@drbedros.hu"}, {Name: "Blazsovits Éva", Address: "info@sasadiovi.hu"}, {Name: "Bóna Eszter", Address: "mihuxsi@gmail.com"}, {Name: "Bóna Eszter párja", Address: "benebator@gmail.com"}, {Name: "Bőr Tamás Szécsi Julianna", Address: "dermavet@t-online.hu"}, {Name: "Czibula Katalin", Address: "czibula.katalin@gmail.com"}, {Name: "Csapó Zita", Address: "csapo@daruline.hu"}, {Name: "Cseh Tiborné", Address: "tiborne.cseh@groupama.hu"}, {Name: "Csilling László", Address: "laszlo@csilling.com"}, {Name: "Eszter Molnár", Address: "eszter0316@gmail.com"}, {Name: "Gábor Szomolányi", Address: "szomolg@gmail.com"}, {Name: "Gack Józsefné", Address: "gack.jozsefne@datatrain.hu"}, {Name: "Gulácsi Tamás", Address: "gt-siofok@gthomas.eu"}, {Name: "Hermann Tibor", Address: "hermann@stanctechnik.hu"}, {Name: "Hermann Tibor", Address: "info@stanctechnik.hu"}, {Name: "", Address: "ikalman@msn.com"}, {Name: "Ildikó Kószó", Address: "koszoildi10@gmail.com"}, {Name: "Jex Gábor", Address: "JexG@modusz.hu"}, {Name: "Kálmán Mónika", Address: "kalman.monika@otpeletjaradek.hu"}, {Name: "Kaló Orsolya", Address: "kaloors@gmail.com"}, {Name: "kaman karoly", Address: "karoly.kaman@gmail.com"}, {Name: "Kis Alice", Address: "alice@oraesekszerhaz.t-online.hu"}, {Name: "Klara Schneider", Address: "schneider.klarika@gmail.com"}, {Name: "Kollárik Éva", Address: "enagyova@yahoo.com"}, {Name: "Koszta Veronika", Address: "verula21@gmail.com"}, {Name: "Kovács István", Address: "istkov@freemail.hu"}, {Name: "Kováts András", Address: "andras.kovats@keletterv.hu"}, {Name: "Kuglics József", Address: "nbd6970@gmail.com"}, {Name: "Lassan György", Address: "lassan.gyorgy20@gmail.com"}, {Name: "Leitner György", Address: "gyorgy.l.leitner@gmail.com"}, {Name: "Magyari László", Address: "magyari@daruline.hu"}, {Name: "Márta T.Vajda", Address: "tothvajda.marta@gmail.com"}, {Name: "Melinda Tóth", Address: "t.rezmelinda@gmail.com"}, {Name: "Mórocz-Schmitz Irén", Address: "irmoschmitz@gmail.com"}, {Name: "Nagy Gáborné Nagy Tamás", Address: "ng0130@gmail.com"}, {Name: "Nagy Tamás", Address: "nt71@citromail.hu"}, {Name: "Nagy Zoltán", Address: "magdolna.nagyne@gmail.com"}, {Name: "Nyikus Éva", Address: "evi@csilling.com"}, {Name: "ordog.istvan", Address: "ordog.istvan@upcmail.hu"}, {Name: "OTP Életjáradék", Address: "info@otpeletjaradek.hu"}, {Name: "Perneczky Tamás t. perneczky_at_gmail. com", Address: "t.perneczky@gmail.com"}, {Name: "Pethes Nóra", Address: "tariek@t-online.hu"}, {Name: "Pethes Olga", Address: "pethes.olga@chello.hu"}, {Name: "Rácz Zsuzsanna", Address: "razsuzsanna@gmail.com"}, {Name: "Ruff János", Address: "Ruff@iveco-levantex.hu"}, {Name: "", Address: "seri@seri.hu"}, {Name: "Skivraha Éva", Address: "skivrahae@t-online.hu"}, {Name: "Stefán György", Address: "stgeorge108@gmail.com"}, {Name: "Szabó Ervin", Address: "szaboervin@gmail.com"}, {Name: "Szabó Tímea", Address: "timi2141@freemail.hu"}, {Name: "Szabó Zoltán", Address: "szabo@sysinforg.hu"}, {Name: "Szalay Judit", Address: "judit@szalay-family.hu"}, {Name: "Szécsi Julianna", Address: "szjulcsi123@gmail.com"}, {Name: "Szente János", Address: "52jszente@gmail.com"}, {Name: "Tamás Miklós", Address: "elnok@citytaxi.hu"}, {Name: "Toldi Imréné", Address: "toldiimre@t-online.hu"}, {Name: "Toth Tamás", Address: "totht.tamas66@gmail.com"}, {Name: "Urbancsok Tamás", Address: "t.urbancsok@gmail.com"}, {Name: "Urbancsok Zsuzsanna", Address: "urbancsokzsuzsi@gmail.com"}, {Name: "Varga József B67", Address: "vargajp63@gmail.com"}, {Name: "Vida László", Address: "vida.bt@citromail.hu"}, {Name: "Vödrös Anikó", Address: "vodros.aniko@gmail.com"}, {Name: "'Lajos Gyócsi'", Address: "lui6567@gmail.com"}},
		},
	} {
		k, v := k, v
		t.Run(k, func(t *testing.T) {
			got, err := ParseAddressList(v.In)
			if err != nil {
				t.Error(err)
			}
			if got, want := len(got), len(v.Want); got != want {
				t.Errorf("got %d, wanted %d addresses", got, want)
			}
			var buf strings.Builder
			buf.WriteString("[]mail.Address{")
			for i, g := range got {
				var w mail.Address
				if len(v.Want) > i {
					w = v.Want[i]
				}
				if g.Address != w.Address {
					t.Errorf("%d.Address got %q, wanted %q", i, g.Address, w.Address)
				}
				if g.Name != w.Name {
					t.Errorf("%d.Name got %q, wanted %q", i, g.Name, w.Name)
				}

				fmt.Fprintf(&buf, "{Address:%q,Name:%q}, ", g.Address, g.Name)
			}
			buf.WriteString("}")
			t.Log(buf.String())
		})
	}
}

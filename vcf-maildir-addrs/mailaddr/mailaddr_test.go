package mailaddr

import (
	"net/mail"
	"testing"
)

func TestParseAddressList(t *testing.T) {
	const data = "Adler György <gyorgyadler@t-online.hu>, Adlerné S. Anikó <adlerne@freemail.hu>, \"Anna Marietta\" <marietta.anna62@gmail.com>, \"Attila Toldi\" <cafelutyo@gmail.com>, \"Balatoni Henrik\" <balatonihenrik@gmail.com>, Balázsi Győző <GYOZO.BALAZSI@hotmail.com>, Bedros Róbert <drbedros@drbedros.hu>, Blazsovits Éva <info@sasadiovi.hu>, Bóna Eszter <mihuxsi@gmail.com>, Bóna Eszter párja <benebator@gmail.com>, Bőr Tamás Szécsi Julianna <dermavet@t-online.hu>, \"Czibula Katalin\" <czibula.katalin@gmail.com>, Csapó Zita <csapo@daruline.hu>, Cseh Tiborné <tiborne.cseh@groupama.hu>, Csilling László <laszlo@csilling.com>, Eszter Molnár <eszter0316@gmail.com>, Gábor Szomolányi <szomolg@gmail.com>, Gack Józsefné <gack.jozsefne@datatrain.hu>, Gulácsi Tamás <gt-siofok@gthomas.eu>, \"Hermann Tibor\" <hermann@stanctechnik.hu>, \"Hermann Tibor\" <info@stanctechnik.hu>, <ikalman@msn.com>, Ildikó Kószó <koszoildi10@gmail.com>, Jex Gábor <JexG@modusz.hu>, Kálmán Mónika <kalman.monika@otpeletjaradek.hu>, Kaló Orsolya <kaloors@gmail.com>, \"kaman karoly\" <karoly.kaman@gmail.com>, \"Kis Alice\" <alice@oraesekszerhaz.t-online.hu>, \"Klara Schneider\" <schneider.klarika@gmail.com>, Kollárik Éva <enagyova@yahoo.com>, \"Koszta Veronika\" <verula21@gmail.com>, Kovács István <istkov@freemail.hu>, Kováts András <andras.kovats@keletterv.hu>, Kuglics József <nbd6970@gmail.com>, Lassan György <lassan.gyorgy20@gmail.com>, Leitner György <gyorgy.l.leitner@gmail.com>, Magyari László <magyari@daruline.hu>, Márta T.Vajda <tothvajda.marta@gmail.com>, Melinda Tóth <t.rezmelinda@gmail.com>, Mórocz-Schmitz Irén <irmoschmitz@gmail.com>, Nagy Gáborné Nagy Tamás <ng0130@gmail.com>, Nagy Tamás <nt71@citromail.hu>, Nagy Zoltán <magdolna.nagyne@gmail.com>, Nyikus Éva <evi@csilling.com>, \"ordog.istvan\" <ordog.istvan@upcmail.hu>, OTP Életjáradék <info@otpeletjaradek.hu>, Perneczky Tamás t. perneczky@gmail. com <t.perneczky@gmail.com>, Pethes Nóra <tariek@t-online.hu>, \"Pethes Olga\" <pethes.olga@chello.hu>, Rácz Zsuzsanna <razsuzsanna@gmail.com>, Ruff János <Ruff@iveco-levantex.hu>, <seri@seri.hu>, Skivraha Éva <skivrahae@t-online.hu>, Stefán György <stgeorge108@gmail.com>, Szabó Ervin <szaboervin@gmail.com>, Szabó Tímea <timi2141@freemail.hu>, Szabó Zoltán <szabo@sysinforg.hu>, \"Szalay Judit\" <judit@szalay-family.hu>, Szécsi Julianna <szjulcsi123@gmail.com>, Szente János <52jszente@gmail.com>, Tamás Miklós <elnok@citytaxi.hu>, Toldi Imréné <toldiimre@t-online.hu>, Toth Tamás <totht.tamas66@gmail.com>, Urbancsok Tamás <t.urbancsok@gmail.com>, \"Urbancsok Zsuzsanna\" <urbancsokzsuzsi@gmail.com>, Varga József B67 <vargajp63@gmail.com>, Vida László <vida.bt@citromail.hu>, Vödrös Anikó <vodros.aniko@gmail.com>, 'Lajos Gyócsi' <lui6567@gmail.com>"
	aa, err := mail.ParseAddressList(data)
	if err != nil {
		t.Error(err)
	}
	t.Log(aa)
}

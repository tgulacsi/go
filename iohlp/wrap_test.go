/*
Copyright 2016 Tamás Gulácsi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package iohlp

import (
	"io/ioutil"
	"strings"
	"testing"
)

func TestWrappingReader(t *testing.T) {
	for i, tc := range []struct {
		in    string
		width int
		await string
	}{
		{"a b c", 1, "a\nb\nc\n"},
		{
			"Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.",
			80,
			`Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod tempor
incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis
nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu
fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in
culpa qui officia deserunt mollit anim id est laborum.
`},
		{
			"Tisztelt Kobe! Mai nap folyamán kaptam önöktől egy levelet 2015.12.03.-an történt egy baleset és a levélbe csatoltak még egy résztvevői nyiltakozatott és kérik hogy 5 napon belül jutasam vissza önökhöz de én már másnap voltam a tatabányai kirendeltségbe és egy ugyan ilyet már kitöltöttek velünk kérdésem az hogy mi lett azzal a papíral vagy ez a levél érkezett meg késve ? Válaszát előre is köszönöm! TISZTELETTEL: Tromposch Tamás ",
			40,
			"Tisztelt Kobe! Mai nap folyamán kaptam önöktől egy levelet 2015.12.03.-an történt egy baleset és a levélbe csatoltak még egy résztvevői nyiltakozatott és kérik hogy 5 napon belül jutasam vissza önökhöz de én már másnap voltam a tatabányai kirendeltségbe és egy ugyan ilyet már kitöltöttek velünk kérdésem az hogy mi lett azzal a papíral vagy ez a levél érkezett meg késve ? Válaszát előre is köszönöm! TISZTELETTEL: Tromposch Tamás ",
		},
	} {
		b, err := ioutil.ReadAll(WrappingReader(strings.NewReader(tc.in), uint(tc.width)))
		if err != nil {
			t.Errorf("%d: %v", i, err)
			continue
		}
		got := string(b)
		if got != tc.await {
			t.Errorf("%d. got %q, awaited %q.", i, got, tc.await)
		}
	}
}

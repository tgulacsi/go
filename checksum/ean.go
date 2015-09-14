// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package checksum

type Calculator interface {
	Calculate(string) uint8
}

type Checksum interface {
	Calculator
	Decorate(string) string
	Undecorate(string) string
	IsValid(string) bool
}

var (
	CDV     = checksum{Calculator: csCDV{}}
	EAN8    = checksum{Calculator: csEAN8{}}
	CvtEAN8 = checksum{Calculator: csCvtEAN8{}, Direction: dirPrepend}
)

type direction uint8

const (
	dirAppend = direction(iota)
	dirPrepend
)

type csCDV struct{}

var cdvMul = [...]uint8{19, 17, 13, 11, 7, 5, 3, 1}

func (cs csCDV) Calculate(text string) uint8 {
	if len(text) > 8 {
		text = text[:8]
	}
	var sum uint8
	for i, r := range text {
		sum = (sum + (cdvMul[i] * uint8(r-'0'))) % 97
	}
	return sum % 10
}

type csCvtEAN8 struct{}

func (cs csCvtEAN8) Calculate(text string) uint8 {
	var sum uint8
	mul := uint8(3)
	for _, r := range text[:len(text)-1] {
		sum = (sum + uint8(r-'0')*mul) % 10
		mul = 4 - mul
	}
	return sum
}

type csEAN8 struct{}

func (cs csEAN8) Calculate(text string) uint8 {
	var sum uint8
	mul := uint8(3)
	for _, r := range text {
		sum = (sum + uint8(r-'0')*mul) % 10
		mul = 4 - mul
	}
	return (10 - sum) % 10
}

type checksum struct {
	Calculator
	Direction direction
}

func (c checksum) IsValid(text string) bool {
	return text == c.Decorate(c.Undecorate(text))
}

func (c checksum) Decorate(text string) string {
	if c.Direction == dirPrepend {
		return string([]byte{c.Calculate(text) + '0'}) + text
	}
	return text + string([]byte{c.Calculate(text) + '0'})
}
func (c checksum) Undecorate(text string) string {
	if c.Direction == dirPrepend {
		return text[1:]
	}
	return text[:len(text)-1]
}

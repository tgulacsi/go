// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package checksum

type Checksum interface {
	Calculate(string) uint8
}

type Decorator interface {
	Decorate(string) string
	Undecorate(string) string
}

type Checker interface {
	IsValid(string) bool
}

var (
	CDV     = check{csAppend{csCDV{}}}
	EAN8    = check{csAppend{csEAN8{}}}
	CvtEAN8 = check{csPrepend{csCvtEAN8{}}}
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
func (cs csCDV) Decorate(text string) string {
	return text + string([]byte{'0' + cs.Calculate(text)})
}
func (cs csCDV) Undecorate(text string) string {
	return text[:len(text)-1]
}
func (cs csCDV) IsValid(text string) bool { return text == cs.Decorate(cs.Undecorate(text)) }

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
func (cs csCvtEAN8) Decorate(text string) string {
	return string([]byte{cs.Calculate(text) + '0'}) + text
}
func (cs csCvtEAN8) Undecorate(text string) string {
	return text[1:]
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

type ChecksumDecorator interface {
	Checksum
	Decorator
}
type check struct {
	ChecksumDecorator
}

func (c check) IsValid(text string) bool {
	return text == c.Decorate(c.Undecorate(text))
}

type csAppend struct {
	Checksum
}

func (cs csAppend) Decorate(text string) string {
	return text + string([]byte{cs.Calculate(text) + '0'})
}
func (cs csAppend) Undecorate(text string) string {
	return text[:len(text)-1]
}

type csPrepend struct {
	Checksum
}

func (cs csPrepend) Decorate(text string) string {
	return string([]byte{cs.Calculate(text) + '0'}) + text
}
func (cs csPrepend) Undecorate(text string) string {
	return text[1:]
}

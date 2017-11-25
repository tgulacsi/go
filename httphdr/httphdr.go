/*
copyright 2017 tamás gulácsi

licensed under the apache license, version 2.0 (the "license");
you may not use this file except in compliance with the license.
you may obtain a copy of the license at

     http://www.apache.org/licenses/license-2.0

unless required by applicable law or agreed to in writing, software
distributed under the license is distributed on an "as is" basis,
without warranties or conditions of any kind, either express or implied.
see the license for the specific language governing permissions and
limitations under the license.
*/

// Package httphdr provides some support for HTTP headers.
package httphdr

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

var bPool = sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 0, 128)) }}

// ContentDisposition returns a formatted http://tools.ietf.org/html/rfc6266 header.
func ContentDisposition(dispType string, filename string) string {
	b := bPool.Get().(*bytes.Buffer)
	defer func() {
		b.Reset()
		bPool.Put(b)
	}()
	b.WriteString(dispType)
	b.WriteString(`; filename="`)
	justLatin := true
	start := b.Len()
	for _, r := range filename {
		if r < unicode.MaxLatin1 && r != '"' && !unicode.IsControl(r) && !unicode.IsSpace(r) {
			b.WriteByte(byte(r))
		} else {
			fmt.Fprintf(b, "%%%x", r)
			justLatin = false
		}
	}
	if justLatin {
		b.WriteByte('"')
		return b.String()
	}
	encoded := b.Bytes()[start:]
	b.WriteString(`" filename*=utf-8''`)
	b.Write(encoded)
	return b.String()
}

type Accept []KeyVal
type KeyVal struct {
	Key string
	Val interface{}
}

// Match the given Accept header.
//
// Parts of the code is copied from github.com/jchannon/negotiator.
func (A Accept) Match(accept string) interface{} {
	if len(A) == 0 {
		return nil
	}
	if accept == "" {
		return A[0].Val
	}

	for _, mr := range parseMediaRanges(accept) {
		if len(mr.Value) == 0 {
			continue
		}

		if strings.EqualFold(mr.Value, "*/*") {
			return A[0].Val
		}

		for _, pair := range A {
			if doesMatch(mr.Value, pair.Key) {
				return pair.Val
			}
		}
	}
	return nil
}
func split(x string) (a, b string) {
	a, b = x, "*"
	if i := strings.IndexByte(x, '/'); i >= 0 {
		a, b = x[:i], x[i+1:]
	}
	if a == "" {
		a = "*"
	}
	if b == "" {
		b = "*"
	}
	return a, b
}

func doesMatch(needle, candidate string) bool {
	nA, nB := split(needle)
	cA, cB := split(candidate)
	return (strings.EqualFold(cA, "*") || strings.EqualFold(nA, "*") || strings.EqualFold(cA, nA)) &&
		(strings.EqualFold(cB, "*") || strings.EqualFold(nB, "*") || strings.EqualFold(cB, nB))
}

const (
	// parameteredMediaRangeWeight is the default weight of a media range with an
	// accept-param
	parameteredMediaRangeWeight float64 = 1.0 //e.g text/html;level=1
	// typeSubtypeMediaRangeWeight is the default weight of a media range with
	// type and subtype defined
	typeSubtypeMediaRangeWeight float64 = 0.9 //e.g text/html
	// typeStarMediaRangeWeight is the default weight of a media range with a type
	// defined but * for subtype
	typeStarMediaRangeWeight float64 = 0.8 //e.g text/*
	// starStarMediaRangeWeight is the default weight of a media range with any
	// type or any subtype defined
	starStarMediaRangeWeight float64 = 0.7 //e.g */*
)

// MediaRanges returns prioritized media ranges
func parseMediaRanges(accept string) []weightedValue {
	mrs := strings.Split(accept, ",")
	retVals := make([]weightedValue, 0, len(mrs))

	for _, mr := range mrs {
		mrAndAcceptParam := strings.Split(mr, ";")
		//if no accept-param
		if len(mrAndAcceptParam) == 1 {
			retVals = append(retVals, handleMediaRangeNoAcceptParams(mrAndAcceptParam[0]))
			continue
		}

		retVals = append(retVals, handleMediaRangeWithAcceptParams(mrAndAcceptParam[0], mrAndAcceptParam[1:]))
	}

	//If no Accept header field is present, then it is assumed that the client
	//accepts all media types. If an Accept header field is present, and if the
	//server cannot send a response which is acceptable according to the combined
	//Accept field value, then the server SHOULD send a 406 (not acceptable)
	//response.
	sort.Sort(byWeight(retVals))

	return retVals
}

func handleMediaRangeWithAcceptParams(mediaRange string, acceptParams []string) weightedValue {
	wv := new(weightedValue)
	wv.Value = strings.TrimSpace(mediaRange)
	wv.Weight = parameteredMediaRangeWeight

	for index := 0; index < len(acceptParams); index++ {
		ap := strings.ToLower(acceptParams[index])
		if isQualityAcceptParam(ap) {
			wv.Weight = parseQuality(ap)
		} else {
			wv.Value = strings.Join([]string{wv.Value, acceptParams[index]}, ";")
		}
	}
	return *wv
}

func isQualityAcceptParam(acceptParam string) bool {
	return strings.Contains(acceptParam, "q=")
}

func parseQuality(acceptParam string) float64 {
	weight, err := strconv.ParseFloat(strings.SplitAfter(acceptParam, "q=")[1], 64)
	if err != nil {
		weight = 1.0
	}
	return weight
}

func handleMediaRangeNoAcceptParams(mediaRange string) weightedValue {
	wv := new(weightedValue)
	wv.Value = strings.TrimSpace(mediaRange)
	wv.Weight = 0.0

	typeSubtype := strings.Split(wv.Value, "/")
	if len(typeSubtype) == 2 {
		switch {
		//a type of * with a non-star subtype is invalid, so if the type is
		//star the assume that the subtype is too
		case typeSubtype[0] == "*": //&& typeSubtype[1] == "*":
			wv.Weight = starStarMediaRangeWeight
		case typeSubtype[1] == "*":
			wv.Weight = typeStarMediaRangeWeight
		case typeSubtype[1] != "*":
			wv.Weight = typeSubtypeMediaRangeWeight
		}
	} //else invalid media range the weight remains 0.0

	return *wv
}

// weightedValue is a value and associate weight between 0.0 and 1.0
type weightedValue struct {
	Value  string
	Weight float64
}

// ByWeight implements sort.Interface for []WeightedValue based
//on the Weight field. The data will be returned sorted decending
type byWeight []weightedValue

func (a byWeight) Len() int           { return len(a) }
func (a byWeight) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byWeight) Less(i, j int) bool { return a[i].Weight > a[j].Weight }

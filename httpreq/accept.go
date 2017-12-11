/*
Copyright 2017 Tamás Gulácsi

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

package httpreq

import (
	"mime"
	"strconv"
	"strings"
)

// ParseAccept parses the HTTP requests' "Accept" header - see RFC2616
/*
Content-Type   = "Content-Type" ":" media-type
media-type     = type "/" subtype *( ";" parameter )
parameter      = attribute "=" value
attribute      = token
value          = token | quoted-string
quoted-string  = ( <"> *(qdtext | quoted-pair ) <"> )
qdtext         = <any TEXT except <">>
quoted-pair    = "\" CHAR
type           = token
subtype        = token
token          = 1*<any CHAR except CTLs or separators>
separators     = "(" | ")" | "<" | ">" | "@"
               | "," | ";" | ":" | "\" | <">
               | "/" | "[" | "]" | "?" | "="
               | "{" | "}" | SP | HT
CTL            = <any US-ASCII ctl chr (0-31) and DEL (127)>
*/
func ParseAccept(accept string) (mediaRanges []mediaRange, err error) {
	var mr mediaRange
	for _, mRange := range strings.Split(accept, ",") {
		if mr, err = parseMediaRange(mRange); err != nil {
			return
		}
		mediaRanges = append(mediaRanges, mr)
	}
	return
}

// BestAcceptMatch returns the best match between supported media types and
// accepted media ranges
func BestAcceptMatch(supported, accept string) (string, error) {
	mediaRanges, err := ParseAccept(accept)
	if err != nil {
		return "", err
	}
	var (
		target, match mediaRange
		bestMedia     string
		bestQ         = float32(-1)
	)
	for _, mType := range strings.Split(supported, ",") {
		mType = strings.TrimSpace(mType)
		if target, err = parseMediaRange(mType); err != nil {
			return "", err
		}
		match = fit(target, mediaRanges)
		if match.typ == "" { // not found
			continue
		}
		if match.q > bestQ {
			bestQ, bestMedia = match.q, mType
		}
	}
	return bestMedia, nil
}

func parseMediaRange(mRange string) (mr mediaRange, err error) {
	if mr.typ, mr.params, err = mime.ParseMediaType(strings.TrimSpace(mRange)); err != nil {
		return
	}
	if i := strings.Index(mr.typ, "/"); i >= 0 {
		mr.typ, mr.subtyp = mr.typ[:i], mr.typ[i+1:]
	}
	mr.q = 1
	q := mr.params["q"]
	if q != "" {
		f, parseErr := strconv.ParseFloat(q, 32)
		if parseErr == nil && f >= 0.0 && f <= 1.0 {
			mr.q = float32(f)
		}
	}
	return
}

type mediaRange struct {
	typ, subtyp string
	q           float32
	params      map[string]string
}

// fit returns the best fitting mediaRange from to the parsed mediaRanges
func fit(target mediaRange, mediaRanges []mediaRange) (bestFit mediaRange) {
	bestFitness := -1
	var fitness int
	for _, mr := range mediaRanges {
		fitness = 0
		if (mr.typ == target.typ || mr.typ == "*") &&
			(mr.subtyp == target.subtyp || mr.subtyp == "*") {
			if mr.typ == target.typ {
				fitness = 10000
			}
			if mr.subtyp == target.subtyp {
				fitness += 100
			}
			for k, v := range target.params {
				if k != "q" && mr.params["k"] == v {
					fitness++
				}
			}
			if fitness > bestFitness {
				bestFitness = fitness
				bestFit = mr
			}
		}
	}
	return bestFit
}

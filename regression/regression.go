// Copyright 2014 Tamás Gulácsi
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

// Package regression contains a simple Thiel-Sen estimator for linear regression.
package regression

import (
	"sort"
)

// LinearRegression returns the slope and intercept, using Thiel-Sen estimator.
// This is the median of the slopes defined only from pairs of points having distinct x-coordinates.
func LinearRegression(xData, yData []float64) (float64, float64) {
	if len(xData) != len(yData) {
		panic("x and y must have the same length!")
	}
	if len(xData) == 0 {
		return 0, 0
	}
	lns := lines(make([]line, 0, len(xData)*len(xData)))
	for i, x1 := range xData {
		for j, x2 := range xData {
			if i == j || x1 == x2 {
				continue
			}
			a := (yData[j] - yData[i]) / (x2 - x1)
			// y = a*x + b  ==>  b = y - a*x
			lns = append(lns, line{a: a, b: yData[j] - a*x2})
		}
	}
	m := lns.Median()
	return m.a, m.b
}

type line struct {
	a, b float64
}

type lines []line

func (ln lines) Len() int           { return len(ln) }
func (ln lines) Less(i, j int) bool { return ln[i].a < ln[j].a }
func (ln lines) Swap(i, j int)      { ln[i], ln[j] = ln[j], ln[i] }

// Median returns the median from all the lines - based on the slope.
// This sorts the underlying slice.
func (ln lines) Median() line {
	if len(ln) == 0 {
		return line{}
	}
	sort.Sort(ln)
	return ln[len(ln)/2]
}

// Median returns the median value of the data.
// If it is already sorted, then it will not sort again.
// If it is not sorted, then the slice is copied first, to not influence the original.
func Median(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	if sort.Float64sAreSorted(x) {
		return x[len(x)/2]
	}
	x2 := make([]float64, len(x))
	copy(x2, x)
	sort.Float64s(x2)
	return x2[len(x2)/2]
}

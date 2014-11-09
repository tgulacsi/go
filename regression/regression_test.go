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

package regression

import (
	"testing"
)

func TestMedian(t *testing.T) {
	for i, st := range []struct {
		arr []float64
		res float64
	}{
		{arr: []float64{9, 8, 7, 6, 5, 4, 3, 2, 1}, res: 5},
	} {
		m := Median(st.arr)
		if m != st.res {
			t.Errorf("%d. awaited %f, got %f.", i, st.res, m)
		}
	}
}

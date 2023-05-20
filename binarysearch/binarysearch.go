// Copyright 2023 Tamás Gulácsi.
//
// SPDX-License-Identifier: MIT

// Package binarysearch implements the branchless binary search
// algorithm from https://orlp.net/blog/bitwise-binary-search/
//
// $ go test -bench=. -count=10 >/tmp/search.txt
// $ benchstat <(sed -e '/^BenchmarkSearch/d; s/Std//' /tmp/search.txt ) <(grep -v ^BenchmarkStdSearch /tmp/search.txt)
// goos: linux
// goarch: amd64
// pkg: github.com/tgulacsi/go/binarysearch
// cpu: 11th Gen Intel(R) Core(TM) i5-11320H @ 3.20GHz
//
//	│  StdSearch  │            Search             │
//	│   sec/op    │   sec/op     vs base          │
//
// Search-8   523.4n ± 7%   557.7n ± 6%  ~ (p=0.052 n=10)
package binarysearch

import "math/bits"

// Search uses binary search to find and return
// the smallest index i in [0, n) at which f(i) is true,
// assuming that on the range [0, n), f(i) == true implies f(i+1) == true.
//
// This is the same interface as sort.Search, but uses Orson Peters' branchless algorithm
// from  https://orlp.net/blog/bitwise-binary-search/
//
/*
template<typename It, typename T, typename Cmp>

	It lower_bound(It begin, It end, const T& value, Cmp comp) {
	    size_t n = end - begin;
	    if (n == 0) return begin;

	    size_t two_k = size_t(1) << (std::bit_width(n) - 1);
	    size_t b = comp(begin[n / 2], value) ? n - two_k : -1;
	    for (size_t bit = two_k >> 1; bit != 0; bit >>= 1) {
	        if (comp(begin[b + bit], value)) b += bit;
	    }
	    return begin + (b + 1);
	}
*/
func Search(n int, f func(int) bool) int {
	if n == 0 {
		return 0
	}
	twoK := 1 << (bits.Len(uint(n)) - 1)
	b := -1
	if !f(n / 2) {
		b = n - twoK
	}
	for bit := twoK >> 1; bit != 0; bit >>= 1 {
		if !f(b + bit) {
			b += bit
		}
	}
	return b + 1
}

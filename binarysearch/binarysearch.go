// Copyright 2023 Tamás Gulácsi.
//
// SPDX-License-Identifier: MIT

// From https://orlp.net/blog/bitwise-binary-search/
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
    size_t b = 0;
    for (size_t bit = std::bit_floor(n); bit != 0; bit >>= 1) {
        size_t i = (b | bit) - 1;
        if (i < n && comp(begin[i], value)) b |= bit;
    }
    return begin + b;
}
*/
func Search(n int, f func(int) bool) int {
	var b int
	for bit := stdFloor(n); bit != 0; bit >>= 1 {
		i := (b | bit) - 1
		if i < n && !f(i) {
			b |= bit
		}
	}
	return b
}

func stdFloor(n int) int {
	if n == 0 {
		return 0
	}
	l := bits.Len(uint(n))
	if l == 0 {
		return 0
	}
	return 1 << (bits.Len(uint(n)) - 1)
}

package cgo22

import "testing"

func BenchmarkCGO(b *testing.B) {
	for i := 0; i < b.N; i++ {
		call(1)
	}
}

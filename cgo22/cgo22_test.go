package cgo22

import "testing"

func BenchmarkCGONaive(b *testing.B) {
	for i := 0; i < b.N; i++ {
		callNaive(1)
	}
}
func BenchmarkCGOSliced(b *testing.B) {
	for i := 0; i < b.N; i++ {
		callSliced(1)
	}
}

func BenchmarkCGOCapped(b *testing.B) {
	for i := 0; i < b.N; i++ {
		callCapped(1)
	}
}

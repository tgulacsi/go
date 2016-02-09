package cgo22

/*
#include <stdio.h>
#include <inttypes.h>

uint32_t call(uint32_t **n, uint32_t **m) {
	uint32_t i = **n;
	uint32_t j = **m;
	return i * j + i;
}
*/
import "C"

var arr []*C.uint32_t

func init() {
	arr = make([]*C.uint32_t, 1000000)
	for i := range arr {
		arr[i] = (*C.uint32_t)(C.malloc(4))
		*arr[i] = C.uint32_t(i)
	}
}

func callNaive(batchLen int) {
	n := C.uint32_t(len(arr))
	for i := 0; i < batchLen; i++ {
		C.call(&arr[i%int(n)], &arr[(i+100)%int(n)])
	}
}

func callSliced(batchLen int) {
	n := C.uint32_t(len(arr))
	for i := 0; i < batchLen; i++ {
		i := i % int(n)
		carr := arr[i : i+1]
		i = (i + 100) % int(n)
		darr := arr[i : i+1]
		C.call(&carr[0], &darr[0])
	}
}
func callCapped(batchLen int) {
	n := C.uint32_t(len(arr))
	for i := 0; i < batchLen; i++ {
		i := i % int(n)
		carr := arr[i : i+1 : i+1]
		i = (i + 100) % int(n)
		darr := arr[i : i+1 : i+1]
		C.call(&carr[0], &darr[0])
	}
}

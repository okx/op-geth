//go:build !cgo

package core

var hashFunc = func(in *[8]uint64, capacity *[4]uint64, result *[4]uint64) {
	panic("poseidon hash not implemented without cgo")
}

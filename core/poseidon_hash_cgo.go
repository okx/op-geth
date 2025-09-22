//go:build cgo

package core

import poseidon "github.com/okx/poseidongold/go"

var hashFunc = poseidon.HashWithResult

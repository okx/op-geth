package core

import (
	"encoding/hex"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGeneratePowerOfTwoKeyRanges(t *testing.T) {

	ranges := generatePowerOfTwoKeyRanges(32)

	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000000000000", hex.EncodeToString(ranges[0].Start))
	assert.Equal(t, "0800000000000000000000000000000000000000000000000000000000000000", hex.EncodeToString(ranges[0].End))

	assert.Equal(t, "0800000000000000000000000000000000000000000000000000000000000000", hex.EncodeToString(ranges[1].Start))
	assert.Equal(t, "1000000000000000000000000000000000000000000000000000000000000000", hex.EncodeToString(ranges[1].End))

	assert.Equal(t, "f800000000000000000000000000000000000000000000000000000000000000", hex.EncodeToString(ranges[31].Start))
	assert.Equal(t, "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", hex.EncodeToString(ranges[31].End))

}

func TestLargestPowerOfTwo(t *testing.T) {
	assert.Equal(t, 1, largestPowerOfTwo(1))
	assert.Equal(t, 2, largestPowerOfTwo(3))
	assert.Equal(t, 16, largestPowerOfTwo(17))
	assert.Equal(t, 32, largestPowerOfTwo(32))
	assert.Equal(t, 32, largestPowerOfTwo(33))
}

func TestLog2Bits(t *testing.T) {
	assert.Equal(t, 5, log2Bits(32))
	assert.Equal(t, 0, log2Bits(1))
}

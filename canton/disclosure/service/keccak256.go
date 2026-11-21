package main

import (
	"encoding/binary"
	"math/bits"
)

// Keccak-256 as Daml keccak256 and Ethereum compute it: Keccak-f[1600], rate 136 bytes, padding
// byte 0x01. The standard library crypto/sha3 pads with 0x06 (FIPS 202), which gives other digests.

const keccak256RateBytes = 136

var keccakRoundConstants = [24]uint64{
	0x0000000000000001, 0x0000000000008082, 0x800000000000808a, 0x8000000080008000,
	0x000000000000808b, 0x0000000080000001, 0x8000000080008081, 0x8000000000008009,
	0x000000000000008a, 0x0000000000000088, 0x0000000080008009, 0x000000008000000a,
	0x000000008000808b, 0x800000000000008b, 0x8000000000008089, 0x8000000000008003,
	0x8000000000008002, 0x8000000000000080, 0x000000000000800a, 0x800000008000000a,
	0x8000000080008081, 0x8000000000008080, 0x0000000080000001, 0x8000000080008008,
}

// keccakRotationOffsets[x+5*y] is the rho rotation of lane (x, y).
var keccakRotationOffsets = [25]int{
	0, 1, 62, 28, 27,
	36, 44, 6, 55, 20,
	3, 10, 43, 25, 39,
	41, 45, 15, 21, 8,
	18, 2, 61, 56, 14,
}

// keccakF1600 permutes state in place. state[x+5*y] is lane (x, y).
func keccakF1600(state *[25]uint64) {
	var columnParity [5]uint64
	var rotatedLanes [25]uint64
	for round := 0; round < len(keccakRoundConstants); round++ {
		for x := 0; x < 5; x++ {
			columnParity[x] = state[x] ^ state[x+5] ^ state[x+10] ^ state[x+15] ^ state[x+20]
		}
		for x := 0; x < 5; x++ {
			thetaEffect := columnParity[(x+4)%5] ^ bits.RotateLeft64(columnParity[(x+1)%5], 1)
			for y := 0; y < 25; y += 5 {
				state[x+y] ^= thetaEffect
			}
		}
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {
				rotatedLanes[y+5*((2*x+3*y)%5)] = bits.RotateLeft64(state[x+5*y], keccakRotationOffsets[x+5*y])
			}
		}
		for y := 0; y < 25; y += 5 {
			for x := 0; x < 5; x++ {
				state[x+y] = rotatedLanes[x+y] ^ (^rotatedLanes[(x+1)%5+y] & rotatedLanes[(x+2)%5+y])
			}
		}
		state[0] ^= keccakRoundConstants[round]
	}
}

// keccak256 returns the Keccak-256 digest of input.
func keccak256(input []byte) [32]byte {
	var state [25]uint64
	absorbBlock := func(block []byte) {
		if len(block) != keccak256RateBytes {
			panic("disclosure-service: keccak256 block is not the rate")
		}
		for lane := 0; lane < keccak256RateBytes/8; lane++ {
			state[lane] ^= binary.LittleEndian.Uint64(block[8*lane:])
		}
		keccakF1600(&state)
	}
	fullBlockBytes := len(input) - len(input)%keccak256RateBytes
	for offset := 0; offset < fullBlockBytes; offset += keccak256RateBytes {
		absorbBlock(input[offset : offset+keccak256RateBytes])
	}
	var finalBlock [keccak256RateBytes]byte
	copy(finalBlock[:], input[fullBlockBytes:])
	finalBlock[len(input)-fullBlockBytes] ^= 0x01
	finalBlock[keccak256RateBytes-1] ^= 0x80
	absorbBlock(finalBlock[:])

	var digest [32]byte
	for lane := 0; lane < 4; lane++ {
		binary.LittleEndian.PutUint64(digest[8*lane:], state[lane])
	}
	return digest
}

package main

import "golang.org/x/crypto/sha3"

const keccak256DigestByteCount = 32

// keccak256Digest is Keccak-256 as Daml keccak256 and Ethereum compute it (padding byte 0x01).
// The standard library crypto/sha3 pads with 0x06 (FIPS 202), which gives other digests.
func keccak256Digest(input []byte) [keccak256DigestByteCount]byte {
	hasher := sha3.NewLegacyKeccak256()
	if hasher.Size() != keccak256DigestByteCount {
		panic("disclosure-service: Keccak-256 hasher size is not 32 bytes")
	}
	hasher.Write(input)
	var digest [keccak256DigestByteCount]byte
	if written := copy(digest[:], hasher.Sum(nil)); written != keccak256DigestByteCount {
		panic("disclosure-service: Keccak-256 digest is not 32 bytes")
	}
	return digest
}

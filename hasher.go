package slicesync

import (
	"crypto/sha1"
	hsh "hash"
)

// hasherName returns the implementation name of the whole file hasher
func hasherName() string {
	return "sha1"
}

// newHasher returns a Hash implementation for the whole file (usually SHA1)
func newHasher() hsh.Hash {
	return sha1.New()
}

// sliceHasherName returns the implementation name of the slice hasher
func sliceHasherName() string {
	return "sha1"
}

// newSliceHasher returns a Hash implementation for each slice 
// (SHA1 on naive implementation or rolling+hash in rsync's symulation)
func newSliceHasher() hsh.Hash {
	return sha1.New()
}

package slicesync

import (
	"crypto/md5"
	"crypto/sha1"
	"hash"
)

// namedHash is a hash.Hash with a name
type namedHash interface {
	hash.Hash
	Name() string
}

// simpleHash implements the namedHash for a single hash.Hash
type simpleHash struct {
	hash.Hash
	name string
}

// Name is the name of this namedHash
func (sh *simpleHash) Name() string {
	return sh.name
}

// Complex hash implements hash.Hash composed of a 32bit rolling hash and strong Hash
type complexHash struct {
	rolling hash.Hash32
	strong  hash.Hash
	name    string
}

// Write for complexHash's io.Writer implementation
func (ch *complexHash) Write(p []byte) (n int, err error) {
	n, err = ch.rolling.Write(p)
	if err != nil {
		return
	}
	return ch.strong.Write(p)
}

// Sum for complexHash's hash.Hash implementation:
// Appends the current hash to b and returns the resulting slice.
// It does not change the underlying hash state.
func (ch *complexHash) Sum(b []byte) []byte {
	sum := ch.rolling.Sum(b)
	return append(sum, ch.strong.Sum(b)...)
}

// Reset for complexHash's hash.Hash implementation:
// Resets the hash to one with zero bytes written.
func (ch *complexHash) Reset() {
	ch.rolling.Reset()
	ch.strong.Reset()
}

// Size for complexHash's hash.Hash implementation:
// Returns the number of bytes Sum will return.
func (ch *complexHash) Size() int {
	return ch.rolling.Size() + ch.strong.Size()
}

// BlockSize for complexHash's hash.Hash implementation:
// Returns the hash's underlying block size.
// The Write method must be able to accept any amount
// of data, but it may operate more efficiently if all writes
// are a multiple of the block size.
// (complexHash returns the string hash blocksize as it is the more cpu intensive one)
func (ch *complexHash) BlockSize() int {
	return ch.strong.BlockSize()
}

// Name is the name of this namedHash
func (ch *complexHash) Name() string {
	return ch.name
}

// newHasher returns a Hash implementation for the whole file (usually SHA1)
func newHasher() namedHash {
	return &simpleHash{sha1.New(), "sha1"}
}

// newSliceHasher returns a Hash implementation for each slice 
// (SHA1 on naive implementation or rolling+hash in rsync's symulation)
func newSliceHasher() namedHash {
	return &complexHash{New(), md5.New(), "adler32+md5"}
}

// autoHasher returns a newHasher() if the offset is 0 and slice is AUTOSIZE and newSliceHasher() otherwise
func autoHasher(offset, slice int64) namedHash {
	if offset == 0 && slice == AUTOSIZE {
		return newHasher()
	}
	return newSliceHasher()
}

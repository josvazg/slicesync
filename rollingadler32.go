package slicesync

const (
	mod = 65521
)

// The size of an Adler-32 checksum in bytes.
const Size = 4

// digest represents the partial evaluation of a checksum.
type digest struct {
	// invariant: (a < mod && b < mod) || a <= b
	// invariant: a + b + 255 <= 0xffffffff
	a, b uint32
}

func (d *digest) Reset() { d.a, d.b = 1, 0 }

// newRollingAdler32 returns a new hash.Hash32 computing the Adler-32 checksum.
func NewRollingAdler32() RollingHash32 {
	d := new(digest)
	d.Reset()
	return d
}

func (d *digest) Size() int { return Size }

func (d *digest) BlockSize() int { return 1 }

// Add p to the running checksum a, b.
func update(a, b uint32, p []byte) (aa, bb uint32) {
	for _, pi := range p {
		a += uint32(pi)
		b += a
		// invariant: a <= b
		if b > (0xffffffff-255)/2 {
			a %= mod
			b %= mod
			// invariant: a < mod && b < mod
		} else {
			// invariant: a + b + 255 <= 2 * b + 255 <= 0xffffffff
		}
	}
	return a, b
}

// Return the 32-bit checksum corresponding to a, b.
func finish(a, b uint32) uint32 {
	if b >= mod {
		a %= mod
		b %= mod
	}
	return b<<16 | a
}

func (d *digest) Write(p []byte) (nn int, err error) {
	d.a, d.b = update(d.a, d.b, p)
	return len(p), nil
}

func (d *digest) Sum32() uint32 { return finish(d.a, d.b) }

func (d *digest) Sum(in []byte) []byte {
	s := d.Sum32()
	in = append(in, byte(s>>24))
	in = append(in, byte(s>>16))
	in = append(in, byte(s>>8))
	in = append(in, byte(s))
	return in
}

// Checksum returns the Adler-32 checksum of data.
func Checksum(data []byte) uint32 { return finish(update(1, 0, data)) }

func roll(a, b uint32, window, oldest, newest uint32) (aa, bb uint32) {
	// As instructed at http://www.samba.org/~tridge/phd_thesis.pdf (pg. 55) and at golang nuts by Péter Szilágyi:
	//    (https://groups.google.com/forum/?fromgroups=#!topic/golang-nuts/ZiBcYH3Qw1g)
	// The idea is removing oldest "effect" on a and b while adding newest at the same time
	a += newest - oldest
	b += a - (window * oldest) - 1
	// invariant: a <= b
	if b > (0xffffffff-255)/2 {
		a %= mod
		b %= mod
		// invariant: a < mod && b < mod
	} else {
		// invariant: a + b + 255 <= 2 * b + 255 <= 0xffffffff
	}
	return a, b
}

// Roll32 will displace the window checksum window by one position, 
// taking old-byte from the beginning and adding new-byte at the end
// and returns the new current checksum 32bit state
func (d *digest) Roll32(window uint32, oldbyte, newbyte byte) uint32 {
	d.a, d.b = roll(d.a, d.b, window, uint32(oldbyte), uint32(newbyte))
	return d.Sum32()
}

// Roll calls Roll32 and returns the rolled checksum as a byte slice
func (d *digest) Roll(window uint32, oldbyte, newbyte byte) []byte {
	d.Roll32(window, oldbyte, newbyte)
	return d.Sum(nil)
}

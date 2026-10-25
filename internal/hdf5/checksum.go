// Adapted from SituCraft internal/hdf5/writer.go (Apache-2.0).
package hdf5

import "encoding/binary"

// lookup3 is Bob Jenkins' hashlittle (lookup3.c, 2006, public domain), the checksum of HDF5's
// metadata.
func lookup3(k []byte, initval uint32) uint32 {
	rot := func(x uint32, n uint) uint32 { return x<<n | x>>(32-n) }
	a := 0xdeadbeef + uint32(len(k)) + initval
	b, c := a, a
	for len(k) > 12 {
		a += binary.LittleEndian.Uint32(k)
		b += binary.LittleEndian.Uint32(k[4:])
		c += binary.LittleEndian.Uint32(k[8:])
		a -= c
		a ^= rot(c, 4)
		c += b
		b -= a
		b ^= rot(a, 6)
		a += c
		c -= b
		c ^= rot(b, 8)
		b += a
		a -= c
		a ^= rot(c, 16)
		c += b
		b -= a
		b ^= rot(a, 19)
		a += c
		c -= b
		c ^= rot(b, 4)
		b += a
		k = k[12:]
	}
	if len(k) == 0 {
		return c
	}
	var tail [12]byte
	copy(tail[:], k)
	a += binary.LittleEndian.Uint32(tail[:])
	b += binary.LittleEndian.Uint32(tail[4:])
	c += binary.LittleEndian.Uint32(tail[8:])
	c ^= b
	c -= rot(b, 14)
	a ^= c
	a -= rot(c, 11)
	b ^= a
	b -= rot(a, 25)
	c ^= b
	c -= rot(b, 16)
	a ^= c
	a -= rot(c, 4)
	b ^= a
	b -= rot(a, 14)
	c ^= b
	c -= rot(b, 24)
	return c
}

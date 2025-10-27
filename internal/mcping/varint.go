package mcping

import (
	"errors"
	"io"
)

// Minecraft encodes integers as little-endian base-128 varints: seven data bits
// per byte, high bit set while more bytes follow. A 32-bit value never needs
// more than five bytes, so anything longer is a malformed stream.
const maxVarIntBytes = 5

var errVarIntTooLong = errors.New("mcping: varint longer than 5 bytes")

func writeVarInt(w io.Writer, v int32) error {
	var buf [maxVarIntBytes]byte
	n := putVarInt(buf[:], v)
	_, err := w.Write(buf[:n])
	return err
}

func putVarInt(buf []byte, v int32) int {
	u := uint32(v)
	i := 0
	for {
		b := byte(u & 0x7f)
		u >>= 7
		if u != 0 {
			b |= 0x80
		}
		buf[i] = b
		i++
		if u == 0 {
			return i
		}
	}
}

func readVarInt(r io.ByteReader) (int32, error) {
	var result uint32
	for i := 0; i < maxVarIntBytes; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= uint32(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return int32(result), nil
		}
	}
	return 0, errVarIntTooLong
}

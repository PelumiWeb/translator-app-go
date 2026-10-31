package fish

import "encoding/binary"

// encoder writes MessagePack (msgpack.org), the binary cousin of JSON. Fish
// Audio accepts a voice sample inline only in MessagePack, not in JSON. The
// request needs only maps, arrays, strings, bytes and unsigned integers, a
// few dozen lines, which is less than a dependency would bring.
type encoder struct {
	buf []byte
}

// Each value starts with a type byte. Small values fit the length or the
// number into that byte itself ("fix" forms); larger ones follow it with a
// big-endian length or number of 1, 2 or 4 bytes.

func (e *encoder) mapHeader(entries int) {
	e.header(entries, 0x80, 15, 0xde, 0xdf)
}

func (e *encoder) arrayHeader(items int) {
	e.header(items, 0x90, 15, 0xdc, 0xdd)
}

func (e *encoder) str(s string) {
	switch n := len(s); {
	case n <= 31:
		e.buf = append(e.buf, 0xa0|byte(n))
	case n <= 0xff:
		e.buf = append(e.buf, 0xd9, byte(n))
	case n <= 0xffff:
		e.buf = binary.BigEndian.AppendUint16(append(e.buf, 0xda), uint16(n))
	default:
		e.buf = binary.BigEndian.AppendUint32(append(e.buf, 0xdb), uint32(n))
	}
	e.buf = append(e.buf, s...)
}

func (e *encoder) bytes(b []byte) {
	switch n := len(b); {
	case n <= 0xff:
		e.buf = append(e.buf, 0xc4, byte(n))
	case n <= 0xffff:
		e.buf = binary.BigEndian.AppendUint16(append(e.buf, 0xc5), uint16(n))
	default:
		e.buf = binary.BigEndian.AppendUint32(append(e.buf, 0xc6), uint32(n))
	}
	e.buf = append(e.buf, b...)
}

func (e *encoder) uint(n uint32) {
	switch {
	case n <= 0x7f:
		e.buf = append(e.buf, byte(n))
	case n <= 0xff:
		e.buf = append(e.buf, 0xcc, byte(n))
	case n <= 0xffff:
		e.buf = binary.BigEndian.AppendUint16(append(e.buf, 0xcd), uint16(n))
	default:
		e.buf = binary.BigEndian.AppendUint32(append(e.buf, 0xce), n)
	}
}

// header writes the start of a map or an array of n entries.
func (e *encoder) header(n int, fix byte, fixMax int, tag16, tag32 byte) {
	switch {
	case n <= fixMax:
		e.buf = append(e.buf, fix|byte(n))
	case n <= 0xffff:
		e.buf = binary.BigEndian.AppendUint16(append(e.buf, tag16), uint16(n))
	default:
		e.buf = binary.BigEndian.AppendUint32(append(e.buf, tag32), uint32(n))
	}
}

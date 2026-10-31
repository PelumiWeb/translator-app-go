package fish

import (
	"bytes"
	"strings"
	"testing"
)

// The expected bytes follow the MessagePack specification (msgpack.org).
func TestEncoder(t *testing.T) {
	tests := []struct {
		name  string
		write func(*encoder)
		want  []byte
	}{
		{"short string", func(e *encoder) { e.str("hi") }, []byte{0xa2, 'h', 'i'}},
		{"string of 32 bytes", func(e *encoder) { e.str(strings.Repeat("a", 32)) },
			append([]byte{0xd9, 32}, bytes.Repeat([]byte("a"), 32)...)},
		{"string of 256 bytes", func(e *encoder) { e.str(strings.Repeat("a", 256)) },
			append([]byte{0xda, 0x01, 0x00}, bytes.Repeat([]byte("a"), 256)...)},
		{"a few bytes", func(e *encoder) { e.bytes([]byte{1, 2}) }, []byte{0xc4, 2, 1, 2}},
		{"300 bytes", func(e *encoder) { e.bytes(make([]byte, 300)) },
			append([]byte{0xc5, 0x01, 0x2c}, make([]byte, 300)...)},
		{"70000 bytes", func(e *encoder) { e.bytes(make([]byte, 70_000)) },
			append([]byte{0xc6, 0x00, 0x01, 0x11, 0x70}, make([]byte, 70_000)...)},
		{"small number", func(e *encoder) { e.uint(5) }, []byte{0x05}},
		{"number up to 255", func(e *encoder) { e.uint(200) }, []byte{0xcc, 200}},
		{"16000", func(e *encoder) { e.uint(16_000) }, []byte{0xcd, 0x3e, 0x80}},
		{"70000", func(e *encoder) { e.uint(70_000) }, []byte{0xce, 0x00, 0x01, 0x11, 0x70}},
		{"map of 2", func(e *encoder) { e.mapHeader(2) }, []byte{0x82}},
		{"map of 16", func(e *encoder) { e.mapHeader(16) }, []byte{0xde, 0x00, 0x10}},
		{"array of 1", func(e *encoder) { e.arrayHeader(1) }, []byte{0x91}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e encoder
			tt.write(&e)
			if !bytes.Equal(e.buf, tt.want) {
				t.Errorf("got % x, want % x", head(e.buf), head(tt.want))
			}
		})
	}
}

func TestEncodeRequest(t *testing.T) {
	got := encodeRequest("hola", "es", []byte{1, 2}, "hi")

	var want []byte
	add := func(b ...byte) { want = append(want, b...) }
	str := func(s string) { add(0xa0 | byte(len(s))); add([]byte(s)...) }
	add(0x85) // a map of five entries
	str("text")
	str("hola")
	str("references")
	add(0x91, 0x82) // one reference, a map of two entries
	str("audio")
	add(0xc4, 2, 1, 2)
	str("text")
	str("hi")
	str("format")
	str("pcm")
	str("sample_rate")
	add(0xcd, 0x3e, 0x80)
	str("language")
	str("es")

	if !bytes.Equal(got, want) {
		t.Errorf("got  % x\nwant % x", got, want)
	}
}

func TestEncodeRequestLeavesOutAnUnknownLanguage(t *testing.T) {
	got := encodeRequest("hola", "", []byte{1}, "hi")

	if got[0] != 0x84 {
		t.Errorf("map header = %#x, want 0x84: four entries", got[0])
	}
	if bytes.Contains(got, []byte("language")) {
		t.Error("the request names a language")
	}
}

// head keeps failure messages readable for long values.
func head(b []byte) []byte {
	if len(b) > 16 {
		return b[:16]
	}
	return b
}

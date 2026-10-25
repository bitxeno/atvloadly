package asset

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestBandDataLen(t *testing.T) {
	cases := []struct {
		rowLen uint16
		arg6   uint16
		want   uint32
	}{
		// Older archives fit the band in 16 bits (Arg6 == 0).
		{rowLen: 100, arg6: 0, want: 100},
		{rowLen: 0xFFFF, arg6: 0, want: 0xFFFF},
		// Payload 2 flattened band: low word 57637, high word 1.
		{rowLen: 57637, arg6: 1, want: 57637 + 1<<16},
		// Payload 4 flattened bands: high words 2 and 3.
		{rowLen: 54876, arg6: 2, want: 54876 + 2<<16},
		{rowLen: 25195, arg6: 3, want: 25195 + 3<<16},
	}
	for _, c := range cases {
		v3 := &CUIThemePixelRenditionV3{RowDataLen: c.rowLen, Arg6: c.arg6}
		if got := v3.BandDataLen(); got != c.want {
			t.Fatalf("BandDataLen(rowLen=%d, arg6=%d) = %d, want %d", c.rowLen, c.arg6, got, c.want)
		}
	}
}

// TestDecodeImageV3BandOver64KB builds a version 3 rendition whose single
// band is exactly 64KB (RowDataLen == 0, Arg6 == 1). Reading only the low
// word yields zero bytes and fails; the full 32-bit length must be used.
func TestDecodeImageV3BandOver64KB(t *testing.T) {
	const width, height = 128, 128

	raw := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := (y*width + x) * 4
			raw[i] = byte(x)
			raw[i+1] = byte(y)
			raw[i+2] = 0x7F
			raw[i+3] = 0xFF
		}
	}

	buf := &bytes.Buffer{}
	write := func(v interface{}) {
		t.Helper()
		if err := binary.Write(buf, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	write([4]byte{'M', 'L', 'E', 'C'})                    // Tag
	write(uint32(3))                                      // Version
	write(uint32(kRenditionCompressionType_uncompressed)) // CompressionType
	write(uint32(1))                                      // RawDataLength: one band
	write(uint16(0))                                      // Arg1
	write(uint16(0))                                      // Arg2
	write(uint32(0))                                      // Arg3
	write(uint32(0))                                      // Arg4
	write(uint32(height))                                 // Height
	write(uint16(0))                                      // RowDataLen (low word of 65536)
	write(uint16(1))                                      // Arg6 (high word of 65536)
	if _, err := buf.Write(raw); err != nil {
		t.Fatal(err)
	}

	a := &asset{}
	csi := &csiheader{Width: width, Height: height}
	img, err := a.decodeImage("ARGB", buf, csi)
	if err != nil {
		t.Fatalf("decodeImage returned error: %v", err)
	}
	if got := img.Bounds(); got.Dx() != width || got.Dy() != height {
		t.Fatalf("bounds = %v, want %dx%d", got, width, height)
	}
	// Spot-check pixels (BGRA storage exposed as RGBA via RGBAAt swap).
	for _, p := range [][2]int{{0, 0}, {10, 20}, {127, 127}} {
		got := img.At(p[0], p[1])
		r, g, b, a8 := got.RGBA()
		_ = a8
		if uint8(r>>8) != 0x7F || uint8(g>>8) != byte(p[1]) || uint8(b>>8) != byte(p[0]) {
			t.Fatalf("pixel %v = (%d,%d,%d), want swapped pattern", p, r>>8, g>>8, b>>8)
		}
	}
}

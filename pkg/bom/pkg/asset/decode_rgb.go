package asset

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"io/ioutil"

	"github.com/bitxeno/atvloadly/pkg/bom/pkg/mreader"
	lzfse "github.com/bitxeno/lzfse-go"
)

// BGRA to RGBA
type BGRA struct {
	image.RGBA
}

func (p *BGRA) RGBAAt(x, y int) color.RGBA {
	c := p.RGBA.RGBAAt(x, y)
	return color.RGBA{R: c.B, G: c.G, B: c.R, A: c.A}
}

func (p *BGRA) At(x, y int) color.Color {
	return p.RGBAAt(x, y)
}

func (p *BGRA) SubImage(r image.Rectangle) image.Image {
	c := p.RGBA.SubImage(r).(*image.RGBA)
	return &BGRA{*c}
}

// gray alpha 8 bit
type GA8 struct {
	Pix    []uint8
	Stride int
	Rect   image.Rectangle
}

func (p *GA8) ColorModel() color.Model { return color.RGBAModel }

func (p *GA8) Bounds() image.Rectangle { return p.Rect }

func (p *GA8) At(x, y int) color.Color {
	return p.GA8At(x, y)
}

func (p *GA8) GA8At(x, y int) color.RGBA {
	if !(image.Point{x, y}.In(p.Rect)) {
		return color.RGBA{}
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+2 : i+2] // Small cap improves performance, see https://golang.org/issue/27857
	return color.RGBA{s[0], s[0], s[0], s[1]}
}

func (p *GA8) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*2
}

// maxDecodedDimension caps the pixel dimension of images decoded from an
// Assets.car. Catalogs can contain very large flattened/launch images whose
// decompressed size (width*height*4 bytes) would otherwise dominate memory
// usage during an icon lookup.
const maxDecodedDimension = 4096

// format: "ARGB", "BGRA", "GA8", "RGB5", "RGBW", "GA16"
func (a *asset) decodeImage(format string, d io.Reader, c *csiheader) (image.Image, error) {
	if c.Width > maxDecodedDimension || c.Height > maxDecodedDimension {
		return nil, fmt.Errorf("image too large to decode: %dx%d", c.Width, c.Height)
	}
	p := &CUIThemePixelRendition{}
	if err := binary.Read(d, binary.LittleEndian, &p.Tag); err != nil {
		return nil, err
	}
	if err := binary.Read(d, binary.LittleEndian, &p.Version); err != nil {
		return nil, err
	}
	if err := binary.Read(d, binary.LittleEndian, &p.CompressionType); err != nil {
		return nil, err
	}
	if err := binary.Read(d, binary.LittleEndian, &p.RawDataLength); err != nil {
		return nil, err
	}

	rawData := mreader.New()
	bytesPerPixel := imageBytesPerPixel(format)
	expectedTotalBytes := int(c.Width) * int(c.Height) * bytesPerPixel

	// decode header
	switch p.Version {
	case 0, 2:
		buf := make([]byte, p.RawDataLength)
		if _, err := io.ReadFull(d, buf); err != nil {
			return nil, err
		}
		decodedChunk, err := decodeCompressedChunk(p.CompressionType, buf, expectedTotalBytes)
		if err != nil {
			return nil, err
		}
		rawData.Add(io.NopCloser(bytes.NewReader(decodedChunk)))
	case 1, 3:
		for i := 0; i < int(p.RawDataLength); i++ {
			v3 := &CUIThemePixelRenditionV3{}
			if err := binary.Read(d, binary.LittleEndian, v3); err != nil {
				return nil, err
			}
			// v3.RowDataLen alone truncates bands larger than 64KB;
			// use the full 32-bit band length (see BandDataLen).
			buf := make([]byte, v3.BandDataLen())
			_, err := io.ReadFull(d, buf)
			if err != nil {
				return nil, err
			}

			expectedChunkBytes := int(v3.Height) * int(c.Width) * bytesPerPixel
			if expectedChunkBytes <= 0 {
				expectedChunkBytes = expectedTotalBytes
			}
			decodedChunk, err := decodeCompressedChunk(p.CompressionType, buf, expectedChunkBytes)
			if err != nil {
				return nil, err
			}
			rawData.Add(io.NopCloser(bytes.NewReader(decodedChunk)))
		}
	default:
		return nil, fmt.Errorf("unsupport version: %v", p.Version)
	}

	defer rawData.Close()
	return decodeImage(format, int(c.Width), int(c.Height), rawData)
}

// format: "JPEG", "HEIF"
func (a *asset) decodeJpg(format string, d io.Reader, c *csiheader) (image.Image, error) {
	p := &CUIRawPixelRendition{}
	if err := binary.Read(d, binary.LittleEndian, &p.Tag); err != nil {
		return nil, err
	}
	if err := binary.Read(d, binary.LittleEndian, &p.Version); err != nil {
		return nil, err
	}
	if err := binary.Read(d, binary.LittleEndian, &p.RawDataLength); err != nil {
		return nil, err
	}

	if p.Tag.String() != "DWAR" {
		return nil, fmt.Errorf("unsupport %s tag: %v", format, p.Tag.String())
	}

	// decode header
	buf := make([]byte, p.RawDataLength)
	if _, err := io.ReadFull(d, buf); err != nil {
		return nil, err
	}

	// JPEG renditions carry no dimensions in the catalog header, so peek
	// at the JPEG header before decoding to avoid decompressing oversized
	// images (the peak allocation of a decoded frame is width*height*4).
	if cfg, err := jpeg.DecodeConfig(bytes.NewBuffer(buf)); err == nil {
		if cfg.Width > maxDecodedDimension || cfg.Height > maxDecodedDimension {
			return nil, fmt.Errorf("image too large to decode: %dx%d", cfg.Width, cfg.Height)
		}
	}

	return jpeg.Decode(bytes.NewBuffer(buf))
}

func umCompression(t RenditionCompressionType, r io.Reader) (decoded io.ReadCloser, err error) {
	// upcompression raw data
	switch t {
	case kRenditionCompressionType_zip:
		return gzip.NewReader(r)
	case kRenditionCompressionType_lzfse, kRenditionCompressionType_blurred,
		kRenditionCompressionType_deepmap_lzfse, kRenditionCompressionType_deepmap_2:
		d, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}
		decodedBuf, err := decodeLZFSE(d, 0, false)
		if err != nil {
			return nil, err
		}
		decoded = io.NopCloser(bytes.NewBuffer(decodedBuf))
	case kRenditionCompressionType_uncompressed:
		decoded = io.NopCloser(r)
	// NOTE: do nothing
	// TODO
	// case kRenditionCompressionType_deepmap_2:
	default:
		return nil, fmt.Errorf("unsupport compression type: %v", t)
	}
	return
}

// The four-byte stream magics that begin a plain lzfse-family payload:
// "bvx6" (lzfse), "bvx2" (older lzfse) and "bvxn" (lzvn). Chunked and
// bitmap payloads do not carry them, and feeding those to lzfse's
// DecodeBuffer makes it grow its output buffer up to ~50MB per failed
// attempt, so DecodeBuffer is only ever called on data that actually
// starts with one of these magics.
func looksLikePlainLZFSE(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	if data[0] != 0x62 || data[1] != 0x76 || data[2] != 0x78 {
		return false
	}
	switch data[3] {
	case 0x36, 0x32, 0x6e: // bvx6, bvx2, bvxn
		return true
	}
	return false
}

func decodeLZFSE(data []byte, maxOutput int, allowPlain bool) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	// Try with metadata prefixes skipped first: many CoreUI payloads
	// prefix lzfse bytes with small headers, and decoding the full
	// payload fails before the correctly-aligned attempt succeeds.
	// Decompress into a fixed-size buffer so a failing attempt cannot
	// balloon memory (lzfse's DecodeBuffer grows its output buffer
	// up to ~50MB whenever a decode fails).
	for _, skip := range []int{0, 4, 8, 12, 16} {
		if len(data) <= skip {
			continue
		}
		if maxOutput > 0 {
			dst := make([]byte, maxOutput)
			if n, err := lzfse.DecodeZBMBuffer(data[skip:], dst); err == nil && n > 0 && n <= len(dst) {
				return dst[:n], nil
			}
		}
	}

	// Plain lzfse streams (identified by their magic header) are decoded
	// with DecodeBuffer. A valid stream succeeds on the first attempt
	// without growing its buffer; the output-size bound additionally
	// rejects any inflated result. Non-lzfse payloads (chunked, deepmap)
	// never reach DecodeBuffer, so they cannot trigger the ~50MB growth.
	if allowPlain {
		for _, skip := range []int{0, 4, 8, 12, 16} {
			if len(data) <= skip {
				continue
			}
			chunk := data[skip:]
			if !looksLikePlainLZFSE(chunk) {
				continue
			}
			out, err := lzfse.DecodeBuffer(chunk)
			if err == nil && len(out) > 0 && (maxOutput <= 0 || len(out) <= maxOutput*2) {
				return out, nil
			}
		}
	}

	if out, ok := decodeLZFSEChunked(data, binary.LittleEndian); ok {
		return out, nil
	}
	if out, ok := decodeLZFSEChunked(data, binary.BigEndian); ok {
		return out, nil
	}

	return nil, fmt.Errorf("lzfse decode failed: empty output")
}

func decodeLZFSEChunked(data []byte, order binary.ByteOrder) ([]byte, bool) {
	r := bytes.NewReader(data)
	out := bytes.NewBuffer(nil)

	for r.Len() > 0 {
		if r.Len() < 4 {
			return nil, false
		}
		var n uint32
		if err := binary.Read(r, order, &n); err != nil {
			return nil, false
		}
		if n == 0 || int(n) > r.Len() {
			return nil, false
		}
		chunk := make([]byte, n)
		if _, err := io.ReadFull(r, chunk); err != nil {
			return nil, false
		}
		decoded, err := lzfse.DecodeBuffer(chunk)
		if err != nil || len(decoded) == 0 {
			return nil, false
		}
		out.Write(decoded)
	}

	if out.Len() == 0 {
		return nil, false
	}
	return out.Bytes(), true
}

func decodeCompressedChunk(t RenditionCompressionType, data []byte, expectedLen int) ([]byte, error) {
	// lzfse payloads are decompressed into a fixed-size buffer first:
	// lzfse's DecodeBuffer grows its output buffer up to ~50MB when a
	// decode fails, so a single corrupt rendition would balloon memory.
	// With the expected pixel size known, decoding never allocates more
	// than expectedLen bytes.
	switch t {
	case kRenditionCompressionType_lzfse:
		return decodeLZFSE(data, expectedLen, true)
	case kRenditionCompressionType_blurred, kRenditionCompressionType_deepmap_lzfse,
		kRenditionCompressionType_deepmap_2:
		// Not plain lzfse streams: only try the bounded bitmap decoder,
		// never DecodeBuffer (which would balloon on these formats).
		return decodeLZFSE(data, expectedLen, false)
	}

	r, err := umCompression(t, bytes.NewBuffer(data))
	if err == nil {
		defer r.Close()
		decoded, readErr := ioutil.ReadAll(r)
		if readErr != nil {
			return nil, readErr
		}
		return decoded, nil
	}

	return nil, err
}

func imageBytesPerPixel(format string) int {
	switch format {
	case "ARGB", "BGRA":
		return 4
	case "GA8":
		return 2
	default:
		return 0
	}
}

func decodeImage(format string, width, height int, r io.Reader) (image.Image, error) {
	offset := 0
	rawData, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	switch format {
	case "ARGB", "BGRA":
		if v := len(rawData) - int(width*height*4); v != 0 {
			offset = v / int(height*4)
		}
		if offset < 0 {
			return nil, errors.New("error image content")
		}
		rect := image.Rectangle{
			Min: image.Point{0, 0},
			Max: image.Point{
				X: int(width),
				Y: int(height),
			},
		}
		bgra := &BGRA{image.RGBA{
			Pix:    rawData,
			Stride: (rect.Dx() + offset) * 4,
			Rect:   rect,
		}}
		return bgra, nil
	case "GA8":

		if v := len(rawData) - int(width*height*2); v != 0 {
			offset = v / int(height*2)
		}
		if offset < 0 {
			return nil, errors.New("error image content")
		}

		rect := image.Rectangle{
			Min: image.Point{0, 0},
			Max: image.Point{
				X: int(width),
				Y: int(height),
			},
		}
		bgra := &GA8{
			Pix:    rawData,
			Stride: (rect.Dx() + offset) * 2,
			Rect:   rect,
		}
		return bgra, nil
	case "JPEG":
		img, err := jpeg.Decode(r)
		if err != nil {
			return nil, errors.New("error image content")
		}
		return img, nil
	case "RGB5":
	case "RGBW":
	case "GA16":
	}
	return nil, fmt.Errorf("unsupport image format: %v", format)
}

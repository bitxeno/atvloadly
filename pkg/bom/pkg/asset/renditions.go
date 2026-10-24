package asset

import (
	"fmt"

	"github.com/bitxeno/atvloadly/pkg/bom/pkg/helper"
)

type csiheader struct {
	// uint32_t tag;								// 'CTSI'
	Tag helper.String4
	// uint32_t version;
	Version uint32
	// struct renditionFlags renditionFlags;
	RenditionFlags renditionFlags
	// uint32_t width;
	Width uint32
	// uint32_t height;
	Height uint32
	// uint32_t scaleFactor;
	ScaleFactor uint32 // 100 to @1x, 200 to @2x, 300 to @3x
	// uint32_t pixelFormat;
	PixelFormat helper.String4
	// type struct {
	// 	uint32_t colorSpaceID:4;
	// 	uint32_t reserved:28;
	// } colorSpace;
	ColorSpace colorSpace
	// struct csimetadata csimetadata;
	Csimetadata csimetadata
	// struct csibitmaplist csibitmaplist;
	Csibitmaplist csibitmaplist
}

type colorSpace uint32

//	struct {
//		ColorSpaceID uint32
//		Reserved     uint32
//	}
func (c colorSpace) ColorSpaceID() uint32 {
	return uint32(c) & (0b1111 << 0)
}
func (c colorSpace) Reserved() uint32 {
	return uint32(c) & (0b1111111111111111111111111111 << 4)
}

type renditionFlags uint32

//  struct renditionFlags {
// 	uint32_t isHeaderFlaggedFPO:1;
// 	uint32_t isExcludedFromContrastFilter:1;
// 	uint32_t isVectorBased:1;
// 	uint32_t isOpaque:1;
// 	uint32_t bitmapEncoding:4;
// 	uint32_t optOutOfThinning:1;
// 	uint32_t isFlippable:1;
// 	uint32_t isTintable:1;
// 	uint32_t preservedVectorRepresentation:1;
// 	uint32_t reserved:20;
// } __attribute__((packed));

func (r renditionFlags) IsHeaderFlaggedFPO() uint32 {
	return uint32(r) & (1 << 0)
}
func (r renditionFlags) IsExcludedFromContrastFilter() uint32 {
	return uint32(r) & (1 << 1)
}
func (r renditionFlags) IsVectorBased() uint32 {
	return uint32(r) & (1 << 2)
}
func (r renditionFlags) IsOpaque() uint32 {
	return uint32(r) & (1 << 3)
}
func (r renditionFlags) BitmapEncoding() uint32 {
	return uint32(r) & (0b1111 << 4)
}
func (r renditionFlags) OptOutOfThinning() uint32 {
	return uint32(r) & (1 << 8)
}
func (r renditionFlags) IsFlippable() uint32 {
	return uint32(r) & (1 << 9)
}
func (r renditionFlags) IsTintable() uint32 {
	return uint32(r) & (1 << 10)
}
func (r renditionFlags) PreservedVectorRepresentation() uint32 {
	return uint32(r) & (1 << 11)
}
func (r renditionFlags) Reserved() uint32 {
	return uint32(r) & (0b11111111111111111111 << 12)
}

type csimetadata struct {
	// uint32_t modtime;
	Modtime uint32
	// uint16_t layout;
	Layout RenditionLayoutType
	// uint16_t zero;
	Zero uint16
	// char name[128];
	Name helper.String128
}

type csibitmaplist struct {
	// uint32_t tvlLength;			// Length of all the TLV following the csiheader
	TvlLength uint32
	// uint32_t unknown;
	Unknown uint32
	// uint32_t zero;
	Zero uint32
	// uint32_t renditionLength;
	RenditionLength uint32
}

// TLV (Type-length-value)
type tlvValue struct {
	TlvTag    uint32
	TlvLength uint32
	TlvValues []uint8
}

type RenditionTLVType uint32

const (
	kRenditionTLVType_Slices              = RenditionTLVType(0x3E9)
	kRenditionTLVType_Metrics             = RenditionTLVType(0x3EB)
	kRenditionTLVType_BlendModeAndOpacity = RenditionTLVType(0x3EC)
	kRenditionTLVType_UTI                 = RenditionTLVType(0x3ED)
	kRenditionTLVType_EXIFOrientation     = RenditionTLVType(0x3EE)
	kRenditionTLVType_ExternalTags        = RenditionTLVType(0x3F0)
	kRenditionTLVType_Frame               = RenditionTLVType(0x3F1)
)

func (r RenditionTLVType) String() string {
	switch r {
	case kRenditionTLVType_Slices:
		return "Slices"
	case kRenditionTLVType_Metrics:
		return "Metrics"
	case kRenditionTLVType_BlendModeAndOpacity:
		return "BlendModeAndOpacity"
	case kRenditionTLVType_UTI:
		return "UTI"
	case kRenditionTLVType_EXIFOrientation:
		return "EXIFOrientation"
	case kRenditionTLVType_ExternalTags:
		return "ExternalTags"
	case kRenditionTLVType_Frame:
		return "Frame"
	default:
		return fmt.Sprintf("Unknown 0x%04X", uint32(r))
	}
}

type RenditionLayoutType uint16

const (
	kRenditionLayoutType_TextEffect = RenditionLayoutType(0x007)
	kRenditionLayoutType_Vector     = RenditionLayoutType(0x009)

	kRenditionLayoutType_Data              = RenditionLayoutType(0x3E8)
	kRenditionLayoutType_ExternalLink      = RenditionLayoutType(0x3E9)
	kRenditionLayoutType_LayerStack        = RenditionLayoutType(0x3EA)
	kRenditionLayoutType_InternalReference = RenditionLayoutType(0x3EB)
	kRenditionLayoutType_PackedImage       = RenditionLayoutType(0x3EC)
	kRenditionLayoutType_NameList          = RenditionLayoutType(0x3ED)
	kRenditionLayoutType_UnknownAddObject  = RenditionLayoutType(0x3EE)
	kRenditionLayoutType_Texture           = RenditionLayoutType(0x3EF)
	kRenditionLayoutType_TextureImage      = RenditionLayoutType(0x3F0)
	kRenditionLayoutType_Color             = RenditionLayoutType(0x3F1)
	kRenditionLayoutType_MultisizeImage    = RenditionLayoutType(0x3F2)
	kRenditionLayoutType_LayerReference    = RenditionLayoutType(0x3F4)
	kRenditionLayoutType_ContentRendition  = RenditionLayoutType(0x3F5)
	kRenditionLayoutType_RecognitionObject = RenditionLayoutType(0x3F6)
)

type CUIThemePixelRendition struct {
	// uint32_t tag;					// 'CELM'
	Tag helper.String4
	// uint32_t version;
	Version uint32
	// uint32_t compressionType;
	CompressionType RenditionCompressionType
	// uint32_t rawDataLength;
	RawDataLength uint32
	// uint8_t rawData[]; // rawData or []CUIThemePixelRenditionV3
	// RawData []byte
}

type CUIRawPixelRendition struct {
	// uint32_t tag;					// 'CELM'
	Tag helper.String4
	// uint32_t version;
	Version uint32
	// uint32_t rawDataLength;
	RawDataLength uint32
	// uint8_t rawData[]; // rawData or []CUIThemePixelRenditionV3
	// RawData []byte
}

// Each entry describes one band (a horizontal slice of rows) of a
// version 1/3 pixel rendition. Bands produced by newer asset catalog
// tools exceed 64KB, which does not fit in RowDataLen alone: the band
// data length is a 32-bit value whose low word is RowDataLen and whose
// high word is Arg6. Older archives have Arg6 == 0.
type CUIThemePixelRenditionV3 struct {
	Arg1       uint16
	Arg2       uint16
	Arg3       uint32
	Arg4       uint32
	Height     uint32
	RowDataLen uint16
	Arg6       uint16
}

// BandDataLen returns the full band data length in bytes: the 32-bit
// value composed of RowDataLen (low word) and Arg6 (high word).
func (v *CUIThemePixelRenditionV3) BandDataLen() uint32 {
	return uint32(v.RowDataLen) | uint32(v.Arg6)<<16
}

// As seen in _CUIConvertCompressionTypeToString
type RenditionCompressionType uint32

const (
	kRenditionCompressionType_uncompressed  = RenditionCompressionType(0)
	kRenditionCompressionType_rle           = RenditionCompressionType(1)
	kRenditionCompressionType_zip           = RenditionCompressionType(2)
	kRenditionCompressionType_lzvn          = RenditionCompressionType(3)
	kRenditionCompressionType_lzfse         = RenditionCompressionType(4)
	kRenditionCompressionType_jpeg_lzfse    = RenditionCompressionType(5)
	kRenditionCompressionType_blurred       = RenditionCompressionType(6)
	kRenditionCompressionType_astc          = RenditionCompressionType(7)
	kRenditionCompressionType_palette_img   = RenditionCompressionType(8)
	kRenditionCompressionType_deepmap_lzfse = RenditionCompressionType(9)
	kRenditionCompressionType_unknow        = RenditionCompressionType(10) // unknow
	kRenditionCompressionType_deepmap_2     = RenditionCompressionType(11)
)

func (t RenditionCompressionType) String() string {
	switch t {
	case kRenditionCompressionType_uncompressed:
		return "uncompressed"
	case kRenditionCompressionType_rle:
		return "rle"
	case kRenditionCompressionType_zip:
		return "zip"
	case kRenditionCompressionType_lzvn:
		return "lzvn"
	case kRenditionCompressionType_lzfse:
		return "lzfse"
	case kRenditionCompressionType_jpeg_lzfse:
		return "jpeg-lzfse"
	case kRenditionCompressionType_blurred:
		return "blurred"
	case kRenditionCompressionType_astc:
		return "astc"
	case kRenditionCompressionType_palette_img:
		return "palette-img"
	case kRenditionCompressionType_deepmap_lzfse:
		return "deepmap-lzfse"
	case kRenditionCompressionType_deepmap_2:
		return "deepmap-2"
	default:
		return fmt.Sprintf("Unknown type: %v", int(t))
	}
}

type kCoreThemeIdiom uint32

const (
	kCoreThemeIdiomUniversal = kCoreThemeIdiom(0)
	kCoreThemeIdiomPhone     = kCoreThemeIdiom(1)
	kCoreThemeIdiomPad       = kCoreThemeIdiom(2)
	kCoreThemeIdiomTV        = kCoreThemeIdiom(3)
	kCoreThemeIdiomCar       = kCoreThemeIdiom(4)
	kCoreThemeIdiomWatch     = kCoreThemeIdiom(5)
	kCoreThemeIdiomMarketing = kCoreThemeIdiom(6)

	kCoreThemeIdiomMax = kCoreThemeIdiom(7)
)

var kCoreThemeIdiomNames = [kCoreThemeIdiomMax]string{"", "phone", "pad", "tv", "car", "watch", "marketing"}

type CUIThemeMultisizeImageSetRendition struct {
	// uint32_t tag;					// 'SISM'
	Tag    helper.String4
	Idiom  kCoreThemeIdiom // ?
	Scale  uint32          // ?
	Width  uint32
	Heigth uint32
	Index  uint32
}

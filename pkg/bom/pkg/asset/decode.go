package asset

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"io/ioutil"
	"log"
	"strings"

	"github.com/bitxeno/atvloadly/pkg/bom/pkg/bom"
	"github.com/bitxeno/atvloadly/pkg/bom/pkg/helper"
)

// const typeMap = map[string]interface{}{
// "CARHEADER": CarHeader,
// "EXTENDED_METADATA": CarextendedMetadata,
// "KEYFORMAT": RenditionKeyFmt,
// "CARGLOBALS":
// "KEYFORMATWORKAROUND":
// "EXTERNAL_KEYS":

// // tree
// "FACETKEYS": Tree,
// "RENDITIONS": Tree,
// "APPEARANCEKEYS": Tree,
// "COLORS": Tree,
// "FONTS": Tree,
// "FONTSIZES": Tree,
// "GLYPHS": Tree,
// "BEZELS": Tree,
// "BITMAPKEYS": Tree,
// "ELEMENT_INFO": Tree,
// "PART_INFO": Tree,
// }

type AssetParser interface {
}

type asset struct {
	bom bom.BomParser
	// renditionFilter, when set, is consulted before decoding each
	// rendition in Renditions. Returning false skips the rendition
	// entirely, which avoids decompressing image data that the caller
	// does not need (e.g. looking for a single icon in a large
	// Assets.car would otherwise decode every image in the catalog).
	renditionFilter func(RenditionAttrs) bool
}

func (a *asset) SetRenditionFilter(f func(RenditionAttrs) bool) {
	a.renditionFilter = f
}

func New(b bom.BomParser) *asset {
	return &asset{bom: b}
}

func NewWithReadSeeker(r io.ReadSeeker) (*asset, error) {
	b := bom.New(r)
	if err := b.Parse(); err != nil {
		return nil, err
	}
	return &asset{bom: b}, nil
}

func (a *asset) read(name string, order binary.ByteOrder, p interface{}) error {
	d, err := a.bom.ReadBlock(name)
	if err != nil {
		return err
	}

	if err := binary.Read(d, order, p); err != nil {
		return err
	}

	return nil
}

func (a *asset) CarHeader() (*CarHeader, error) {
	c := &CarHeader{}
	if err := a.read("CARHEADER", binary.LittleEndian, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (a *asset) KeyFormat() (*RenditionKeyFmt, error) {
	buf, err := a.bom.ReadBlock("KEYFORMAT")
	if err != nil {
		return nil, err
	}

	c := &RenditionKeyFmt{}
	if err := binary.Read(buf, binary.LittleEndian, &c.Tag); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &c.Version); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &c.MaximumRenditionKeyTokenCount); err != nil {
		return nil, err
	}
	// read key tokens
	c.RenditionKeyTokens = make([]RenditionAttributeType, c.MaximumRenditionKeyTokenCount)
	for i := uint32(0); i < c.MaximumRenditionKeyTokenCount; i++ {
		t := RenditionAttributeType(0)
		if err := binary.Read(buf, binary.LittleEndian, &t); err != nil {
			return nil, err
		}
		c.RenditionKeyTokens[i] = t
	}

	return c, nil
}

func (a *asset) ExtendedMetadata() (*CarextendedMetadata, error) {
	c := &CarextendedMetadata{}
	if err := a.read("EXTENDED_METADATA", binary.BigEndian, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (a *asset) AppearanceKeys() (map[string]uint16, error) {
	keys := map[string]uint16{}
	if err := a.bom.ReadTree("APPEARANCEKEYS", func(k io.Reader, d io.Reader) error {
		value := uint16(0)
		if err := binary.Read(d, binary.BigEndian, &value); err != nil {
			return err
		}
		key, err := ioutil.ReadAll(k)
		if err != nil {
			return err
		}
		keys[string(key)] = value
		return nil
	}); err != nil {
		return nil, err
	}
	return keys, nil
}

func (a *asset) FacetKeys() (map[string]RenditionAttrs, error) {
	data := map[string]RenditionAttrs{}
	if err := a.bom.ReadTree("FACETKEYS", func(k io.Reader, d io.Reader) error {
		attrs := map[RenditionAttributeType]uint16hex{}
		t := &Renditionkeytoken{}
		if err := binary.Read(d, binary.LittleEndian, &t.CursorHotSpot); err != nil {
			return err
		}
		if err := binary.Read(d, binary.LittleEndian, &t.NumberOfAttributes); err != nil {
			return err
		}
		t.Attributes = make([]RenditionAttribute, t.NumberOfAttributes)
		for i := uint16(0); i < t.NumberOfAttributes; i++ {
			a := RenditionAttribute{}
			if err := binary.Read(d, binary.LittleEndian, &a); err != nil {
				return err
			}
			t.Attributes[i] = a
			attrs[RenditionAttributeType(a.Name)] = a.Value
		}
		name, err := ioutil.ReadAll(k)
		if err != nil {
			return err
		}
		data[string(name)] = attrs
		return nil
	}); err != nil {
		return nil, err
	}
	return data, nil
}

func (a *asset) BitmapKeys() error {
	if err := a.bom.ReadTree("BITMAPKEYS", func(k io.Reader, d io.Reader) error {
		// TODO: handle bitmapKeys
		kk, err := ioutil.ReadAll(k)
		if err != nil {
			return err
		}
		dd, err := ioutil.ReadAll(d)
		if err != nil {
			return err
		}
		log.Printf("%v: %v", kk, dd)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

type RenditionAttrs map[RenditionAttributeType]uint16hex

type RenditionType int

const (
	RenditionTypeImage = RenditionType(0)
	RenditionTypeData  = RenditionType(1)
	RenditionTypeColor = RenditionType(3)
)

type RenditionCallback struct {
	Attrs RenditionAttrs
	Type  RenditionType
	Err   error
	Image image.Image
	Name  string
}

const appIconPart = uint16hex(0x00DC)

// ImageOptions narrows image selection when multiple renditions share a lookup name.
type ImageOptions struct {
	Idiom string
	Scale int
}

type imageCandidate struct {
	name          string
	renditionName string
	image         image.Image
	attrs         RenditionAttrs
}

// ImageCandidateInfo is a public, simplified view of an internal imageCandidate.
// It intentionally omits internal attrs but includes image dimensions.
type ImageCandidateInfo struct {
	Name          string
	RenditionName string
	Image         image.Image
	Width         int
	Height        int
}

func (a *asset) Renditions(loop func(cb *RenditionCallback) (stop bool)) error {
	kf, err := a.KeyFormat()
	if err != nil {
		return err
	}

	if err := a.bom.ReadTree("RENDITIONS", func(k io.Reader, d io.Reader) error {
		attrs := RenditionAttrs{}
		for i := 0; i < len(kf.RenditionKeyTokens); i++ {
			v := uint16hex(0)
			binary.Read(k, binary.LittleEndian, &v)
			attrs[kf.RenditionKeyTokens[i]] = v
		}

		c := &csiheader{}
		if err := binary.Read(d, binary.LittleEndian, c); err != nil {
			return err
		}

		// TODO: skip TLV for now
		tmp := make([]byte, c.Csibitmaplist.TvlLength)
		if _, err := io.ReadFull(d, tmp); err != nil {
			return err
		}

		// log.Printf("%s: %s: %s attrs: %+v TVL: %+v %v", c.Tag.String(), c.PixelFormat.String(), c.Csimetadata.Name.String(), attrs, c, len(tmp))
		// Skip renditions the caller does not want before decompressing
		// them. Decoding every rendition of a large Assets.car can
		// allocate hundreds of megabytes of pixel data for a single
		// icon lookup.
		if a.renditionFilter != nil && !a.renditionFilter(attrs) {
			return nil
		}
		// string value reverse
		format := strings.TrimSpace(string(helper.Reverse(c.PixelFormat[:])))
		switch format {
		case "DATA":
			// TODO:
			// log.Print("TODO: handle DATA")
			return nil
		case "JPEG", "HEIF":
			cb := &RenditionCallback{
				Attrs: attrs,
				Type:  RenditionTypeImage,
				Name:  c.Csimetadata.Name.String(),
			}

			img, err := a.decodeJpg(format, d, c)
			if err != nil {
				cb.Err = err
				stop := loop(cb)
				if stop {
					return err
				}
			}
			cb.Image = img
			stop := loop(cb)
			if stop {
				return nil
			}
		case "ARGB", "BGRA", "GA8", "RGB5", "RGBW", "GA16":
			// TODO:
			cb := &RenditionCallback{
				Attrs: attrs,
				Type:  RenditionTypeImage,
				Name:  c.Csimetadata.Name.String(),
			}

			img, err := a.decodeImage(format, d, c)
			if err != nil {
				cb.Err = err
				stop := loop(cb)
				if stop {
					return err
				}
			}
			cb.Image = img
			stop := loop(cb)
			if stop {
				return nil
			}
		case string([]byte{0, 0, 0, 0}):
			switch c.Csimetadata.Layout {
			case kRenditionLayoutType_Color:
				// TODO:
			case kRenditionLayoutType_MultisizeImage:
				// _CUIThemeMultisizeImageSetRendition
				// TODO:
				p := CUIThemeMultisizeImageSetRendition{}
				if err := binary.Read(d, binary.LittleEndian, &p); err != nil {
					return err
				}
			}
		default:
			log.Printf("unknown rendition with pixel format: %v", c.PixelFormat.String())
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (a *asset) ImageWalker(loop func(name string, img image.Image) (end bool)) error {
	c, err := a.FacetKeys()
	if err != nil {
		return err
	}
	type item struct {
		name  string
		attrs RenditionAttrs
		done  bool
	}
	idMap := map[uint16hex]*item{}
	for k, v := range c {
		id, ok := v[kRenditionAttributeType_Identifier]
		if !ok {
			continue
		}
		idMap[id] = &item{name: k, attrs: v}
	}
	return a.Renditions(func(cb *RenditionCallback) (stop bool) {
		if cb.Err != nil {
			return false
		}
		if cb.Type != RenditionTypeImage {
			return false
		}
		id, ok := cb.Attrs[kRenditionAttributeType_Identifier]
		if !ok {
			return false
		}

		row, ok := idMap[id]
		if !ok || row.done {
			return false
		}
		row.done = true

		return loop(row.name, cb.Image)
	})
}

func appIconFacetNames(facets map[string]RenditionAttrs) map[string]struct{} {
	names := map[string]struct{}{}
	for name, attrs := range facets {
		part, ok := attrs[kRenditionAttributeType_Part]
		if ok && part == appIconPart {
			names[name] = struct{}{}
		}
	}
	if len(names) > 0 {
		return names
	}
	for name := range facets {
		normalized := strings.ToLower(strings.TrimSpace(name))
		if strings.HasPrefix(normalized, "app icon") || strings.HasPrefix(normalized, "appicon") {
			names[name] = struct{}{}
		}
	}
	return names
}

func isAppIconLookup(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "appicon", "icon":
		return true
	default:
		return false
	}
}

func parseImageIdiom(name string) (uint16hex, error) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "universal" {
		return uint16hex(kCoreThemeIdiomUniversal), nil
	}
	for value, idiom := range kCoreThemeIdiomNames {
		if idiom == normalized {
			return uint16hex(value), nil
		}
	}
	return 0, fmt.Errorf("unknown idiom: %v", name)
}

func isBetterImageCandidate(current imageCandidate, best imageCandidate) bool {
	currentBounds := current.image.Bounds()
	bestBounds := best.image.Bounds()
	currentArea := currentBounds.Dx() * currentBounds.Dy()
	bestArea := bestBounds.Dx() * bestBounds.Dy()
	if currentArea != bestArea {
		return currentArea > bestArea
	}
	if currentBounds.Dx() != bestBounds.Dx() {
		return currentBounds.Dx() > bestBounds.Dx()
	}
	if currentBounds.Dy() != bestBounds.Dy() {
		return currentBounds.Dy() > bestBounds.Dy()
	}
	return current.attrs[kRenditionAttributeType_Scale] > best.attrs[kRenditionAttributeType_Scale]
}

func pickBestImageCandidate(candidates []imageCandidate, options ImageOptions) (*imageCandidate, error) {
	var (
		best       *imageCandidate
		idiomValue uint16hex
		err        error
	)
	if strings.TrimSpace(options.Idiom) != "" {
		idiomValue, err = parseImageIdiom(options.Idiom)
		if err != nil {
			return nil, err
		}
	}
	for i := range candidates {
		candidate := &candidates[i]
		if strings.TrimSpace(options.Idiom) != "" && candidate.attrs[kRenditionAttributeType_Idiom] != idiomValue {
			continue
		}
		if options.Scale > 0 && candidate.attrs[kRenditionAttributeType_Scale] != uint16hex(options.Scale) {
			continue
		}
		if best == nil || isBetterImageCandidate(*candidate, *best) {
			best = candidate
		}
	}
	return best, nil
}

func formatImageOptions(options ImageOptions) string {
	parts := []string{}
	if idiom := strings.TrimSpace(options.Idiom); idiom != "" {
		parts = append(parts, fmt.Sprintf("idiom=%s", idiom))
	}
	if options.Scale > 0 {
		parts = append(parts, fmt.Sprintf("scale=%d", options.Scale))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf(" (%s)", strings.Join(parts, ", "))
}

func (a *asset) facetNamesForImage(name string, facets map[string]RenditionAttrs) (map[string]struct{}, error) {
	if _, ok := facets[name]; ok {
		return map[string]struct{}{name: {}}, nil
	}

	if !isAppIconLookup(name) {
		return nil, fmt.Errorf("not found: %v", name)
	}

	appIcons := appIconFacetNames(facets)
	if len(appIcons) == 0 {
		return nil, fmt.Errorf("not found: %v", name)
	}
	return appIcons, nil
}

func (a *asset) imageCandidates(name string) ([]imageCandidate, error) {
	facets, err := a.FacetKeys()
	if err != nil {
		return nil, err
	}

	facetNames, err := a.facetNamesForImage(name, facets)
	if err != nil {
		return nil, err
	}

	ids := map[uint16hex]string{}
	for facetName := range facetNames {
		attrs := facets[facetName]
		id, ok := attrs[kRenditionAttributeType_Identifier]
		if !ok {
			continue
		}
		ids[id] = facetName
	}

	candidates := []imageCandidate{}
	// Only decode renditions that belong to a matching facet. Renditions
	// iterates every entry in the catalog and would otherwise decompress
	// all images (a large Assets.car can hold hundreds of megabytes of
	// pixel data) just to find one icon.
	a.SetRenditionFilter(func(attrs RenditionAttrs) bool {
		id, ok := attrs[kRenditionAttributeType_Identifier]
		if !ok {
			return false
		}
		_, ok = ids[id]
		return ok
	})
	err = a.Renditions(func(cb *RenditionCallback) (stop bool) {
		if cb.Err != nil || cb.Type != RenditionTypeImage || cb.Image == nil {
			return false
		}
		id, ok := cb.Attrs[kRenditionAttributeType_Identifier]
		if !ok {
			return false
		}
		facetName, ok := ids[id]
		if !ok {
			return false
		}
		candidates = append(candidates, imageCandidate{
			name:          facetName,
			renditionName: cb.Name,
			image:         cb.Image,
			attrs:         cb.Attrs,
		})
		return false
	})
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("not found: %v", name)
	}
	return candidates, nil
}

// ImageWithOptions returns the largest decoded image that matches the name and optional idiom or scale filters.
func (a *asset) ImageWithOptions(name string, options ImageOptions) (image.Image, error) {
	candidates, err := a.imageCandidates(name)
	if err != nil {
		return nil, err
	}

	best, err := pickBestImageCandidate(candidates, options)
	if err != nil {
		return nil, err
	}
	if best == nil {
		return nil, fmt.Errorf("not found: %v%v", name, formatImageOptions(options))
	}
	return best.image, nil
}

func (a *asset) Image(name string) (image.Image, error) {
	return a.ImageWithOptions(name, ImageOptions{})
}

// ImageCandidates returns the decoded image renditions matching name,
// with their dimensions. Only renditions whose facet matches name are
// decoded; everything else in the catalog is skipped without
// decompressing, so a large Assets.car costs a fraction of the memory it
// would if every image were decoded.
func (a *asset) ImageCandidates(name string) ([]ImageCandidateInfo, error) {
	candidates, err := a.imageCandidates(name)
	if err != nil {
		return nil, err
	}
	out := make([]ImageCandidateInfo, 0, len(candidates))
	for _, c := range candidates {
		w, h := 0, 0
		if c.image != nil {
			b := c.image.Bounds()
			w = b.Dx()
			h = b.Dy()
		}
		out = append(out, ImageCandidateInfo{
			Name:          c.name,
			RenditionName: c.renditionName,
			Image:         c.image,
			Width:         w,
			Height:        h,
		})
	}
	return out, nil
}

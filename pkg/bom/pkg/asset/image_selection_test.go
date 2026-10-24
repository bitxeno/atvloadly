package asset

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestPickBestImageCandidatePrefersLargest(t *testing.T) {
	candidates := []imageCandidate{
		newImageCandidate("Icon-Small@2x.png", 58, 58, 1, 2),
		newImageCandidate("Icon-Small-60@3x.png", 180, 180, 1, 3),
		newImageCandidate("1024x1024.png", 1024, 1024, 6, 1),
	}

	best, err := pickBestImageCandidate(candidates, ImageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if best == nil {
		t.Fatal("expected a best image candidate")
	}
	if best.renditionName != "1024x1024.png" {
		t.Fatalf("unexpected best candidate: got %s", best.renditionName)
	}
}

func TestPickBestImageCandidateFiltersByIdiomAndScale(t *testing.T) {
	candidates := []imageCandidate{
		newImageCandidate("Icon-Small-40@2x.png", 80, 80, 1, 2),
		newImageCandidate("Icon-Small-60@3x.png", 180, 180, 1, 3),
		newImageCandidate("Icon-83.5@2x.png", 167, 167, 2, 2),
	}

	best, err := pickBestImageCandidate(candidates, ImageOptions{Idiom: "pad", Scale: 2})
	if err != nil {
		t.Fatal(err)
	}
	if best == nil {
		t.Fatal("expected a filtered image candidate")
	}
	if best.renditionName != "Icon-83.5@2x.png" {
		t.Fatalf("unexpected filtered candidate: got %s", best.renditionName)
	}
}

func TestPickBestImageCandidateRejectsUnknownIdiom(t *testing.T) {
	_, err := pickBestImageCandidate([]imageCandidate{newImageCandidate("Icon.png", 60, 60, 1, 2)}, ImageOptions{Idiom: "desktop"})
	if err == nil {
		t.Fatal("expected an error for unknown idiom")
	}
}

func TestAppIconFacetNamesFallsBackToNamePrefix(t *testing.T) {
	facets := map[string]RenditionAttrs{
		"App Icon/Front/Content":  {},
		"App Icon/Middle/Content": {},
		"Top Shelf Image":         {},
	}

	names := appIconFacetNames(facets)
	if _, ok := names["App Icon/Front/Content"]; !ok {
		t.Fatal("expected app icon front facet to be included")
	}
	if _, ok := names["App Icon/Middle/Content"]; !ok {
		t.Fatal("expected app icon middle facet to be included")
	}
	if _, ok := names["Top Shelf Image"]; ok {
		t.Fatal("did not expect top shelf image to be included")
	}
}

func TestDecodeImageSupportsBGRA(t *testing.T) {
	img, err := decodeImage("BGRA", 1, 1, bytes.NewReader([]byte{1, 2, 3, 4}))
	if err != nil {
		t.Fatal(err)
	}

	got := color.RGBAModel.Convert(img.At(0, 0)).(color.RGBA)
	want := color.RGBA{R: 3, G: 2, B: 1, A: 4}
	if got != want {
		t.Fatalf("unexpected color: got %+v, want %+v", got, want)
	}
}

func newImageCandidate(name string, width, height int, idiom, scale uint16) imageCandidate {
	return imageCandidate{
		renditionName: name,
		image:         image.NewRGBA(image.Rect(0, 0, width, height)),
		attrs: RenditionAttrs{
			kRenditionAttributeType_Idiom: uint16hex(idiom),
			kRenditionAttributeType_Scale: uint16hex(scale),
		},
	}
}

package asset

import (
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/bitxeno/atvloadly/pkg/bom/pkg/helper"
)

func TestAsset(t *testing.T) {
	fileNames := []string{
		"/Volumes/UsbDrive/Downloads/Payload 2/Assets.car", "AppIcon",
		// "test_data/YouTube.car", "AppIcon",
		// "test_data/Instagram.car", "AppIcon",
		// "test_data/Twitter.car", "ProductionAppIcon",
		// "test_data/YouTubeMusic.car", "LaunchIcon",
	}
	for i := 0; i < len(fileNames)-1; i += 2 {
		decodeFile(fileNames[i], fileNames[i+1], t)
	}
}

func decodeFile(fileName, imageName string, t *testing.T) {

	f, err := os.Open(fileName)

	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	b, err := NewWithReadSeeker(f)
	if err != nil {
		t.Fatal(err)
	}

	if err := b.ImageWalker(func(name string, img image.Image) (end bool) {
		log.Printf("%v: %v", name, img.Bounds())
		return false
	}); err != nil {
		t.Fatal(err)
	}

	// img, err := b.Image(imageName)
	// if err != nil {
	// 	t.Fatal(err)
	// } else {
	// 	log.Printf("b.Image(%q): %v", imageName, img.Bounds())
	// }

	// name: 'RENDITIONS'
	ri := 0
	if err := b.Renditions(func(cb *RenditionCallback) (stop bool) {
		if cb.Err != nil {
			// log.Print(cb.Err)
			return false
		}
		if cb.Type == RenditionTypeImage {
			os.MkdirAll("output", 0755)
			fileName := fmt.Sprintf("output/%v-%v.png", ri, cb.Name)
			outf, err := os.Create(fileName)
			if err != nil {
				t.Fatal(err)
			}
			defer outf.Close()
			if err := png.Encode(outf, cb.Image); err != nil {
				log.Print(err)
			}
		}
		ri++
		return false
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAssetBom(t *testing.T) {

	const fileName = "../bom/test_data/Assets.car"
	f, err := os.Open(fileName)

	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	b, err := NewWithReadSeeker(f)
	if err != nil {
		t.Fatal(err)
	}

	// name: 'CARHEADER'
	c, err := b.CarHeader()
	if err != nil {
		t.Fatal(err)
	} else {
		tc := &CarHeader{
			Tag:                helper.NewString4("RATC"),
			CoreuiVersion:      691,
			StorageVersion:     17,
			StorageTimestamp:   0,
			RenditionCount:     7,
			MainVersionString:  helper.NewString128("@(#)PROGRAM:CoreUI  PROJECT:CoreUI-691.2\n"),
			VersionString:      helper.NewString256("Xcode 12.5 (12E262) via IBCocoaTouchImageCatalogTool"),
			UUID:               helper.NewString16(""),
			AssociatedChecksum: 0,
			SchemaVersion:      2,
			ColorSpaceID:       1,
			KeySemantics:       2,
		}
		if !reflect.DeepEqual(c, tc) {
			t.Fail()
		}
	}

	// name: 'EXTENDED_METADATA'
	if c, err := b.ExtendedMetadata(); err != nil {
		t.Fatal(err)
	} else {
		tc := &CarextendedMetadata{
			Tag:                       helper.NewString4("META"),
			ThinningArguments:         helper.NewString256(""),
			DeploymentPlatformVersion: helper.NewString256("13.1"),
			DeploymentPlatform:        helper.NewString256("ios"),
			AuthoringTool:             helper.NewString256("@(#)PROGRAM:CoreThemeDefinition  PROJECT:CoreThemeDefinition-491\n"),
		}
		if !reflect.DeepEqual(c, tc) {
			t.Fail()
		}
	}

	// name: 'KEYFORMAT'
	if c, err := b.KeyFormat(); err != nil {
		t.Fatal(err)
	} else {
		tc := &RenditionKeyFmt{
			Tag:                           helper.NewString4("tmfk"),
			Version:                       0,
			MaximumRenditionKeyTokenCount: 7,
			RenditionKeyTokens:            []RenditionAttributeType{12, 15, 16, 9, 17, 1, 2},
		}
		if !reflect.DeepEqual(c, tc) {
			t.Fail()
		}
	}

	// name: 'APPEARANCEKEYS'
	if c, err := b.AppearanceKeys(); err != nil {
		t.Fatal(err)
	} else {
		if !reflect.DeepEqual(map[string]uint16{"UIAppearanceAny": 0}, c) {
			t.Fail()
		}
	}

	// name: 'FACETKEYS'
	if c, err := b.FacetKeys(); err != nil {
		t.Fatal(err)
	} else {
		tc := map[string]RenditionAttrs{
			"AppIcon": {
				kRenditionAttributeType_Element:    0x0055,
				kRenditionAttributeType_Part:       0x00DC,
				kRenditionAttributeType_Identifier: 0x1AC1,
			},
			"test": {
				kRenditionAttributeType_Element:    0x0055,
				kRenditionAttributeType_Part:       0x00B5,
				kRenditionAttributeType_Identifier: 0x41A3,
			},
			"test2": {
				kRenditionAttributeType_Element:    0x0055,
				kRenditionAttributeType_Part:       0x00B5,
				kRenditionAttributeType_Identifier: 0x684F,
			},
			"test3": {
				kRenditionAttributeType_Element:    0x0055,
				kRenditionAttributeType_Part:       0x00B5,
				kRenditionAttributeType_Identifier: 0xF4B0,
			},
		}
		if !reflect.DeepEqual(tc, c) {
			t.Fail()
		}
	}

	// TODO:
	// name: 'BITMAPKEYS'
	// if err := b.BitmapKeys(); err != nil {
	// 	t.Fatal(err)
	// }

}

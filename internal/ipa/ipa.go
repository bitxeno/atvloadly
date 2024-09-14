package ipa

import (
	"archive/zip"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/iineva/CgbiPngFix/ipaPng"

	"github.com/iineva/bom/pkg/asset"
	"github.com/iineva/ipa-server/pkg/plist"
	"github.com/iineva/ipa-server/pkg/seekbuf"
)

var (
	ErrInfoPlistNotFound = errors.New("Info.plist not found")
)

var (
	// Payload/UnicornApp.app/AppIcon_TikTok76x76@2x~ipad.png
	// Payload/UnicornApp.app/AppIcon76x76.png
	regNewIconRegular   = regexp.MustCompile(`^Payload\/.*\.app\/AppIcon-?_?\w*(\d+(\.\d+)?)x(\d+(\.\d+)?)(@\dx)?(~ipad)?\.png$`)
	regOldIconRegular   = regexp.MustCompile(`^Payload\/.*\.app\/Icon-?_?\w*(\d+(\.\d+)?)?.png$`)
	regAssetRegular     = regexp.MustCompile(`^Payload\/.*\.app/Assets.car$`)
	regInfoPlistRegular = regexp.MustCompile(`^Payload\/.*\.app/Info.plist$`)
)

// TODO: use InfoPlistIcon to parse icon files
type InfoPlistIcon struct {
	CFBundlePrimaryIcon struct {
		CFBundleIconFiles []string `json:"CFBundleIconFiles,omitempty"`
		CFBundleIconName  string   `json:"CFBundleIconName,omitempty"`
	} `json:"CFBundlePrimaryIcon,omitempty"`
}
type InfoPlist struct {
	CFBundleDisplayName        string   `json:"CFBundleDisplayName,omitempty"`
	CFBundleExecutable         string   `json:"CFBundleExecutable,omitempty"`
	CFBundleIconName           string   `json:"CFBundleIconName,omitempty"`
	CFBundleIdentifier         string   `json:"CFBundleIdentifier,omitempty"`
	CFBundleName               string   `json:"CFBundleName,omitempty"`
	CFBundleShortVersionString string   `json:"CFBundleShortVersionString,omitempty"`
	CFBundleSupportedPlatforms []string `json:"CFBundleSupportedPlatforms,omitempty"`
	CFBundleVersion            string   `json:"CFBundleVersion,omitempty"`
	// not standard
	Channel string `json:"channel"`
}

func ParseFile(path string) (*IPA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	return Parse(f, stat.Size())
}

func Parse(readerAt io.ReaderAt, size int64) (*IPA, error) {
	r, err := zip.NewReader(readerAt, size)
	if err != nil {
		return nil, err
	}

	// match files
	var plistFile *zip.File
	var iconFiles []*zip.File
	var assetFile *zip.File
	for _, f := range r.File {
		// parse Info.plist
		if match := regInfoPlistRegular.MatchString(f.Name); match {
			plistFile = f
		}

		// parse old icons
		if match := regOldIconRegular.MatchString(f.Name); match {
			iconFiles = append(iconFiles, f)
		}

		// parse new icons
		if match := regNewIconRegular.MatchString(f.Name); match {
			iconFiles = append(iconFiles, f)
		}

		// parse Assets.car
		if match := regAssetRegular.MatchString(f.Name); match {
			assetFile = f
		}

	}

	// parse Info.plist
	if plistFile == nil {
		return nil, ErrInfoPlistNotFound
	}
	var app *IPA
	{
		pf, err := plistFile.Open()
		if err != nil {
			return nil, err
		}
		defer pf.Close()
		info := &InfoPlist{}
		err = plist.Decode(pf, info)
		if err != nil {
			return nil, err
		}
		app = &IPA{
			info: info,
			size: size,
		}
	}

	// select bigest icon file
	var iconFile *zip.File
	var maxSize = -1
	for _, f := range iconFiles {
		size, err := iconSize(f.Name)
		if err != nil {
			return nil, err
		}
		if size > maxSize {
			maxSize = size
			iconFile = f
		}
	}
	// parse icon
	img, err := parseIconImage(iconFile)
	if err == nil {
		app.icon = img
	} else if assetFile != nil {
		// try get icon from Assets.car
		if img, err := parseIconAssets(assetFile); err == nil {
			app.icon = img
		} else {
			fmt.Println(err)
		}
	}

	return app, nil
}

func iconSize(fileName string) (s int, err error) {
	size := float64(0)
	match := regOldIconRegular.MatchString(fileName)
	name := strings.TrimSuffix(filepath.Base(fileName), ".png")
	if match {
		arr := strings.Split(name, "-")
		if len(arr) == 2 {
			size, err = strconv.ParseFloat(arr[1], 32)
		} else {
			size = 160
		}
	}
	match = regNewIconRegular.MatchString(fileName)
	if match {
		s := strings.Split(name, "@")[0]
		s = strings.Split(s, "x")[1]
		s = strings.Split(s, "~")[0]
		size, err = strconv.ParseFloat(s, 32)
		if strings.Contains(name, "@2x") {
			size *= 2
		} else if strings.Contains(name, "@3x") {
			size *= 3
		}
	}
	return int(size), err
}

func parseIconImage(iconFile *zip.File) (image.Image, error) {

	if iconFile == nil {
		return nil, errors.New("icon file is nil")
	}

	f, err := iconFile.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf, err := seekbuf.Open(f, seekbuf.MemoryMode)
	if err != nil {
		return nil, err
	}
	defer buf.Close()

	img, err := png.Decode(buf)
	if err != nil {
		// try fix to std png
		cgbi, err := ipaPng.Decode(buf)
		if err != nil {
			return nil, err
		}
		img = cgbi.Img
	}

	return img, nil
}

func parseIconAssets(assetFile *zip.File) (image.Image, error) {
	f, err := assetFile.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf, err := seekbuf.Open(f, seekbuf.MemoryMode)
	if err != nil {
		return nil, err
	}
	defer buf.Close()

	a, err := asset.NewWithReadSeeker(buf)
	if err != nil {
		return nil, err
	}

	var img image.Image
	err = a.ImageWalker(func(name string, i image.Image) (end bool) {
		if strings.Contains(strings.ToLower(name), "icon") {
			img = i
			return true
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	return img, err
}


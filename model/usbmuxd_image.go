package model

import (
	"fmt"
	"strings"

	"github.com/bitxeno/atvloadly/internal/utils"
)

type UsbmuxdImage struct {
	Device UsbmuxdDevice

	ImageMounted                      bool   `json:"ImageMounted,omitempty"`
	DeveloperDiskImageUrl             string `json:"DeveloperDiskImageUrl,omitempty"`
	DeveloperDiskImageVersion         string `json:"DeveloperDiskImageVersion,omitempty"`
	DeveloperDiskImageFallbackUrl     string `json:"DeveloperDiskImageFallbackUrl,omitempty"`
	DeveloperDiskImageFallbackVersion string `json:"DeveloperDiskImageFallbackVersion,omitempty"`
}

func NewUsbmuxdImage(device UsbmuxdDevice, imageSource string) *UsbmuxdImage {
	arr := strings.Split(device.ProductVersion, ".")
	version := ""
	FallbackVersion := ""
	major := 0
	minor := 0
	if len(arr) < 2 {
		major = utils.MustParseInt(arr[0])
		minor = 0
		version = fmt.Sprintf("%d.0", major)
	} else {
		major = utils.MustParseInt(arr[0])
		minor = utils.MustParseInt(arr[1])
		version = fmt.Sprintf("%d.%d", major, minor)
	}

	// Newest system lack DeveloperDiskImage, try fallback to last minor version
	if minor > 0 {
		FallbackVersion = fmt.Sprintf("%d.%d", major, minor-1)
	}

	return &UsbmuxdImage{
		Device:                            device,
		ImageMounted:                      false,
		DeveloperDiskImageUrl:             strings.Replace(imageSource, "{0}", version, -1),
		DeveloperDiskImageVersion:         version,
		DeveloperDiskImageFallbackUrl:     strings.Replace(imageSource, "{0}", FallbackVersion, -1),
		DeveloperDiskImageFallbackVersion: FallbackVersion,
	}
}

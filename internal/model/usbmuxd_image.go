package model

import (
	"fmt"
	"strings"

	"github.com/bitxeno/atvloadly/internal/utils"
)

type UsbmuxdImage struct {
	Device UsbmuxdDevice

	VersionMajor              int    `json:"VersionMajor"`
	VersionMinor              int    `json:"VersionMinor"`
	ImageMounted              bool   `json:"ImageMounted,omitempty"`
	DeveloperDiskImageUrl     string `json:"DeveloperDiskImageUrl,omitempty"`
	DeveloperDiskImageVersion string `json:"DeveloperDiskImageVersion,omitempty"`
}

func NewUsbmuxdImage(device UsbmuxdDevice, imageSource string) *UsbmuxdImage {
	arr := strings.Split(device.ProductVersion, ".")
	version := ""
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

	return &UsbmuxdImage{
		Device:                    device,
		VersionMajor:              major,
		VersionMinor:              minor,
		ImageMounted:              false,
		DeveloperDiskImageUrl:     strings.Replace(imageSource, "{0}", version, -1),
		DeveloperDiskImageVersion: version,
	}
}

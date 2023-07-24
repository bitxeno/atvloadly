package model

import (
	"fmt"
	"strings"

	"github.com/bitxeno/atvloadly/internal/utils"
)

type UsbmuxdImage struct {
	Device UsbmuxdDevice

	ImageMounted                       bool   `json:"ImageMounted,omitempty"`
	DeveloperDiskImageUrl              string `json:"DeveloperDiskImageUrl,omitempty"`
	DeveloperDiskImageVersion          string `json:"DeveloperDiskImageVersion,omitempty"`
	DowngradeDeveloperDiskImageUrl     string `json:"DowngradeDeveloperDiskImageUrl,omitempty"`
	DowngradeDeveloperDiskImageVersion string `json:"DowngradeDeveloperDiskImageVersion,omitempty"`
}

func NewUsbmuxdImage(device UsbmuxdDevice, imageSource string) *UsbmuxdImage {
	arr := strings.Split(device.ProductVersion, ".")
	version := ""
	downgradeVersion := ""
	major := 0
	minor := 0
	if len(arr) < 2 {
		major = utils.MustParseInt(arr[0])
		minor = 0
		version = fmt.Sprintf("%s.0", arr[0])
		downgradeVersion = fmt.Sprintf("%d.%d", major-1, minor)
	} else {
		major = utils.MustParseInt(arr[0])
		minor = utils.MustParseInt(arr[1])
		version = fmt.Sprintf("%s.%s", arr[0], arr[1])
		downgradeVersion = fmt.Sprintf("%d.%d", major, minor-1)
	}

	return &UsbmuxdImage{
		Device:                             device,
		ImageMounted:                       false,
		DeveloperDiskImageUrl:              strings.Replace(imageSource, "{0}", version, -1),
		DeveloperDiskImageVersion:          version,
		DowngradeDeveloperDiskImageUrl:     strings.Replace(imageSource, "{0}", downgradeVersion, -1),
		DowngradeDeveloperDiskImageVersion: downgradeVersion,
	}
}

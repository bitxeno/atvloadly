package model

import (
	"fmt"
	"strings"
)

type UsbmuxdImage struct {
	Device UsbmuxdDevice

	ImageMounted              bool   `json:"ImageMounted,omitempty"`
	DeveloperDiskImageUrl     string `json:"DeveloperDiskImageUrl,omitempty"`
	DeveloperDiskImageVersion string `json:"DeveloperDiskImageVersion,omitempty"`
}

func NewUsbmuxdImage(device UsbmuxdDevice, imageSource string) *UsbmuxdImage {
	arr := strings.Split(device.ProductVersion, ".")
	tvOSVersion := ""
	if len(arr) < 2 {
		tvOSVersion = fmt.Sprintf("%s.0", arr[0])
	} else {
		tvOSVersion = fmt.Sprintf("%s.%s", arr[0], arr[1])
	}

	return &UsbmuxdImage{
		Device:                    device,
		ImageMounted:              false,
		DeveloperDiskImageUrl:     strings.Replace(imageSource, "{0}", tvOSVersion, -1),
		DeveloperDiskImageVersion: tvOSVersion,
	}
}

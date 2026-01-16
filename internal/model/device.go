package model

import "strings"

type Device struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	ServiceName    string       `json:"service_name"`
	IP             string       `json:"ip"`
	MacAddr        string       `json:"mac_addr"`
	UDID           string       `json:"udid"`
	Status         DeviceStatus `json:"status"`
	Enable         bool         `json:"enable"`
	Message        string       `json:"message"`
	DeviceClass    string       `json:"device_class"`
	ProductType    string       `json:"product_type"`
	ProductVersion string       `json:"product_version"`
}

func (d *Device) ParseDeviceClass() {
	if d.DeviceClass == "" {
		if strings.HasSuffix(d.Name, "AppleTV") {
			d.DeviceClass = string(DeviceClassAppleTV)
		} else if strings.HasSuffix(d.Name, "iPad") {
			d.DeviceClass = string(DeviceClassiPad)
		} else if strings.HasSuffix(d.Name, "iPhone") {
			d.DeviceClass = string(DeviceClassiPhone)
		} else {
			d.DeviceClass = string(DeviceClassAppleTV)
		}
	}
}

func (d *Device) IsIPhone() bool {
	return d.DeviceClass == string(DeviceClassiPhone) || d.DeviceClass == string(DeviceClassiPad)
}

const (
	Unpaired DeviceStatus = "unpaired"
	Paired   DeviceStatus = "paired"
	Pairable DeviceStatus = "pairable"

	DeviceClassiPhone  DeviceClass = "iPhone"
	DeviceClassiPad    DeviceClass = "iPad"
	DeviceClassAppleTV DeviceClass = "AppleTV"
)

type DeviceStatus string
type DeviceClass string

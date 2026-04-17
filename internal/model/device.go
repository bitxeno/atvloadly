package model

import "strings"

type Device struct {
	ID                       string           `json:"id"`
	Name                     string           `json:"name"`
	ServiceName              string           `json:"service_name"`
	IP                       string           `json:"ip"`
	Port                     uint16           `json:"port,omitempty"`
	MacAddr                  string           `json:"mac_addr"`
	UDID                     string           `json:"udid"`
	Connection               DeviceConnection `json:"connection"`
	Status                   DeviceStatus     `json:"status"`
	Enable                   bool             `json:"enable"`
	Message                  string           `json:"message"`
	DeviceClass              string           `json:"device_class"`
	ProductType              string           `json:"product_type"`
	ProductVersion           string           `json:"product_version"`
	DeveloperModeStatus      bool             `json:"developer_mode_status"`
	PersonalizedImageMounted bool             `json:"personalized_image_mounted"`
	PairingFile              string           `json:"pairing_file"`
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

	LockdownConnection DeviceConnection = "lockdown"
	RemoteConnection   DeviceConnection = "remote"
)

type DeviceStatus string
type DeviceClass string
type DeviceConnection string

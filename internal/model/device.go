package model

import (
	"strings"
	"time"
)

type Device struct {
	ID                       string           `json:"id"`
	Name                     string           `json:"name"`
	ServiceName              string           `json:"service_name"`
	IP                       string           `json:"ip"`
	// Port is the port tunnel-capable operations (installs, probes,
	// screenshots) connect to, i.e. the regular remote-pairing service.
	// Discovered devices learn it from their announcement; manually added
	// devices start on Apple's fixed manual-pairing port and the
	// install-port scan rewrites it once the regular service is found.
	Port uint16 `json:"port,omitempty"`
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
	DiscoveryAt              time.Time        `json:"-"`
	// Manual marks devices registered by hand (Add Device dialog or a
	// pairing-record import) that no discovery service announces; only
	// manual devices are deletable from the portal.
	Manual bool `json:"manual,omitempty"`
}

// ParseDeviceClass classifies the device: the product type is authoritative
// (iPhone12,1, iPad13,1, AppleTV11,1), the name suffix is the fallback, and
// anything unknown is treated as an Apple TV. It is safe to run over a
// previously guessed value: the manual add flow classifies a placeholder
// name, and the product type observed after pairing overrides it.
func (d *Device) ParseDeviceClass() {
	switch {
	case strings.HasPrefix(d.ProductType, string(DeviceClassiPhone)):
		d.DeviceClass = string(DeviceClassiPhone)
		return
	case strings.HasPrefix(d.ProductType, string(DeviceClassiPad)):
		d.DeviceClass = string(DeviceClassiPad)
		return
	case strings.HasPrefix(d.ProductType, string(DeviceClassAppleTV)):
		d.DeviceClass = string(DeviceClassAppleTV)
		return
	}

	if strings.HasSuffix(d.Name, string(DeviceClassAppleTV)) {
		d.DeviceClass = string(DeviceClassAppleTV)
	} else if strings.HasSuffix(d.Name, string(DeviceClassiPad)) {
		d.DeviceClass = string(DeviceClassiPad)
	} else if strings.HasSuffix(d.Name, string(DeviceClassiPhone)) {
		d.DeviceClass = string(DeviceClassiPhone)
	} else {
		d.DeviceClass = string(DeviceClassAppleTV)
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

	DeviceConnectionLockdown DeviceConnection = "Lockdown"
	DeviceConnectionRemote   DeviceConnection = "RPPairing"
)

type DeviceStatus string
type DeviceClass string
type DeviceConnection string

// ProfilePlatform returns the provisioning profile platform ("iOS", "tvOS")
// required to install apps on the device class, or "" when unknown.
func (c DeviceClass) ProfilePlatform() string {
	switch c {
	case DeviceClassAppleTV:
		return "tvOS"
	case DeviceClassiPhone, DeviceClassiPad:
		return "iOS"
	default:
		return ""
	}
}

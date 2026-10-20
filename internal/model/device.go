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
	// Port is the port the pairing flow connects to. Devices on the network
	// advertise two services: the manual-pairing service that accepts the
	// PIN flow, and the regular remote-pairing service that createListener
	// installs run over. The two live on different ports.
	Port uint16 `json:"port,omitempty"`
	// InstallPort is the port of the regular remote-pairing service
	// (_remotepairing._tcp), used by installs, probes and screenshots. A
	// device added through an imported pairing record has no pair port and
	// keeps Port set to the same value as InstallPort.
	InstallPort uint16 `json:"install_port,omitempty"`
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

// ServicePort returns the port the tunnel-capable remote-pairing service
// listens on: installs, network probes and screenshots open a tunnel through
// createListener, which only that service accepts. Manual devices pair on a
// separate manual-pairing port and carry an explicit InstallPort; when unset,
// everything shares Port.
func (d *Device) ServicePort() uint16 {
	if d.InstallPort != 0 {
		return d.InstallPort
	}
	return d.Port
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

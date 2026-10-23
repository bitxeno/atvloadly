package model

import "strings"

type RemoteDevice struct {
	ID                string `json:"id"`
	AccountID         string `json:"account_id"`
	Model             string `json:"model"`
	Name              string `json:"name"`
	RemotePairingUDID string `json:"remotepairing_udid"`
	// Manual is set when the device was paired by a manually entered IP
	// (e.g. over Tailscale). RemotePairingIP then holds that address.
	Manual          bool   `json:"manual,omitempty"`
	RemotePairingIP string `json:"remotepairing_ip,omitempty"`
}

func (d *RemoteDevice) GetDeviceClass() string {
	if d.Model != "" {
		if strings.Contains(d.Model, "AppleTV") {
			return string(DeviceClassAppleTV)
		} else if strings.Contains(d.Model, "iPad") {
			return string(DeviceClassiPad)
		} else if strings.Contains(d.Model, "iPhone") {
			return string(DeviceClassiPhone)
		} else {
			return string(DeviceClassAppleTV)
		}
	}
	return ""
}

package model

import "strings"

type RemoteDevice struct {
	ID                string `json:"id"`
	AccountID         string `json:"account_id"`
	Model             string `json:"model"`
	Name              string `json:"name"`
	RemotePairingUDID string `json:"remotepairing_udid"`
}

func (d *RemoteDevice) GetDeviceClass() string {
	if d.Model != "" {
		if strings.HasSuffix(d.Model, "AppleTV") {
			return string(DeviceClassAppleTV)
		} else if strings.HasSuffix(d.Model, "iPad") {
			return string(DeviceClassiPad)
		} else if strings.HasSuffix(d.Model, "iPhone") {
			return string(DeviceClassiPhone)
		} else {
			return string(DeviceClassAppleTV)
		}
	}
	return ""
}

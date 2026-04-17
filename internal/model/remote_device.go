package model

type RemoteDevice struct {
	Id             string `json:"id"`
	UUID           string `json:"uuid"`
	DeviceClass    string `json:"device_class"`
	UniqueDeviceID string `json:"unique_device_id"`
	PairingFile    string `json:"pairing_file"`
}

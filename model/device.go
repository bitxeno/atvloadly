package model

type Device struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	ServiceName string       `json:"service_name"`
	IP          string       `json:"ip"`
	MacAddr     string       `json:"mac_addr"`
	UDID        string       `json:"udid"`
	Status      DeviceStatus `json:"status"`
	Enable      bool         `json:"enable"`
	Message     string       `json:"message"`
}

const (
	Unpaired DeviceStatus = "unpaired"
	Paired   DeviceStatus = "paired"
	Pairable DeviceStatus = "pairable"
)

type DeviceStatus string

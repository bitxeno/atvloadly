package model

type UsbmuxdDevice struct {
	DeviceName           string `json:"DeviceName,omitempty"`
	ProductVersion       string `json:"ProductVersion,omitempty"`
	ProductType          string `json:"ProductType,omitempty"`
	ModelNumber          string `json:"ModelNumber,omitempty"`
	SerialNumber         string `json:"SerialNumber,omitempty"`
	PhoneNumber          string `json:"PhoneNumber,omitempty"`
	CPUArchitecture      string `json:"CPUArchitecture,omitempty"`
	ProductName          string `json:"ProductName,omitempty"`
	ProtocolVersion      string `json:"ProtocolVersion,omitempty"`
	RegionInfo           string `json:"RegionInfo,omitempty"`
	TimeIntervalSince197 string `json:"TimeIntervalSince197,omitempty"`
	TimeZone             string `json:"TimeZone,omitempty"`
	UniqueDeviceID       string `json:"UniqueDeviceID,omitempty"`
	WiFiAddress          string `json:"WiFiAddress,omitempty"`
	BluetoothAddress     string `json:"BluetoothAddress,omitempty"`
	BasebandVersion      string `json:"BasebandVersion,omitempty"`
	DeviceColor          string `json:"DeviceColor,omitempty"`
	DeviceClass          string `json:"DeviceClass,omitempty"`
}

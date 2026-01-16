package model

type DeviceInfo struct {
	UniqueDeviceID string `json:"unique_device_id"`
	DeviceName     string `json:"device_name"`
	DeviceClass    string `json:"device_class"`
	ProductName    string `json:"product_name"`
	ProductType    string `json:"product_type"`
	ProductVersion string `json:"product_version"`
	SerialNumber   string `json:"serial_number"`
	WiFiAddress    string `json:"wifi_address"`
}

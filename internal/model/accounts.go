package model

type AppleAccount struct {
	Email  string `json:"email,omitempty"`
	Status string `json:"status,omitempty"`
}

type AccountDevice struct {
	DeviceID       string `json:"deviceId"`
	Name           string `json:"name"`
	DeviceNumber   string `json:"deviceNumber"`
	DevicePlatform string `json:"devicePlatform"`
	Status         string `json:"status"`
	DeviceClass    string `json:"deviceClass"`
	ExpirationDate string `json:"expirationDate"`
}

type Accounts struct {
	Accounts map[string]AppleAccount `json:"accounts"`
}

package model

type Certificate struct {
	SerialNumber   string `json:"serialNumber"`
	Name           string `json:"name"`
	MachineName    string `json:"machineName"`
	Status         string `json:"status"`
	Type           string `json:"type"`
	ExpirationDate string `json:"expirationDate"`
	CreatedDate    string `json:"createdDate"`
}

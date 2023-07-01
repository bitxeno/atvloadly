package model

type ServiceStatus struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
}

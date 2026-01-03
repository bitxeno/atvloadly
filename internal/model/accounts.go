package model

type AppleAccount struct {
	Email  string `json:"email,omitempty"`
	Status string `json:"status,omitempty"`
}

type Accounts struct {
	Accounts map[string]AppleAccount `json:"accounts"`
}

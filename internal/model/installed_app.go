package model

import (
	"encoding/json"
	"time"

	masker "github.com/ggwhite/go-masker/v2"
	"gorm.io/gorm"
)

type InstalledApp struct {
	gorm.Model

	IpaName          string     `json:"ipa_name"`
	IpaPath          string     `json:"ipa_path"`
	Description      string     `json:"description,omitempty"`
	Device           string     `json:"device"`
	UDID             string     `gorm:"column:udid" json:"udid"`
	Account          string     `json:"account"`
	Password         string     `json:"password"`
	InstalledDate    *time.Time `json:"installed_date"`
	RefreshedDate    *time.Time `json:"refreshed_date"`
	ExpirationDate   *time.Time `json:"expiration_date"`
	RefreshedResult  bool       `json:"refreshed_result"`
	Icon             string     `json:"icon"`
	BundleIdentifier string     `json:"bundle_identifier"`
	Version          string     `json:"version"`
	Enabled          bool       `json:"enabled,omitempty"`
}

// 输出json时，清空密码字段，提高安全性
func (t InstalledApp) MarshalJSON() ([]byte, error) {
	type Alias InstalledApp
	return json.Marshal(&struct {
		*Alias
		Password string `json:"password"`
	}{
		Alias:    (*Alias)(&t),
		Password: "",
	})
}

func (t InstalledApp) MaskAccount() string {
	m := masker.EmailMasker{}
	return m.Marshal("*", t.Account)
}

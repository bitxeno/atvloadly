package model

import (
	"encoding/json"
	"time"

	masker "github.com/ggwhite/go-masker/v2"
	"gorm.io/gorm"
)

type InstalledApp struct {
	gorm.Model

	IpaName          string         `json:"ipa_name"`
	IpaPath          string         `json:"ipa_path"`
	Description      string         `json:"description,omitempty"`
	Device           string         `json:"device"`
	DeviceClass      string         `json:"device_class"`
	UDID             string         `gorm:"column:udid" json:"udid"`
	Account          string         `json:"account"`
	Password         string         `json:"password"`
	InstalledDate    *time.Time     `json:"installed_date"`
	RefreshedDate    *time.Time     `json:"refreshed_date"`
	ExpirationDate   *time.Time     `json:"expiration_date"`
	RefreshedResult  bool           `json:"refreshed_result"`
	RefreshedError   RefreshedError `json:"refreshed_error"`
	Icon             string         `json:"icon"`
	BundleIdentifier string         `json:"bundle_identifier"`
	Version          string         `json:"version"`
	RemoveExtensions bool           `json:"remove_extensions"`
	Enabled          bool           `json:"enabled,omitempty"`
}

type RefreshedError int

const (
	RefreshedErrorNone           RefreshedError = 0
	RefreshedErrorInvalidAccount RefreshedError = 1
	RefreshedErrorInvalidOther   RefreshedError = 99
)

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

func (t InstalledApp) IsIPhoneApp() bool {
	return t.DeviceClass == string(DeviceClassiPhone) || t.DeviceClass == string(DeviceClassiPad)
}

func (t InstalledApp) NeedRefresh(advanceDays int) bool {
	now := time.Now()

	// fix RefreshedDate is nil
	if t.RefreshedDate == nil {
		return true
	}

	// fix ExpirationDate is nil
	expirationDate := t.ExpirationDate
	if expirationDate == nil {
		expireTime := t.RefreshedDate.AddDate(0, 0, 7)
		expirationDate = &expireTime
	}

	// Use configured advance days (default to 1 if not set or invalid)
	if advanceDays <= 0 {
		advanceDays = 1
	}

	return expirationDate.AddDate(0, 0, -advanceDays).Before(now)
}

func (t InstalledApp) IsAccountInvalid() bool {
	return t.RefreshedError == RefreshedErrorInvalidAccount
}

// IsExpired checks if the app has strictly expired (ExpirationDate < now)
func (t InstalledApp) IsExpired() bool {
	now := time.Now()

	// If ExpirationDate is nil, the app is considered expired (never refreshed or unknown expiration)
	if t.ExpirationDate == nil {
		return true
	}

	// Strict check: expired if ExpirationDate is before now
	return t.ExpirationDate.Before(now)
}

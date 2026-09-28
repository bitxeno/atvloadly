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
	CustomName       string         `json:"custom_name,omitempty"`
	Enabled          bool           `json:"enabled,omitempty"`
	Source           AppSource      `gorm:"embedded;embeddedPrefix:source_" json:"source"`

	// SigningMode is empty for records created before signing modes existed;
	// use EffectiveSigningMode to read it.
	SigningMode SigningMode `gorm:"size:32;index" json:"signing_mode"`
	// SigningIdentityID references the SigningIdentity used in SigningModeExternalCertificate.
	SigningIdentityID uint `gorm:"index" json:"signing_identity_id,omitempty"`
	// SignedBundleIdentifier is the main bundle identifier actually installed
	// on the device in SigningModeExternalCertificate: CustomIdentifier when
	// set, else the bundle identifier of the IPA.
	SignedBundleIdentifier string `json:"signed_bundle_identifier,omitempty"`
	// CustomIdentifier is the main bundle identifier requested by the user in
	// SigningModeExternalCertificate (passed to the engine as
	// --custom-identifier); empty keeps the bundle identifiers of the IPA.
	// Reinstallations reuse it.
	CustomIdentifier string `gorm:"column:custom_identifier" json:"custom_identifier,omitempty"`
	// AllowMissingEntitlements records the explicit user choice to sign even when
	// the provisioning profile does not grant every entitlement requested by the app.
	AllowMissingEntitlements bool `json:"allow_missing_entitlements"`
}

type RefreshedError int

const (
	RefreshedErrorNone           RefreshedError = 0
	RefreshedErrorInvalidAccount RefreshedError = 1
	// RefreshedErrorSigningIdentity: external certificate, key or profile is invalid or incompatible.
	RefreshedErrorSigningIdentity RefreshedError = 2
	// RefreshedErrorSigning: signing failed or the signed output did not pass verification.
	RefreshedErrorSigning RefreshedError = 3
	// RefreshedErrorTransport: the device could not be reached or the installation failed on it.
	RefreshedErrorTransport    RefreshedError = 4
	RefreshedErrorInvalidOther RefreshedError = 99
)

// EffectiveSigningMode returns the signing mode, treating historical records as Apple ID.
func (t InstalledApp) EffectiveSigningMode() SigningMode {
	return t.SigningMode.OrDefault()
}

// IsExternalSigning reports whether the app is signed with an imported signing identity.
func (t InstalledApp) IsExternalSigning() bool {
	return t.EffectiveSigningMode() == SigningModeExternalCertificate
}

// 输出json时，清空密码字段，提高安全性
// The effective signing mode is emitted so historical records read as apple_id.
func (t InstalledApp) MarshalJSON() ([]byte, error) {
	type Alias InstalledApp
	return json.Marshal(&struct {
		*Alias
		Password    string      `json:"password"`
		SigningMode SigningMode `json:"signing_mode"`
	}{
		Alias:       (*Alias)(&t),
		Password:    "",
		SigningMode: t.EffectiveSigningMode(),
	})
}

// DisplayName returns the custom name of the app, or its IPA name.
func (t InstalledApp) DisplayName() string {
	if t.CustomName != "" {
		return t.CustomName
	}
	return t.IpaName
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

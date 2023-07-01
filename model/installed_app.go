package model

import (
	"time"

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
	Password         string     `json:"-"`
	InstalledDate    *time.Time `json:"installed_date"`
	RefreshedDate    *time.Time `json:"refreshed_date"`
	RefreshedResult  bool       `json:"refreshed_result"`
	Icon             string     `json:"icon"`
	BundleIdentifier string     `json:"bundle_identifier"`
	Version          string     `json:"version"`
	Enabled          bool       `json:"enabled,omitempty"`
}

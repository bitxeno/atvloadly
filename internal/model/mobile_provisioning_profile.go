package model

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/smallstep/pkcs7"
	plist "howett.net/plist"
)

// ProfileKind classifies a provisioning profile by how it provisions devices.
type ProfileKind string

const (
	ProfileKindDevelopment ProfileKind = "development"
	ProfileKindAdHoc       ProfileKind = "ad_hoc"
	ProfileKindEnterprise  ProfileKind = "enterprise"
	ProfileKindAppStore    ProfileKind = "app_store"
)

// MobileProvisioningProfile is the plist payload of a .mobileprovision file.
// Parsing only reads the CMS content; it does not verify the CMS signature.
type MobileProvisioningProfile struct {
	AppIDName                   string         `plist:"AppIDName"`
	ApplicationIdentifierPrefix []string       `plist:"ApplicationIdentifierPrefix"`
	CreationDate                time.Time      `plist:"CreationDate"`
	ExpirationDate              time.Time      `plist:"ExpirationDate"`
	Name                        string         `plist:"Name"`
	Platform                    []string       `plist:"Platform"`
	IsXcodeManaged              bool           `plist:"IsXcodeManaged"`
	DeveloperCertificates       [][]byte       `plist:"DeveloperCertificates"`
	Entitlements                map[string]any `plist:"Entitlements"`
	ProvisionedDevices          []string       `plist:"ProvisionedDevices"`
	ProvisionsAllDevices        bool           `plist:"ProvisionsAllDevices"`
	TeamIdentifier              []string       `plist:"TeamIdentifier"`
	TeamName                    string         `plist:"TeamName"`
	UUID                        string         `plist:"UUID"`
	Version                     int            `plist:"Version"`
}

func ParseMobileProvisioningProfileFile(path string) (*MobileProvisioningProfile, error) {
	if f, err := os.Stat(path); err != nil || f.Size() == 0 {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseMobileProvisioningProfile(data)
}

// ParseMobileProvisioningProfile decodes the plist carried by a CMS-wrapped
// provisioning profile. The CMS signature is NOT verified here.
func ParseMobileProvisioningProfile(data []byte) (*MobileProvisioningProfile, error) {
	p7, err := pkcs7.Parse(data)
	if err != nil {
		return nil, err
	}

	var profile MobileProvisioningProfile
	if _, err := plist.Unmarshal(p7.Content, &profile); err != nil {
		return nil, err
	}
	if profile.UUID == "" || profile.ExpirationDate.IsZero() {
		return nil, errors.New("provisioning profile is missing UUID or ExpirationDate")
	}

	return &profile, nil
}

// ApplicationIdentifier returns the application-identifier entitlement,
// e.g. "TEAMID.com.example.app" or "TEAMID.*".
func (p *MobileProvisioningProfile) ApplicationIdentifier() string {
	value, _ := p.Entitlements["application-identifier"].(string)
	return value
}

// TeamID returns the first team identifier of the profile.
func (p *MobileProvisioningProfile) TeamID() string {
	if len(p.TeamIdentifier) == 0 {
		return ""
	}
	return p.TeamIdentifier[0]
}

// AppIDPrefix returns the App ID prefix that starts the application-identifier
// entitlement, or "" when no declared prefix matches it.
func (p *MobileProvisioningProfile) AppIDPrefix() string {
	appID := p.ApplicationIdentifier()
	candidates := append(append([]string{}, p.ApplicationIdentifierPrefix...), p.TeamIdentifier...)
	for _, prefix := range candidates {
		if prefix != "" && strings.HasPrefix(appID, prefix+".") {
			return prefix
		}
	}
	return ""
}

// BundleIDPattern returns the application-identifier without its App ID prefix:
// an explicit bundle identifier ("com.example.app") or a wildcard pattern
// ("com.example.*", "*"). It returns "" when the prefix is unknown.
func (p *MobileProvisioningProfile) BundleIDPattern() string {
	prefix := p.AppIDPrefix()
	if prefix == "" {
		return ""
	}
	return strings.TrimPrefix(p.ApplicationIdentifier(), prefix+".")
}

// GetTaskAllow returns the get-task-allow entitlement granted by the profile.
func (p *MobileProvisioningProfile) GetTaskAllow() bool {
	value, _ := p.Entitlements["get-task-allow"].(bool)
	return value
}

// Kind classifies the profile from its device provisioning and debug entitlement.
func (p *MobileProvisioningProfile) Kind() ProfileKind {
	switch {
	case p.ProvisionsAllDevices:
		return ProfileKindEnterprise
	case len(p.ProvisionedDevices) == 0:
		return ProfileKindAppStore
	case p.GetTaskAllow():
		return ProfileKindDevelopment
	default:
		return ProfileKindAdHoc
	}
}

// SupportsPlatform reports whether the profile lists the platform ("iOS", "tvOS", ...).
func (p *MobileProvisioningProfile) SupportsPlatform(platform string) bool {
	for _, value := range p.Platform {
		if strings.EqualFold(value, platform) {
			return true
		}
	}
	return false
}

// ProvisionsDevice reports whether the profile allows installation on the device UDID.
func (p *MobileProvisioningProfile) ProvisionsDevice(udid string) bool {
	if p.ProvisionsAllDevices {
		return true
	}
	for _, value := range p.ProvisionedDevices {
		if strings.EqualFold(value, udid) {
			return true
		}
	}
	return false
}

// IncludesCertificate reports whether the DER certificate is listed in DeveloperCertificates.
func (p *MobileProvisioningProfile) IncludesCertificate(der []byte) bool {
	for _, cert := range p.DeveloperCertificates {
		if bytes.Equal(cert, der) {
			return true
		}
	}
	return false
}

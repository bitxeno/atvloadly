package model

import "time"

// SigningMode selects how an app is signed before installation.
type SigningMode string

const (
	// SigningModeAppleID signs through PlumeImpactor with an Apple ID session.
	SigningModeAppleID SigningMode = "apple_id"
	// SigningModeExternalCertificate signs with an imported P12 identity and its
	// provisioning profile, without any Apple ID session.
	SigningModeExternalCertificate SigningMode = "external_certificate"
)

// OrDefault maps the empty mode of historical records and legacy clients to SigningModeAppleID.
func (m SigningMode) OrDefault() SigningMode {
	if m == "" {
		return SigningModeAppleID
	}
	return m
}

// IsValid reports whether m is a known signing mode. The empty mode is not valid.
func (m SigningMode) IsValid() bool {
	return m == SigningModeAppleID || m == SigningModeExternalCertificate
}

// SigningIdentity is an imported certificate and private key together with the
// provisioning profile used to sign apps in SigningModeExternalCertificate.
//
// The private key is persisted only in sealed (authenticated encryption) form.
// Raw material is excluded from JSON. The model has no soft-delete column so a
// delete really removes the sealed key from the database.
type SigningIdentity struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Name string `json:"name"`

	CertificateSHA1         string    `gorm:"size:40;index" json:"certificate_sha1"`
	CertificateSHA256       string    `gorm:"size:64;uniqueIndex:idx_signing_identity_cert_profile" json:"certificate_sha256"`
	CertificateCommonName   string    `json:"certificate_common_name"`
	CertificateSerialNumber string    `json:"certificate_serial_number"`
	CertificateNotBefore    time.Time `json:"certificate_not_before"`
	CertificateNotAfter     time.Time `json:"certificate_not_after"`
	TeamID                  string    `gorm:"size:16;index" json:"team_id"`
	TeamName                string    `json:"team_name"`

	ProfileUUID                  string      `gorm:"size:64;uniqueIndex:idx_signing_identity_cert_profile" json:"profile_uuid"`
	ProfileName                  string      `json:"profile_name"`
	ProfileAppIDName             string      `json:"profile_app_id_name"`
	ProfileApplicationIdentifier string      `json:"profile_application_identifier"`
	ProfileKind                  ProfileKind `gorm:"size:32" json:"profile_kind"`
	ProfilePlatforms             []string    `gorm:"serializer:json" json:"profile_platforms"`
	ProfileDeviceCount           int         `json:"profile_device_count"`
	ProfileProvisionsAllDevices  bool        `json:"profile_provisions_all_devices"`
	ProfileCreationDate          time.Time   `json:"profile_creation_date"`
	ProfileExpirationDate        time.Time   `json:"profile_expiration_date"`

	// Revision starts at 1 and increases each time the profile is replaced.
	Revision int `json:"revision"`

	CertificateDER   []byte `json:"-"`
	SealedPrivateKey []byte `json:"-"`
	ProfileData      []byte `json:"-"`
}

// ExpiresAt returns the effective signing deadline: the earlier of the
// certificate and provisioning profile expirations.
func (s SigningIdentity) ExpiresAt() time.Time {
	if s.ProfileExpirationDate.Before(s.CertificateNotAfter) {
		return s.ProfileExpirationDate
	}
	return s.CertificateNotAfter
}

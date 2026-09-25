package signing

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bitxeno/atvloadly/internal/model"
)

// maxIdentityNameRunes bounds the display name of an identity.
const maxIdentityNameRunes = 100

// ImportRequest carries the raw inputs of an identity import. P12 and Password
// are never persisted.
type ImportRequest struct {
	Name     string
	P12      []byte
	Password string
	Profile  []byte
}

// NewIdentity decodes and validates an import and returns the identity to
// persist (ID 0, Revision 1) with its private key sealed by sealer, plus the
// non-blocking issues. Blocking findings are returned as *Error.
func NewIdentity(req ImportRequest, sealer *Sealer, now time.Time) (*model.SigningIdentity, []Issue, error) {
	roots, err := appleRootPool()
	if err != nil {
		return nil, nil, Wrap(ClassIdentity, CodeProfileSignatureInvalid, err, "the Apple root certificates are unavailable")
	}
	return newIdentity(req, sealer, now, roots)
}

func newIdentity(req ImportRequest, sealer *Sealer, now time.Time, roots *x509.CertPool) (*model.SigningIdentity, []Issue, error) {
	if sealer == nil {
		return nil, nil, Errorf(ClassIdentity, CodeKeyUnavailable, "the signing key is not available")
	}
	p12, err := DecodeP12(req.P12, req.Password)
	if err != nil {
		return nil, nil, err
	}
	profile, err := verifyProfile(req.Profile, roots)
	if err != nil {
		return nil, nil, err
	}
	issues := ValidateIdentity(p12.Certificate, profile, now)
	if err := identityIssuesError(issues); err != nil {
		return nil, nil, err
	}

	cert := p12.Certificate
	sha256Sum := sha256.Sum256(cert.Raw)
	sha1Sum := sha1.Sum(cert.Raw)
	certificateSHA256 := hex.EncodeToString(sha256Sum[:])

	keyDER, err := x509.MarshalPKCS8PrivateKey(p12.PrivateKey)
	if err != nil {
		return nil, nil, Wrap(ClassIdentity, CodeP12Unsupported, err, "the private key cannot be encoded")
	}
	sealed, err := sealer.Seal(keyDER, IdentityAAD(certificateSHA256))
	clear(keyDER)
	if err != nil {
		return nil, nil, err
	}

	name := sanitizeIdentityName(req.Name)
	if name == "" {
		name = sanitizeIdentityName(cert.Subject.CommonName)
	}
	teamName := profile.TeamName
	if teamName == "" && len(cert.Subject.Organization) > 0 {
		teamName = cert.Subject.Organization[0]
	}
	identity := &model.SigningIdentity{
		Name:                    name,
		CertificateSHA1:         hex.EncodeToString(sha1Sum[:]),
		CertificateSHA256:       certificateSHA256,
		CertificateCommonName:   cert.Subject.CommonName,
		CertificateSerialNumber: strings.ToUpper(cert.SerialNumber.Text(16)),
		CertificateNotBefore:    cert.NotBefore,
		CertificateNotAfter:     cert.NotAfter,
		TeamID:                  certificateTeamID(cert),
		TeamName:                teamName,
		Revision:                1,
		CertificateDER:          cert.Raw,
		SealedPrivateKey:        sealed,
	}
	setProfile(identity, profile, req.Profile)
	return identity, issues, nil
}

// ReplaceProfile validates profileData against the identity certificate and
// returns a copy with every profile field replaced and Revision incremented.
func ReplaceProfile(identity model.SigningIdentity, profileData []byte, now time.Time) (*model.SigningIdentity, []Issue, error) {
	roots, err := appleRootPool()
	if err != nil {
		return nil, nil, Wrap(ClassIdentity, CodeProfileSignatureInvalid, err, "the Apple root certificates are unavailable")
	}
	return replaceProfile(identity, profileData, now, roots)
}

func replaceProfile(identity model.SigningIdentity, profileData []byte, now time.Time, roots *x509.CertPool) (*model.SigningIdentity, []Issue, error) {
	profile, err := verifyProfile(profileData, roots)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(identity.CertificateDER)
	if err != nil {
		return nil, nil, Wrap(ClassIdentity, CodeCertificateInvalid, err, "the stored certificate of the identity cannot be parsed")
	}
	issues := ValidateIdentity(cert, profile, now)
	if err := identityIssuesError(issues); err != nil {
		return nil, nil, err
	}
	updated := identity
	setProfile(&updated, profile, profileData)
	updated.Revision = identity.Revision + 1
	return &updated, issues, nil
}

// IdentityStatus returns the display findings of a stored identity at time now.
func IdentityStatus(identity model.SigningIdentity, now time.Time) []Issue {
	var issues []Issue
	cert, err := x509.ParseCertificate(identity.CertificateDER)
	if err != nil {
		issues = append(issues, Issue{
			Code:     CodeCertificateInvalid,
			Severity: SeverityError,
			Message:  "the stored certificate of the identity cannot be parsed: " + err.Error(),
		})
	}
	profile, err := model.ParseMobileProvisioningProfile(identity.ProfileData)
	if err != nil {
		issues = append(issues, Issue{
			Code:     CodeProfileInvalid,
			Severity: SeverityError,
			Message:  "the stored provisioning profile cannot be parsed: " + err.Error(),
		})
	}
	if issues != nil {
		return issues
	}
	return ValidateIdentity(cert, profile, now)
}

// setProfile copies the metadata and raw bytes of a verified profile into identity.
func setProfile(identity *model.SigningIdentity, profile *model.MobileProvisioningProfile, data []byte) {
	identity.ProfileUUID = profile.UUID
	identity.ProfileName = profile.Name
	identity.ProfileAppIDName = profile.AppIDName
	identity.ProfileApplicationIdentifier = profile.ApplicationIdentifier()
	identity.ProfileKind = profile.Kind()
	identity.ProfilePlatforms = profile.Platform
	identity.ProfileDeviceCount = len(profile.ProvisionedDevices)
	identity.ProfileProvisionsAllDevices = profile.ProvisionsAllDevices
	identity.ProfileCreationDate = profile.CreationDate
	identity.ProfileExpirationDate = profile.ExpirationDate
	identity.ProfileData = data
}

// sanitizeIdentityName removes control characters and surrounding spaces and
// bounds the length of a display name.
func sanitizeIdentityName(name string) string {
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name))
	if utf8.RuneCountInString(name) > maxIdentityNameRunes {
		name = strings.TrimSpace(string([]rune(name)[:maxIdentityNameRunes]))
	}
	return name
}

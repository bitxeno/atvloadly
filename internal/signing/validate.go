package signing

import (
	"crypto/x509"
	"fmt"
	"slices"
	"time"

	"github.com/bitxeno/atvloadly/internal/model"
)

// expiringSoonWindow is how long before the effective expiry an identity is
// reported as expiring soon.
const expiringSoonWindow = 7 * 24 * time.Hour

// ValidateIdentity returns the findings about a certificate and a provisioning
// profile at time now: certificate usage, validity dates, team, profile expiry,
// installability and whether the profile lists the certificate. Compatibility
// with a device or an IPA is not checked here.
func ValidateIdentity(cert *x509.Certificate, profile *model.MobileProvisioningProfile, now time.Time) []Issue {
	var issues []Issue
	fail := func(code string, format string, args ...any) {
		issues = append(issues, Issue{Code: code, Severity: SeverityError, Message: fmt.Sprintf(format, args...)})
	}

	if !slices.Contains(cert.ExtKeyUsage, x509.ExtKeyUsageCodeSigning) {
		fail(CodeCertificateNotCodeSigning, "the certificate is not valid for code signing")
	}
	if now.Before(cert.NotBefore) {
		fail(CodeCertificateNotYetValid, "the certificate is not valid before %s", formatTime(cert.NotBefore))
	}
	if now.After(cert.NotAfter) {
		fail(CodeCertificateExpired, "the certificate expired on %s", formatTime(cert.NotAfter))
	}
	team := certificateTeamID(cert)
	if team == "" {
		fail(CodeCertificateNoTeam, "the certificate subject has no team identifier (OU)")
	} else if profile.TeamID() != team {
		fail(CodeProfileTeamMismatch, "the provisioning profile belongs to team %q but the certificate to team %q", profile.TeamID(), team)
	}
	if now.After(profile.ExpirationDate) {
		fail(CodeProfileExpired, "the provisioning profile expired on %s", formatTime(profile.ExpirationDate))
	}
	if profile.Kind() == model.ProfileKindAppStore {
		fail(CodeProfileNotInstallable, "App Store provisioning profiles cannot be used to install apps on devices")
	}
	if !profile.IncludesCertificate(cert.Raw) {
		fail(CodeProfileCertificateMissing, "the provisioning profile does not include the certificate")
	}
	if profile.AppIDPrefix() == "" {
		fail(CodeProfileInvalid, "the provisioning profile application-identifier %q has no known App ID prefix", profile.ApplicationIdentifier())
	}

	expiry := cert.NotAfter
	if profile.ExpirationDate.Before(expiry) {
		expiry = profile.ExpirationDate
	}
	if !now.After(expiry) && expiry.Sub(now) <= expiringSoonWindow {
		issues = append(issues, Issue{
			Code:     CodeIdentityExpiringSoon,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("the signing identity expires on %s", formatTime(expiry)),
		})
	}
	issues = append(issues, Issue{
		Code:     CodeRevocationNotChecked,
		Severity: SeverityInfo,
		Message:  "Certificate revocation status was not verified",
	})
	return issues
}

// identityIssuesError summarizes blocking identity findings, or returns nil.
func identityIssuesError(issues []Issue) error {
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			return IssuesError(ClassIdentity, issue.Code, "the signing identity is not valid", issues)
		}
	}
	return nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// certificateTeamID returns the team identifier of an Apple code signing
// certificate: the first organizational unit of its subject.
func certificateTeamID(cert *x509.Certificate) string {
	if len(cert.Subject.OrganizationalUnit) == 0 {
		return ""
	}
	return cert.Subject.OrganizationalUnit[0]
}

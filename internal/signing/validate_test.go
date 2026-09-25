package signing

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"slices"
	"testing"
	"time"

	"github.com/bitxeno/atvloadly/internal/model"
)

func TestValidateIdentity(t *testing.T) {
	ca := newTestCA(t, "Synthetic WWDR")
	key := testECKey(t)
	leaf := newTestLeaf(t, key, ca).cert

	withTemplate := func(edit func(*x509.Certificate)) *x509.Certificate {
		tmpl := leafTemplate()
		edit(tmpl)
		return issue(t, tmpl, key, ca).cert
	}
	notCodeSigning := withTemplate(func(c *x509.Certificate) { c.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth} })
	noTeam := withTemplate(func(c *x509.Certificate) { c.Subject = pkix.Name{CommonName: "No Team"} })
	otherTeam := withTemplate(func(c *x509.Certificate) { c.Subject.OrganizationalUnit = []string{"ZZZZZ99999"} })

	profileFor := func(cert *x509.Certificate, edit func(*model.MobileProvisioningProfile)) *model.MobileProvisioningProfile {
		profile := newTestProfile(cert)
		if edit != nil {
			edit(profile)
		}
		return profile
	}
	day := 24 * time.Hour

	tests := []struct {
		name    string
		cert    *x509.Certificate
		profile *model.MobileProvisioningProfile
		now     time.Time
		want    []string
	}{
		{"valid", leaf, profileFor(leaf, nil), testNow, nil},
		{"not a code signing certificate", notCodeSigning, profileFor(notCodeSigning, nil), testNow, []string{CodeCertificateNotCodeSigning}},
		{"certificate not yet valid", leaf, profileFor(leaf, nil), testLeafNotBefore.Add(-time.Second), []string{CodeCertificateNotYetValid}},
		{"certificate valid from its first second", leaf, profileFor(leaf, nil), testLeafNotBefore, nil},
		{"certificate expired", leaf, profileFor(leaf, func(p *model.MobileProvisioningProfile) { p.ExpirationDate = testLeafNotAfter.Add(30 * day) }),
			testLeafNotAfter.Add(time.Second), []string{CodeCertificateExpired}},
		{"certificate without team", noTeam, profileFor(noTeam, nil), testNow, []string{CodeCertificateNoTeam}},
		{"profile of another team", otherTeam, profileFor(otherTeam, nil), testNow, []string{CodeProfileTeamMismatch}},
		{"profile expired", leaf, profileFor(leaf, func(p *model.MobileProvisioningProfile) { p.ExpirationDate = testNow.Add(-time.Second) }),
			testNow, []string{CodeProfileExpired}},
		{"App Store profile", leaf, profileFor(leaf, func(p *model.MobileProvisioningProfile) { p.ProvisionedDevices = nil }),
			testNow, []string{CodeProfileNotInstallable}},
		{"enterprise profile", leaf, profileFor(leaf, func(p *model.MobileProvisioningProfile) {
			p.ProvisionedDevices = nil
			p.ProvisionsAllDevices = true
		}), testNow, nil},
		{"profile without the certificate", leaf, profileFor(otherTeam, nil), testNow, []string{CodeProfileCertificateMissing}},
		{"profile application-identifier without known prefix", leaf, profileFor(leaf, func(p *model.MobileProvisioningProfile) {
			p.Entitlements["application-identifier"] = "QQQQQ11111.com.example.app"
		}), testNow, []string{CodeProfileInvalid}},
		{"certificate expires within 7 days", leaf, profileFor(leaf, nil), testLeafNotAfter.Add(-7 * day), []string{CodeIdentityExpiringSoon}},
		{"certificate expires in more than 7 days", leaf, profileFor(leaf, nil), testLeafNotAfter.Add(-7*day - time.Second), nil},
		{"profile expires within 7 days", leaf, profileFor(leaf, func(p *model.MobileProvisioningProfile) { p.ExpirationDate = testNow.Add(6 * day) }),
			testNow, []string{CodeIdentityExpiringSoon}},
		{"expiring at this very second", leaf, profileFor(leaf, func(p *model.MobileProvisioningProfile) { p.ExpirationDate = testNow }),
			testNow, []string{CodeIdentityExpiringSoon}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := ValidateIdentity(tt.cert, tt.profile, tt.now)
			var got []string
			revocationNoted := false
			for _, issue := range issues {
				if issue.Code == CodeRevocationNotChecked {
					revocationNoted = issue.Severity == SeverityInfo
					continue
				}
				wantSeverity := SeverityError
				if issue.Code == CodeIdentityExpiringSoon {
					wantSeverity = SeverityWarning
				}
				if issue.Severity != wantSeverity {
					t.Errorf("issue %s has severity %s, want %s", issue.Code, issue.Severity, wantSeverity)
				}
				got = append(got, issue.Code)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("codes = %v, want %v (issues %+v)", got, tt.want, issues)
			}
			if !revocationNoted {
				t.Fatal("missing informational revocation issue")
			}
		})
	}
}

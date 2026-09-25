package signing

import (
	"crypto/x509"

	"github.com/smallstep/pkcs7"

	"github.com/bitxeno/atvloadly/internal/model"
)

// profileSignerCommonName is the subject common name of the certificate
// Apple uses to sign provisioning profiles.
const profileSignerCommonName = "Apple iPhone OS Provisioning Profile Signing"

// VerifyProfile parses a .mobileprovision, verifies its CMS signature and that
// the signer is Apple's provisioning profile signing certificate chaining to an
// embedded Apple root CA at the profile creation date. Revocation is not
// checked. Errors are *Error of ClassIdentity with CodeProfileInvalid or
// CodeProfileSignatureInvalid.
func VerifyProfile(data []byte) (*model.MobileProvisioningProfile, error) {
	roots, err := appleRootPool()
	if err != nil {
		return nil, Wrap(ClassIdentity, CodeProfileSignatureInvalid, err, "the Apple root certificates are unavailable")
	}
	return verifyProfile(data, roots)
}

// verifyProfile is VerifyProfile with an explicit trust anchor pool.
func verifyProfile(data []byte, roots *x509.CertPool) (*model.MobileProvisioningProfile, error) {
	if len(data) > MaxProfileSize {
		return nil, Errorf(ClassIdentity, CodeUploadTooLarge, "the provisioning profile exceeds %d bytes", MaxProfileSize)
	}
	p7, err := pkcs7.Parse(data)
	if err != nil {
		return nil, Wrap(ClassIdentity, CodeProfileInvalid, err, "the provisioning profile is not a valid CMS signed message")
	}
	if err := p7.Verify(); err != nil {
		return nil, Wrap(ClassIdentity, CodeProfileSignatureInvalid, err, "the provisioning profile signature is invalid")
	}
	signer := p7.GetOnlySigner()
	if signer == nil {
		return nil, Errorf(ClassIdentity, CodeProfileSignatureInvalid, "the provisioning profile must have exactly one signer")
	}
	if signer.Subject.CommonName != profileSignerCommonName {
		return nil, Errorf(ClassIdentity, CodeProfileSignatureInvalid,
			"the provisioning profile is signed by %q instead of %q", signer.Subject.CommonName, profileSignerCommonName)
	}
	profile, err := model.ParseMobileProvisioningProfile(data)
	if err != nil {
		return nil, Wrap(ClassIdentity, CodeProfileInvalid, err, "the provisioning profile content is invalid")
	}
	if profile.CreationDate.IsZero() {
		return nil, Errorf(ClassIdentity, CodeProfileInvalid, "the provisioning profile has no CreationDate")
	}
	intermediates := x509.NewCertPool()
	for _, cert := range p7.Certificates {
		intermediates.AddCert(cert)
	}
	_, err = signer.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		CurrentTime:   profile.CreationDate,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	if err != nil {
		return nil, Wrap(ClassIdentity, CodeProfileSignatureInvalid, err, "the provisioning profile signer does not chain to an Apple root certificate")
	}
	return profile, nil
}

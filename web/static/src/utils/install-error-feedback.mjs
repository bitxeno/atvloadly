export function installFailureMessage(output, translate) {
  const log = String(output ?? "");
  if (
    log.includes("An Application Group with Identifier") &&
    log.includes("is not available")
  ) {
    return translate("install.toast.app_group_unavailable");
  }
  return translate("install.toast.install_failed");
}

// The signing engine marks the failure returned when the Apple ID account is
// at its certificate limit and revoking a certificate was not authorized.
// Matches the display of the engine's Error::CertificateResetRequired.
const certificateResetRequiredMarker = "[certificate_reset_required]";

export function needsCertificateReset(output) {
  return String(output ?? "").includes(certificateResetRequiredMarker);
}

// pickRevocableCertificates filters the certificates of an account the user
// may authorize to revoke. An expired or already revoked certificate is not
// worth offering: revoking it frees no slot. The serials Apple reports for
// those states are not known here, so only the states the certificate listing
// labels are excluded; an unknown label keeps the certificate on offer.
export function pickRevocableCertificates(certificates) {
  return (certificates ?? [])
    .filter((cert) => cert && cert.serialNumber)
    .filter((cert) => {
      const status = String(cert.status ?? "").trim().toLowerCase();
      return status !== "expired" && status !== "revoked";
    });
}

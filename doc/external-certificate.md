# External certificate signing (P12 + provisioning profile)

atvloadly can sign and install apps with a signing identity you already own
instead of an Apple ID. You import a `.p12` file (certificate and private key)
and a `.mobileprovision` provisioning profile once, then select that identity on
the install page. atvloadly never asks for, stores or uses an Apple ID in this
mode: there is no Apple login, no two-factor prompt, no device registration on
the Apple Developer portal and no profile renewal through an Apple session.

The Apple ID mode keeps working exactly as before. Existing apps stay in Apple
ID mode.

What this mode is **not**:

- It is not a guaranteed offline installation. The device still has to be
  paired and reachable (USB or network) through the existing transports.
- It is not a protection against revocation. If Apple revokes the certificate
  or the profile, the installed apps stop launching. atvloadly does not check
  revocation (see [What is validated](#what-is-validated)).
- It is not a refresh mechanism. Apps signed with an imported identity are
  never refreshed automatically and a reinstall does not extend their validity
  (see [Expiry and reinstall](#expiry-and-reinstall)).

## Requirements

- A code signing certificate with its private key exported as a `.p12` file
  (for example from Keychain Access), and its password. An empty password is
  accepted when the file was exported without one.
- A provisioning profile issued for that certificate:
  - For an **Apple TV**, the profile platform must be **tvOS**. An iOS profile
    is refused for an Apple TV, and a tvOS profile is refused for an iPhone.
  - For an **iPhone / iPad**, the profile platform must be **iOS**.
  - **Ad hoc** and **development** profiles must list the hardware UDID of the
    target device (register the device in your Apple Developer account and
    regenerate the profile after adding it).
  - **Enterprise (in-house)** profiles provision all devices.
  - **App Store** distribution profiles cannot install on devices and are
    refused at import.
- An IPA built for the platform of the target device (a tvOS IPA for an Apple
  TV).

Upload limits: 1 MiB for the `.p12`, 2 MiB for the profile.

## Importing an identity

1. Open the **Accounts** page of the web UI. Signing identities have their own
   section, separate from the Apple ID accounts and their certificates.
2. Choose **Import**, give an optional name (the certificate common name is used
   when left empty), select the `.p12` file, type its password (leave it empty
   if it has none) and select the provisioning profile.
3. The identity is listed with its certificate common name, team, SHA-1
   fingerprint, certificate and profile dates, profile kind, platforms and
   device count, the effective expiry and its status findings.

The P12 is decoded inside the atvloadly process: the password is never written
to disk, passed to another program, logged or returned by the API. The P12 file
itself is not stored. Only the leaf certificate, the profile and the private key
sealed with the [deployment key](#deployment-key) are stored in the database.

### Replacing the profile

**Replace profile** validates a new profile against the same certificate and
swaps it atomically (the identity revision increases by one). Use it when the
profile expires or when you add a device to the profile. It is refused while an
installation using the identity is running.

### Deleting an identity

Deleting removes the identity and its sealed key from the database. It is
refused while an installation using the identity is running. When installed
apps were signed with the identity, the deletion needs an explicit confirmation;
those apps keep their records, but reinstalling them fails with "signing
identity not found" until you reinstall them with another identity.

## What is validated

At import (and again, at the current time, before every installation):

| Check | Result when it fails |
|---|---|
| P12 integrity (MAC) and decryption with the given password. Modern (PBES2/AES) and legacy (RC2/3DES) files are supported; the integrity check is never skipped. | Refused: invalid file, wrong password or unsupported encryption |
| Exactly one private key and exactly one certificate matching it (CA certificates are ignored) | Refused: missing key, missing certificate, key/certificate mismatch or ambiguous identity |
| Certificate usable for code signing, inside its validity dates, carrying a team identifier | Refused |
| Profile CMS signature verified cryptographically; the signer must be Apple's "Apple iPhone OS Provisioning Profile Signing" certificate chaining to the Apple Root CA or Apple Root CA - G3 embedded in atvloadly | Refused |
| Profile not expired, not an App Store profile, listing the imported certificate, from the certificate team | Refused |
| Effective expiry within 7 days | Warning |
| Certificate revocation (OCSP/CRL) | **Not checked**: the status always shows "revocation not verified" |

Reading the profile content and verifying its signature are two different
steps: atvloadly does both. It never modifies the signed content of a profile.

### Bundle identifiers and application identifiers

Like Feather and eSign, atvloadly keeps the bundle identifiers of the IPA: the
main app and its extensions are installed under their own identifiers
(`CFBundleIdentifier` is not changed) and the extensions are kept. The single
provisioning profile of the identity is embedded in every app and app extension
and each of them is signed with the entitlements of the profile, merged with
the entitlements of its executable. Every `*` of the profile entitlements is
replaced by the bundle identifier of the bundle being signed (see the wildcard
exception below), so the application identifier each bundle is signed with
depends on the profile App ID:

| Profile App ID | Application identifier of each bundle | Example for `com.vendor.app` and its widget `com.vendor.app.widget` |
|---|---|---|
| Explicit, `TEAMID.app.example.myapp` | The profile App ID, for every bundle | `TEAMID.app.example.myapp` for both |
| Full wildcard, `TEAMID.*` | `TEAMID.<bundle identifier>` (the natural one) | `TEAMID.com.vendor.app`, `TEAMID.com.vendor.app.widget` |
| Prefix wildcard, `TEAMID.com.example.*` | `TEAMID.com.example.<bundle identifier>` | `TEAMID.com.example.com.vendor.app`, `TEAMID.com.example.com.vendor.app.widget` |

The install page shows the application identifier the main app will be signed
with. Every bundle whose application identifier is not
`TEAMID.<its bundle identifier>` gets a warning
(`application_identifier_from_profile`); the installation is not refused for
it, but the app may not behave as when signed by its developer:

- App Groups, keychain sharing, push notifications, iCloud and Sign in with
  Apple are tied to the application identifier and the developer's App ID
  configuration, so they may not work.
- Every app signed with the same explicit App ID profile gets the same
  application identifier.

**Custom bundle identifier** (optional): when you type one on the install page
(or pass `custom_identifier` to the API), the main bundle identifier of the IPA
is replaced by it wherever it occurs in the bundle identifiers of the main app
and of every app extension, and in the Apple Watch companion identifiers
(`WKCompanionAppBundleIdentifier`, `WKAppBundleIdentifier`). Extensions follow
the main app: with the custom identifier `org.me.app`, the widget
`com.vendor.app.widget` becomes `org.me.app.widget` (an identifier that does
not contain the main one, such as `org.other.intents`, is unchanged). The plan
reports the rewrite (`bundle_identifier_rewritten`). Leave the field empty, or
type the identifier of the IPA, to keep the identifiers. A custom identifier is
made of non-empty parts of letters, digits and hyphens separated by dots, at
most 255 characters; anything else is refused (`custom_identifier_invalid`).
With an explicit App ID profile the application identifier stays the profile
App ID; with a wildcard profile it follows the new bundle identifiers. The
custom identifier is stored with the app and reused by reinstalls.

**Remove extensions** (optional): removes every app extension before signing.
Extensions are never removed unless you enable it; the plan lists the removed
extensions.

### Install-time compatibility checks

Before signing, atvloadly builds a plan of what the signing engine will do and
refuses the installation when something is incompatible. The install page shows
the plan (the same checks are available through the check API). The
installation is refused when:

- **Identity**: the certificate is not listed in the profile, or the
  certificate, the key or the profile is invalid or expired.
- **Platform**: the profile does not support the device platform (tvOS for an
  Apple TV, iOS for an iPhone/iPad) or the IPA targets another platform.
- **Device**: the profile does not list the device hardware UDID (for devices
  reached over the network, atvloadly reads the UDID from the device), unless
  it provisions all devices.
- **Profile App ID**: the application identifier of the profile does not start
  with its App ID prefix (`application_identifier_mismatch`).
- **Wildcard profiles**: the signing engine only replaces the wildcard of the
  profile entitlements when the executable carries its own entitlements. A
  bundle whose executable carries none would be signed with the literal
  wildcard application identifier, so it is refused
  (`wildcard_requires_binary_entitlements`); use an explicit App ID profile, or
  enable **Remove extensions** when only extensions are concerned.
- **Nested apps**: an app nested inside another app (for example a Watch app)
  cannot be signed with the profile (`nested_bundles_not_signed`).
- **Entitlements**: entitlements requested by the app but not granted by the
  profile, or granted with other values, block the installation unless you
  explicitly allow missing entitlements. The app may then lose features or
  crash at launch.

A profile App ID that differs from the bundle identifiers is not a reason to
refuse: it only produces the `application_identifier_from_profile` warning
described above.

### Signing and verification

The signing engine (PlumeImpactor `plumesign`) runs with the certificate, the
unencrypted private key and the profile written into a private per-task
directory (`<work_dir>/signing-work`, directory mode 0700, files 0600, isolated
`HOME` and `TMPDIR`). That directory is removed after success, failure or
cancellation; directories left by a crash are removed at the next start. The
engine runs without `--apple-id`, `-u` or `--refresh`.

After the engine run, atvloadly audits the signed IPA written by the engine:
the set of bundles, and for every kept app and extension the planned bundle
identifier, the embedded profile (identical to the imported one), the signing
certificate (identical to the imported one), and exactly the planned
application identifier and entitlements. The engine installs before it writes
its output, so this audit runs after the installation: an audit failure fails
the task and must be reported, but the app may already be on the device.

Failures are classified:

- **identity** (certificate, key, profile, compatibility) and **signing**
  (unreadable IPA, engine failure, audit failure) errors are never retried and
  never restart `usbmuxd`;
- **transport** errors keep the existing behavior (one retry after restarting
  `usbmuxd`), except a missing or unreadable pairing record of a network
  device (`pairing_record_invalid`), which is never retried: pair the device
  again.

The install log contains `SIGNING_REPORT: {...}` lines with the plan, the
failure class and code, and `{"stage":"verified"}` once the audit passed.

## Expiry and reinstall

The expiry of an identity, and of every app installed with it, is the earlier
of the certificate expiry and the profile expiry.

Apps installed with an imported identity are **not refreshed automatically**.
Reinstalling one re-signs it with the same certificate and profile: it does
**not** extend its validity. To extend it, replace the profile with a newer one
(or import a new certificate and profile) and then reinstall the app.

External certificate installations read an IPA file uploaded on the install
page: the IPA URL and source modes are Apple ID only. For the same reason,
these apps cannot track a source (GitHub releases or AltStore): their updates
would otherwise be signed and installed unattended with the certificate.
Install a new build manually instead.

## Deployment key

The private keys of the imported identities are stored encrypted (AES-256-GCM)
with a deployment key kept in a file outside the database.

- **Location**: `signing.key_file` in `config.yaml`; default
  `<work_dir>/keys/signing-identity.key` (`/data/keys/signing-identity.key` in
  Docker).

  ```yaml
  signing:
    key_file: /data/keys/signing-identity.key
  ```

- **Creation**: the file (32 random bytes) is created on first use with
  directory mode 0700 and file mode 0600. An existing file is never replaced.
  A file readable by group or others is accepted with a warning in the log
  (Docker secrets are mounted 0444).
- **Backup**: back up the key file **together with** the database (`app.db`).
  The database alone cannot unseal the private keys.
- **Restore**: restore both files, keep the same `signing.key_file` setting,
  then start atvloadly.
- **Loss**: the sealed private keys cannot be recovered. Installations fail with
  a key error; delete the affected identities and import every P12 again.
- **Rotation**: there is no in-place rotation. Stop atvloadly, move the old key
  file away, start atvloadly, delete every identity and import every P12 again
  (a new key is created by the first import). Apps signed with a deleted
  identity must then be reinstalled with the new one.
- **Separation**: by default the key file lives in the same `/data` volume as
  the database, so a copy of the whole volume contains both. For a real
  separation, keep the key in another volume or a Docker secret and point
  `signing.key_file` to it. A pre-provisioned file must contain exactly 32
  random bytes:

  ```sh
  (umask 077 && head -c 32 /dev/urandom > signing-identity.key)
  ```

  ```yaml
  # docker-compose.yml
  services:
    atvloadly:
      volumes:
        - /etc/atvloadly:/data
        - /etc/atvloadly-keys:/keys:ro
  # config.yaml
  signing:
    key_file: /keys/signing-identity.key
  ```

## Access control

atvloadly has no authentication. Anyone who can reach its web UI or API can
import and delete identities and install apps signed with them (the API never
returns private keys, passwords or profiles). Only expose atvloadly on a trusted
network, or put it behind a reverse proxy that enforces authentication.

## REST API

All endpoints answer HTTP 200 with `{code, msg, data}`. Failures have
`code: -1`, an English `msg` and `data: {code, class, issues, app_count}` where
`data.code` is a stable code (see `internal/signing/errors.go`).

| Endpoint | Input | Result |
|---|---|---|
| `GET /api/signing/identities` | | identities with `expires_at`, `status`, `app_count`, `in_use` |
| `POST /api/signing/identities/import` | multipart `name`, `password`, `p12`, `profile` | identity |
| `POST /api/signing/identities/:id/profile` | multipart `profile` | identity |
| `POST /api/signing/identities/:id/delete` | JSON `{"force": false}` | `true` |
| `POST /api/signing/identities/:id/check` | JSON `{"ipa_path", "udid", "remove_extensions", "allow_missing_entitlements", "custom_identifier"}` | `{plan, blocking}` |
| `POST /api/install` | existing form + `signing_mode=external_certificate`, `signing_identity_id`, `allow_missing_entitlements`, `custom_identifier` | as before |

`custom_identifier` is optional (empty keeps the bundle identifiers of the IPA)
and only accepted with `signing_mode=external_certificate`. The same fields are
accepted in the install message of the `/ws/install` WebSocket (`signing_mode`,
`signing_identity_id`, `remove_extensions`, `allow_missing_entitlements`,
`custom_identifier`) and, except `allow_missing_entitlements`, by the MCP
`install_app` tool (`signing_identity_id`, `remove_extensions`,
`custom_identifier`).

In the plan, `bundles[].expected_application_identifier` is the application
identifier the engine will sign each bundle with, `signed_main_bundle_id` the
main bundle identifier after the optional custom identifier, and
`custom_identifier` is only present when the bundle identifiers are rewritten.

## Hardware test procedure

Automated tests use synthetic certificates and profiles only. Signing with a
real certificate does not prove that the app installs and launches on tvOS:
report the steps below separately.

1. Start atvloadly **without any Apple ID account** configured.
2. Import the P12 (try the right password, a wrong one and, if you have such a
   file, an empty one) and a **tvOS** profile listing the Apple TV UDID. The
   status must show no error, only "revocation not verified" and possibly an
   expiry warning.
3. Pair the Apple TV as usual (USB or network).
4. On the install page, choose **External certificate**, select the identity,
   the Apple TV and a tvOS IPA. Review the plan: bundle identifier (unchanged
   unless you type a custom bundle identifier), signed application identifier,
   `application_identifier_from_profile` warnings, extensions, entitlements,
   blocking issues.
5. Install. The log must show the plan report, the engine output
   (`Signing bundle`, `Installing to device`, `Installation complete!`),
   `SIGNING_REPORT: {"stage":"verified"}` and `Installation Succeeded!`.
6. Launch the app on the Apple TV.
7. Restart the container and check that the identity is still listed and that
   a reinstall still works (deployment key and database persisted).
8. Optional: install again with a custom bundle identifier and check on the
   device that the app and its extensions carry the new identifiers.
9. Optional negative checks: an iOS profile with the Apple TV
   (`profile_platform_mismatch`), a profile that does not list the device
   (`device_not_provisioned`), an invalid custom bundle identifier such as
   `my_app` (`custom_identifier_invalid`), deleting the identity while an
   installation runs (`identity_in_use`).

What to report: the atvloadly image tag or commit, the Apple TV model and tvOS
version, the transport (USB or network), the profile kind and platforms, the
plan shown before installing, the install log (`/apps/<id>/log`) with its
`SIGNING_REPORT` lines and whether the app launches. Never send the P12, its
password, the private key, the deployment key or the provisioning profile; mask
device UDIDs if you share logs publicly.

The same procedure applies to an iPhone or iPad with an **iOS** profile.

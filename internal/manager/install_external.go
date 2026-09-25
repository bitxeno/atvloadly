package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/signing"
	"github.com/bitxeno/atvloadly/internal/utils"
	plist "howett.net/plist"
)

// ExternalSigning is the materialized signing material of an installation in
// SigningModeExternalCertificate. Every path lives in the private workspace of
// the installation, except PairingFile.
type ExternalSigning struct {
	CertificatePath string
	PrivateKeyPath  string
	ProfilePath     string
	// OutputPath receives the signed IPA; the engine writes it after installing.
	OutputPath string
	// CustomIdentifier is passed as --custom-identifier when not empty.
	CustomIdentifier string
	// PairingFile is the remote pairing record of an RSD device. Empty selects
	// the device record of the atvloadly data directory.
	PairingFile string
	// HomeDir and TempDir isolate the engine from the Apple ID accounts and
	// from the staging directories of other installations.
	HomeDir string
	TempDir string
}

// Log markers of the PlumeImpactor sign and sign-rsd commands delimiting the
// stages of an installation.
const (
	engineMarkerSigning    = "Signing bundle:"
	engineMarkerInstalling = "Installing to device:"
	engineMarkerInstalled  = "Installation complete!"
)

// maxEngineErrorSummary bounds the engine error text kept in error messages.
const maxEngineErrorSummary = 1024

// signingErrorPrefixes are the displayed error prefixes of the PlumeImpactor
// errors raised while it loads the IPA, the PEM files and the provisioning
// profile or prepares the bundles, before any device operation of the lockdown
// flow and after the device connection of the RSD flow.
var signingErrorPrefixes = []string{
	"zip error",
	"i/o error",
	"info.plist not found",
	"unsupported file type",
	"missing certificate pem data",
	"certificate error",
	"certificate pem error",
	"x509 certificate error",
	"rsa error",
	"pkcs1 rsa error",
	"pkcs8 rsa error",
	"entitlements not found",
	"executable not found",
	"plist error",
	"codesign error",
	"codesignbuilder error",
	"bundle failed to rename",
	"failed to parse",
	"core error",
	"-o/--output is required",
}

func isRemoteInstall(opts InstallOptions) bool {
	return opts.IP != "" && opts.Port != 0 && opts.UDID != ""
}

func (t *InstallManager) startExternal(ctx context.Context, opts InstallOptions) error {
	mark := t.outputStdout.Mark()
	ctx = t.withRunTimeout(ctx)

	if opts.External == nil {
		return signing.Errorf(signing.ClassSigning, signing.CodeWorkspaceFailed, "external signing material is missing")
	}
	ext := *opts.External
	if ext.CertificatePath == "" || ext.PrivateKeyPath == "" || ext.ProfilePath == "" || ext.OutputPath == "" || ext.HomeDir == "" || ext.TempDir == "" {
		return signing.Errorf(signing.ClassSigning, signing.CodeWorkspaceFailed, "external signing material is incomplete")
	}
	if isRemoteInstall(opts) {
		// The engine HOME is isolated, so it cannot find the pairing record by itself.
		if ext.PairingFile == "" {
			ext.PairingFile = filepath.Join(app.RemotePairingDir(), opts.UDID+".plist")
		}
		if err := checkPairingRecord(ext.PairingFile); err != nil {
			return signing.Wrap(signing.ClassTransport, signing.CodePairingRecordInvalid, err, "the remote pairing record of device %s is missing or unreadable, pair the device again", opts.UDID)
		}
	}
	opts.External = &ext

	if err := CheckAfcServiceStatus(opts.UDID); err != nil {
		return signing.Wrap(signing.ClassTransport, signing.CodeDeviceUnreachable, err, "afc service not available")
	}

	args := buildExternalInstallArgs(opts)
	if err := t.runEngine(ctx, args, ext.TempDir, externalRunEnv(GetRunEnvs(), ext), nil); err != nil {
		return classifyExternalFailure(t.outputStdout.Since(mark), err)
	}
	if output := t.outputStdout.Since(mark); !strings.Contains(output, engineMarkerInstalled) {
		return classifyExternalFailure(output, errors.New("the signing engine exited without completing the installation"))
	}
	return nil
}

// checkPairingRecord fails when path is not a readable, non-empty plist
// dictionary, the form the engine requires for a remote pairing record. An
// empty file parses as an empty text plist dictionary, hence the size check.
func checkPairingRecord(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var record map[string]any
	if _, err := plist.Unmarshal(data, &record); err != nil {
		return err
	}
	if len(record) == 0 {
		return errors.New("the pairing record is empty")
	}
	return nil
}

// buildExternalInstallArgs returns the engine arguments of an installation in
// SigningModeExternalCertificate. It never uses an Apple ID session, never
// refreshes and always writes the signed IPA to the output path, which the
// engine requires for an .ipa signed without --apple-id.
func buildExternalInstallArgs(opts InstallOptions) []string {
	ext := opts.External
	var args []string
	if isRemoteInstall(opts) {
		args = []string{"sign-rsd", "--package", opts.IpaPath, "--pem", ext.CertificatePath, ext.PrivateKeyPath, "--provision", ext.ProfilePath, "--register-and-install", "--ip", opts.IP, "--port", fmt.Sprintf("%d", opts.Port), "--pairing-file", ext.PairingFile, "--udid", opts.UDID, "--output", ext.OutputPath}
	} else {
		args = []string{"sign", "--package", opts.IpaPath, "--pem", ext.CertificatePath, ext.PrivateKeyPath, "--provision", ext.ProfilePath, "--register-and-install", "--udid", opts.UDID, "--output", ext.OutputPath}
	}
	if ext.CustomIdentifier != "" {
		args = append(args, "--custom-identifier", ext.CustomIdentifier)
	}
	if opts.RemoveExtensions {
		args = append(args, "--remove-extensions")
	}
	if opts.CustomName != "" {
		args = append(args, "--custom-name", opts.CustomName)
	}
	return args
}

// externalRunEnv returns base with HOME and TMPDIR moved into the workspace:
// the engine then sees neither the Apple ID accounts nor the files of other
// installations. The proxy settings of base are kept.
func externalRunEnv(base []string, ext ExternalSigning) []string {
	return utils.MergeEnvs(base, []string{"HOME=" + ext.HomeDir, "TMPDIR=" + ext.TempDir})
}

// classifyExternalFailure maps a failed external certificate engine run to a
// *signing.Error from the last stage its output reached:
//   - installation completed: the signed package could not be saved (signing);
//   - installation started: the device failed (transport);
//   - signing started: the engine failed to sign (signing);
//   - before signing: the pairing record of an RSD device is unreadable
//     (transport, pairing record invalid), signing when the engine failed to
//     load the IPA, the PEM files or the provisioning profile, transport
//     otherwise (device selection, connection or timeout).
func classifyExternalFailure(output string, runErr error) *signing.Error {
	summary := engineErrorSummary(output, runErr)
	switch {
	case strings.Contains(output, engineMarkerInstalled):
		return signing.Wrap(signing.ClassSigning, signing.CodeSignedOutputMissing, runErr, "the app was installed but the signing engine failed to save the signed package: %s", summary)
	case strings.Contains(output, engineMarkerInstalling):
		return signing.Wrap(signing.ClassTransport, signing.CodeTransportFailed, runErr, "the installation on the device failed: %s", summary)
	case strings.Contains(output, engineMarkerSigning):
		return signing.Wrap(signing.ClassSigning, signing.CodeEngineFailed, runErr, "the signing engine failed to sign the app: %s", summary)
	case isPairingRecordError(summary):
		return signing.Wrap(signing.ClassTransport, signing.CodePairingRecordInvalid, runErr, "the signing engine could not read the pairing record of the device, pair the device again: %s", summary)
	case isSigningLoadError(summary):
		return signing.Wrap(signing.ClassSigning, signing.CodeEngineFailed, runErr, "the signing engine could not load the app or the signing material: %s", summary)
	default:
		return signing.Wrap(signing.ClassTransport, signing.CodeTransportFailed, runErr, "the device could not be reached: %s", summary)
	}
}

// isPairingRecordError reports the engine error raised when it cannot
// deserialize the pairing file of an RSD device ("io on plist").
func isPairingRecordError(summary string) bool {
	return strings.Contains(strings.ToLower(summary), "io on plist")
}

func isSigningLoadError(summary string) bool {
	s := strings.ToLower(summary)
	if strings.Contains(s, "can't install to target device type") {
		return true
	}
	for _, prefix := range signingErrorPrefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// engineErrorSummary returns the last error reported by the engine ("Error:"
// line and its "Caused by" chain) on one line, or runErr when there is none.
func engineErrorSummary(output string, runErr error) string {
	lines := strings.Split(output, "\n")
	start := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "Error:") {
			start = i
			break
		}
	}
	if start < 0 {
		if runErr == nil {
			return ""
		}
		return runErr.Error()
	}

	lines[start] = strings.TrimPrefix(lines[start], "Error:")
	summary := strings.Join(strings.Fields(strings.Join(lines[start:], " ")), " ")
	if len(summary) > maxEngineErrorSummary {
		summary = summary[:maxEngineErrorSummary] + "..."
	}
	return summary
}

// shouldRetryInstall decides whether a failed installation takes the single
// usbmuxd-restart retry. Apple ID installations retry every failure other than
// an invalid account (handled before). External certificate installations
// only retry transport failures of a request still active: certificate,
// profile and signing failures would fail again identically, and so would an
// invalid pairing record, which only a new pairing fixes.
func shouldRetryInstall(mode model.SigningMode, err error, ctxErr error) bool {
	if mode.OrDefault() != model.SigningModeExternalCertificate {
		return true
	}
	return ctxErr == nil && signing.ClassOf(err) == signing.ClassTransport && signing.CodeOf(err) != signing.CodePairingRecordInvalid
}

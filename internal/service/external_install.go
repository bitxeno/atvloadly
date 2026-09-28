package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	conf "github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/signing"
	"github.com/bitxeno/atvloadly/internal/signing/appcheck"
)

// SigningReportPrefix starts the structured lines written into the install
// output of an external certificate installation. Each line carries one JSON
// report whose "stage" is "plan", "failure" or "verified".
const SigningReportPrefix = "SIGNING_REPORT: "

// signedOutputName is the file name of the signed IPA inside the workspace.
const signedOutputName = "signed.ipa"

// ExternalCheckRequest is the input of the compatibility check run before an
// installation with an external signing identity.
type ExternalCheckRequest struct {
	IdentityID uint
	// IpaPath is client provided; it must resolve inside the upload or
	// installed-app directories (*signing.Error CodeIPAPathRejected otherwise).
	IpaPath string
	// UDID identifies the target device as listed by the device manager.
	UDID                     string
	RemoveExtensions         bool
	AllowMissingEntitlements bool
	// CustomIdentifier is the main bundle identifier requested by the user,
	// surrounding spaces ignored; an invalid value is reported by the plan.
	CustomIdentifier string
}

// planOptions are the user choices of an external certificate installation
// that shape its plan.
type planOptions struct {
	RemoveExtensions         bool
	AllowMissingEntitlements bool
	CustomIdentifier         string
}

// deviceInfoLookup queries a device for its lockdown values.
type deviceInfoLookup func(dev *model.Device) (*model.DeviceInfo, error)

// CheckExternalInstall builds the compatibility plan of an installation with
// an external signing identity without signing anything. The identity status
// issues are merged into the plan issues. Errors are *signing.Error (identity
// not found, IPA path rejected, device not found or unreachable, IPA invalid).
func CheckExternalInstall(req ExternalCheckRequest) (*appcheck.Plan, error) {
	identity, err := loadSigningIdentity(req.IdentityID)
	if err != nil {
		return nil, err
	}
	ipaPath, err := ResolveClientIPAPath(req.IpaPath)
	if err != nil {
		return nil, err
	}
	dev, found := manager.GetDeviceByUDID(req.UDID)
	if !found || dev == nil {
		return nil, signing.Errorf(signing.ClassTransport, signing.CodeDeviceUnreachable, "device not found for UDID: %s", req.UDID)
	}
	return planExternalInstall(*identity, ipaPath, dev, planOptions{
		RemoveExtensions:         req.RemoveExtensions,
		AllowMissingEntitlements: req.AllowMissingEntitlements,
		CustomIdentifier:         strings.TrimSpace(req.CustomIdentifier),
	}, manager.GetDeviceInfo)
}

// ResolveClientIPAPath resolves a client provided IPA path, following
// symbolic links, and accepts it only when it designates a regular file
// inside the upload directory (<DataDir>/tmp) or the installed apps directory
// (<DataDir>/ipa). Other paths are a *signing.Error with CodeIPAPathRejected.
func ResolveClientIPAPath(path string) (string, error) {
	dataDir := conf.Config.Server.DataDir
	resolved, err := resolvePathWithin(path, filepath.Join(dataDir, "tmp"), filepath.Join(dataDir, "ipa"))
	if err != nil {
		return "", signing.Wrap(signing.ClassSigning, signing.CodeIPAPathRejected, err, "the IPA path is not an uploaded or installed IPA")
	}
	return resolved, nil
}

// resolvePathWithin returns the symlink-free absolute path of the regular file
// path when it lies strictly inside one of roots (themselves resolved).
func resolvePathWithin(path string, roots ...string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("not a regular file")
	}

	for _, root := range roots {
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rootResolved, err := filepath.EvalSymlinks(rootAbs)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(rootResolved, resolved)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}
		return resolved, nil
	}
	return "", errors.New("outside the allowed directories")
}

// CleanExternalUpload removes the uploaded IPA and icon consumed by an
// external certificate installation when they still are regular files inside
// the upload directory (<DataDir>/tmp). Anything else is left alone, notably
// the IPA and icon of an installed app, which SaveApp moved into its record
// directory. Unlike InstallManager.CleanTempFiles, it never removes files of
// other installations.
func CleanExternalUpload(ipaPath, iconPath string) {
	uploadDir := filepath.Join(conf.Config.Server.DataDir, "tmp")
	for _, path := range []string{ipaPath, iconPath} {
		resolved, err := resolvePathWithin(path, uploadDir)
		if err != nil {
			continue
		}
		if err := os.Remove(resolved); err != nil {
			log.Err(err).Msgf("Cannot remove the uploaded file %s", resolved)
		}
	}
}

// ExternalInstall is an installation with an external signing identity,
// prepared and checked but not yet signed. It holds a lease on the identity,
// the identity snapshot used for the whole installation and the private
// workspace with the materialized signing files. Close releases everything.
type ExternalInstall struct {
	// Identity is the snapshot of the signing identity used by this installation.
	Identity model.SigningIdentity
	Plan     *appcheck.Plan
	// IpaPath is the resolved path of the IPA to sign.
	IpaPath string

	workspace *signing.Workspace
	material  *signing.Material
	release   func()
	closeOnce sync.Once
}

// ExternalInstallResult is what an installed app record keeps from a
// successful external certificate installation.
type ExternalInstallResult struct {
	// ExpiresAt is the signing deadline of the identity snapshot used.
	ExpiresAt time.Time
	// SignedBundleIdentifier is the main bundle identifier installed on the device.
	SignedBundleIdentifier string
}

// PrepareExternalInstall leases the signing identity of app, checks the IPA
// against the identity and the device, and materializes the signing files. The
// plan is reported through emit as a SIGNING_REPORT line. Every blocking
// finding is refused here, before anything is signed. Errors are
// *signing.Error; the caller must Close the returned installation.
func PrepareExternalInstall(app model.InstalledApp, dev *model.Device, emit func(string)) (*ExternalInstall, error) {
	return prepareExternalInstall(app, dev, emit, manager.GetDeviceInfo)
}

func prepareExternalInstall(app model.InstalledApp, dev *model.Device, emit func(string), lookup deviceInfoLookup) (_ *ExternalInstall, err error) {
	if app.SigningIdentityID == 0 {
		return nil, signing.Errorf(signing.ClassIdentity, signing.CodeIdentityNotFound, "no signing identity selected")
	}
	if dev == nil {
		return nil, signing.Errorf(signing.ClassTransport, signing.CodeDeviceUnreachable, "device not found for UDID: %s", app.UDID)
	}

	ext := &ExternalInstall{release: signing.AcquireLease(app.SigningIdentityID)}
	defer func() {
		if err != nil {
			ext.Close()
		}
	}()

	identity, err := loadSigningIdentity(app.SigningIdentityID)
	if err != nil {
		return nil, err
	}
	ext.Identity = *identity

	if ext.IpaPath, err = ResolveClientIPAPath(app.IpaPath); err != nil {
		return nil, err
	}

	if ext.Plan, err = planExternalInstall(ext.Identity, ext.IpaPath, dev, planOptionsOf(app), lookup); err != nil {
		return nil, err
	}
	emit(planReportLine(ext.Plan))
	if err := ext.Plan.Err(); err != nil {
		return nil, err
	}

	ws, err := signing.NewWorkspace()
	if err != nil {
		return nil, asSigningError(err, signing.ClassSigning, signing.CodeWorkspaceFailed, "cannot create the signing workspace")
	}
	ext.workspace = ws

	sealer, err := signing.DefaultSealer()
	if err != nil {
		return nil, asSigningError(err, signing.ClassIdentity, signing.CodeKeyUnavailable, "cannot load the signing key file")
	}
	if ext.material, err = signing.Materialize(ws, ext.Identity, sealer); err != nil {
		return nil, asSigningError(err, signing.ClassSigning, signing.CodeWorkspaceFailed, "cannot write the signing material")
	}
	return ext, nil
}

// Signing returns the engine signing material of the installation.
func (e *ExternalInstall) Signing() *manager.ExternalSigning {
	return &manager.ExternalSigning{
		CertificatePath:  e.material.CertificatePath,
		PrivateKeyPath:   e.material.PrivateKeyPath,
		ProfilePath:      e.material.ProfilePath,
		OutputPath:       e.outputPath(),
		CustomIdentifier: e.Plan.CustomIdentifier,
		HomeDir:          e.workspace.HomeDir,
		TempDir:          e.workspace.TempDir,
	}
}

func (e *ExternalInstall) outputPath() string {
	return filepath.Join(e.workspace.Dir, signedOutputName)
}

// Verify audits the signed IPA written by the engine against the plan and the
// identity snapshot. The engine writes it after installing, so a failure
// means an app that does not match the plan may already be on the device.
func (e *ExternalInstall) Verify() error {
	if err := appcheck.VerifySignedIPA(e.outputPath(), e.Plan, e.Identity.CertificateDER, e.Identity.ProfileData); err != nil {
		return asSigningError(err, signing.ClassSigning, signing.CodeSignedOutputMissing, "the signed IPA could not be verified")
	}
	return nil
}

// Close removes the workspace and releases the identity lease. It is safe to
// call more than once.
func (e *ExternalInstall) Close() {
	e.closeOnce.Do(func() {
		if e.workspace != nil {
			if err := e.workspace.Close(); err != nil {
				log.Err(err).Msg("Failed to remove the signing workspace")
			}
		}
		if e.release != nil {
			e.release()
		}
	})
}

// RunExternalInstall signs and installs app on dev with its external signing
// identity: prepare, engine run (TryStart with the transport retry when retry
// is set, Start otherwise), verification of the signed IPA and cleanup. The
// install output of installMgr is reset first; SIGNING_REPORT lines are
// written into it next to the engine output. Errors are *signing.Error.
func RunExternalInstall(ctx context.Context, installMgr *manager.InstallManager, app model.InstalledApp, dev *model.Device, retry bool) (*ExternalInstallResult, error) {
	installMgr.ResetLog()
	result, err := runExternalInstall(ctx, installMgr, app, dev, retry)
	if err != nil {
		installMgr.WriteLog(failureReportLine(err))
		return nil, err
	}
	installMgr.WriteLog(signingReportLine(signingReport{Stage: "verified"}))
	return result, nil
}

func runExternalInstall(ctx context.Context, installMgr *manager.InstallManager, app model.InstalledApp, dev *model.Device, retry bool) (*ExternalInstallResult, error) {
	ext, err := PrepareExternalInstall(app, dev, installMgr.WriteLog)
	if err != nil {
		return nil, err
	}
	defer ext.Close()

	opts := manager.InstallOptions{
		UDID:             app.UDID,
		IP:               dev.IP,
		Port:             dev.Port,
		IpaPath:          ext.IpaPath,
		CustomName:       app.CustomName,
		RemoveExtensions: app.RemoveExtensions,
		SigningMode:      model.SigningModeExternalCertificate,
		External:         ext.Signing(),
	}
	start := installMgr.Start
	if retry {
		start = installMgr.TryStart
	}
	if err := start(ctx, opts); err != nil {
		return nil, asSigningError(err, signing.ClassTransport, signing.CodeTransportFailed, "the installation failed")
	}
	if err := ext.Verify(); err != nil {
		return nil, err
	}

	return &ExternalInstallResult{
		ExpiresAt:              ext.Identity.ExpiresAt(),
		SignedBundleIdentifier: ext.Plan.SignedMainBundleID,
	}, nil
}

// RefreshedErrorOf maps an installation failure to the error recorded on the
// installed app.
func RefreshedErrorOf(err error) model.RefreshedError {
	if errors.Is(err, manager.ErrAccountInvalid) {
		return model.RefreshedErrorInvalidAccount
	}
	switch signing.ClassOf(err) {
	case signing.ClassIdentity:
		return model.RefreshedErrorSigningIdentity
	case signing.ClassSigning:
		return model.RefreshedErrorSigning
	case signing.ClassTransport:
		return model.RefreshedErrorTransport
	default:
		return model.RefreshedErrorInvalidOther
	}
}

// loadSigningIdentity loads the identity, reporting storage failures as
// identity errors so every pipeline failure carries a class.
func loadSigningIdentity(id uint) (*model.SigningIdentity, error) {
	identity, err := GetSigningIdentity(id)
	if err != nil {
		return nil, asSigningError(err, signing.ClassIdentity, signing.CodeIdentityNotFound, "cannot load signing identity %d", id)
	}
	return identity, nil
}

// planOptionsOf returns the plan options recorded on app.
func planOptionsOf(app model.InstalledApp) planOptions {
	return planOptions{
		RemoveExtensions:         app.RemoveExtensions,
		AllowMissingEntitlements: app.AllowMissingEntitlements,
		CustomIdentifier:         app.CustomIdentifier,
	}
}

// planExternalInstall analyzes the IPA and simulates its signing with the
// identity for the device. The identity status is prepended to the plan issues.
func planExternalInstall(identity model.SigningIdentity, ipaPath string, dev *model.Device, opts planOptions, lookup deviceInfoLookup) (*appcheck.Plan, error) {
	info, err := lookup(dev)
	if err != nil {
		return nil, signing.Wrap(signing.ClassTransport, signing.CodeDeviceUnreachable, err, "cannot query device %s", dev.UDID)
	}
	udids := []string{dev.UDID}
	if info.UniqueDeviceID != "" && info.UniqueDeviceID != dev.UDID {
		udids = append(udids, info.UniqueDeviceID)
	}
	// The lockdown DeviceClass is authoritative; the device list class may
	// come from a name heuristic.
	deviceClass := info.DeviceClass
	if deviceClass == "" {
		deviceClass = dev.DeviceClass
	}

	ipa, err := appcheck.AnalyzeIPA(ipaPath)
	if err != nil {
		return nil, asSigningError(err, signing.ClassSigning, signing.CodeIPAInvalid, "cannot read the IPA")
	}
	profile, err := signing.VerifyProfile(identity.ProfileData)
	if err != nil {
		return nil, asSigningError(err, signing.ClassIdentity, signing.CodeProfileInvalid, "the stored provisioning profile is invalid")
	}

	plan := appcheck.BuildPlan(appcheck.PlanInput{
		IPA:                      ipa,
		Profile:                  profile,
		DeviceClass:              model.DeviceClass(deviceClass),
		DeviceUDIDs:              udids,
		RemoveExtensions:         opts.RemoveExtensions,
		AllowMissingEntitlements: opts.AllowMissingEntitlements,
		CustomIdentifier:         opts.CustomIdentifier,
	})
	issues := append([]signing.Issue{}, signing.IdentityStatus(identity, time.Now())...)
	plan.Issues = append(issues, plan.Issues...)
	return plan, nil
}

// asSigningError returns err when it already carries a signing class, else
// wraps it with class and code.
func asSigningError(err error, class signing.Class, code string, format string, args ...any) error {
	var e *signing.Error
	if errors.As(err, &e) {
		return err
	}
	return signing.Wrap(class, code, err, format, args...)
}

// signingReport is one SIGNING_REPORT line. Fields irrelevant to a stage are omitted.
type signingReport struct {
	Stage string `json:"stage"`
}

type signingPlanReport struct {
	Stage              string          `json:"stage"`
	Blocking           bool            `json:"blocking"`
	Issues             []signing.Issue `json:"issues"`
	MainBundleID       string          `json:"main_bundle_id"`
	SignedMainBundleID string          `json:"signed_main_bundle_id"`
	CustomIdentifier   string          `json:"custom_identifier"`
	// MainApplicationIdentifier is the application identifier the main app
	// is signed with (derived from the profile); empty without a main app.
	MainApplicationIdentifier string   `json:"main_application_identifier"`
	RemovedBundles            []string `json:"removed_bundles"`
}

type signingFailureReport struct {
	Stage   string          `json:"stage"`
	Class   signing.Class   `json:"class"`
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Issues  []signing.Issue `json:"issues"`
}

func planReportLine(plan *appcheck.Plan) string {
	report := signingPlanReport{
		Stage:              "plan",
		Blocking:           plan.Blocking(),
		Issues:             append([]signing.Issue{}, plan.Issues...),
		MainBundleID:       plan.MainBundleID,
		SignedMainBundleID: plan.SignedMainBundleID,
		CustomIdentifier:   plan.CustomIdentifier,
		RemovedBundles:     []string{},
	}
	for _, bundle := range plan.Bundles {
		if bundle.Removed {
			report.RemovedBundles = append(report.RemovedBundles, bundle.Path)
		} else if bundle.OriginalID == plan.MainBundleID && report.MainApplicationIdentifier == "" {
			report.MainApplicationIdentifier = bundle.ExpectedApplicationIdentifier
		}
	}
	return signingReportLine(report)
}

func failureReportLine(err error) string {
	var e *signing.Error
	if !errors.As(err, &e) {
		e = signing.Wrap(signing.ClassSigning, signing.CodeEngineFailed, err, "the external installation failed")
	}
	message := e.Message
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return signingReportLine(signingFailureReport{
		Stage:   "failure",
		Class:   e.Class,
		Code:    e.Code,
		Message: message,
		Issues:  append([]signing.Issue{}, e.Issues...),
	})
}

// signingReportLine encodes report as one SIGNING_REPORT line; JSON escapes
// keep it on a single line.
func signingReportLine(report any) string {
	data, err := json.Marshal(report)
	if err != nil {
		log.Err(err).Msg("Failed to encode the signing report")
		return ""
	}
	return SigningReportPrefix + string(data) + "\n"
}

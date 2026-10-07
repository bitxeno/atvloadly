package manager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	execx "github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/gookit/event"
)

var ErrAccountInvalid = errors.New("account invalid")

// ErrCertificateResetRequired marks an Apple ID installation that stopped
// because the account holds as many signing certificates as Apple allows and
// revoking one was not authorized. The engine revoked nothing; a user has to
// pick a certificate on the install page first.
var ErrCertificateResetRequired = errors.New("certificate reset required")

// installTimeout bounds one signing engine run. Large tvOS apps can take
// longer than 30 minutes to sign and install.
const installTimeout = 60 * time.Minute

// maxCapturedOutput bounds the engine output kept in memory for one
// installation; older output is dropped and replaced by a truncation marker.
const maxCapturedOutput = 4 << 20

const outputTruncatedMarker = "[... earlier output truncated ...]\n"

type InstallManager struct {
	quietMode bool

	outputStdout *outputWriter

	stdin io.WriteCloser

	cancel              context.CancelFunc
	em                  *event.Manager
	ProvisioningProfile *model.MobileProvisioningProfile
}

type InstallOptions struct {
	UDID             string
	IP               string
	Port             uint16
	Account          string
	IpaPath          string
	CustomName       string
	RemoveExtensions bool
	RefreshMode      bool
	// SigningMode selects how the app is signed; empty means SigningModeAppleID.
	SigningMode model.SigningMode
	// External holds the signing material of SigningModeExternalCertificate.
	External *ExternalSigning
	// RevokeCertificateSerial authorizes the engine to revoke this one
	// certificate when the Apple ID account is at its certificate limit.
	// Apple ID mode only; empty keeps every certificate untouched.
	RevokeCertificateSerial string
}

func NewInstallManager() *InstallManager {
	em := event.NewManager("output", event.UsePathMode)
	return &InstallManager{
		quietMode:    true,
		outputStdout: newOutputWriter(em),

		em: em,
	}
}

func NewInteractiveInstallManager() *InstallManager {
	ins := NewInstallManager()
	ins.quietMode = false
	return ins
}

func (t *InstallManager) TryStart(ctx context.Context, opts InstallOptions) error {
	err := t.Start(ctx, opts)
	if err != nil {
		if opts.SigningMode.OrDefault() == model.SigningModeAppleID && t.NeedsCertificateReset() {
			// A background refresh has no user to ask: report that a
			// certificate slot is missing and that nothing was revoked.
			return fmt.Errorf("%s %s %w", t.ErrorLog(), err.Error(), ErrCertificateResetRequired)
		}
		if opts.SigningMode.OrDefault() == model.SigningModeAppleID && t.IsAccountInvalid() {
			return fmt.Errorf("%s %s %w", t.ErrorLog(), err.Error(), ErrAccountInvalid)
		}
		if !shouldRetryInstall(opts.SigningMode, err, ctx.Err()) {
			return err
		}

		// AppleTV system has reboot/lockdownd sleep, try restart usbmuxd to fix
		// LOCKDOWN_E_MUX_ERROR / AFC_E_MUX_ERROR /
		ipaName := filepath.Base(opts.IpaPath)
		log.Infof("Try restarting usbmuxd to fix afc connect issue. %s", ipaName)
		if errmux := usbmuxdManager.Restart(); errmux == nil {
			// iPhone reconnect may take a while, wait some time
			usbmuxdManager.TryWaitReady(30 * time.Second)
			log.Infof("Restart usbmuxd complete, try install ipa again. %s", ipaName)
			err = t.Start(ctx, opts)
		}
	}
	return err
}

// Start signs and installs the IPA once with the signing mode of opts.
// External certificate failures are *signing.Error values classified from
// the engine output.
func (t *InstallManager) Start(ctx context.Context, opts InstallOptions) error {
	switch opts.SigningMode.OrDefault() {
	case model.SigningModeAppleID:
		return t.startAppleID(ctx, opts)
	case model.SigningModeExternalCertificate:
		return t.startExternal(ctx, opts)
	default:
		return fmt.Errorf("unknown signing mode: %q", opts.SigningMode)
	}
}

func (t *InstallManager) startAppleID(ctx context.Context, opts InstallOptions) error {
	t.outputStdout.Reset()

	ctx = t.withRunTimeout(ctx)

	runEnv, runTempDir, err := appleIDRunEnv(GetRunEnvs())
	if err != nil {
		return fmt.Errorf("create isolated install temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(runTempDir); err != nil {
			log.Warnf("failed to remove install temp dir %s: %v", runTempDir, err)
		}
	}()

	provisionPath := t.GetMobileProvisionPath()
	defer func() {
		if _, err := os.Stat(provisionPath); err == nil {
			_ = os.Remove(provisionPath)
		}
	}()

	if err := CheckAfcServiceStatus(opts.UDID); err != nil {
		return fmt.Errorf("afc service not available: %w", err)
	}

	args := buildInstallArgs(opts, provisionPath)
	if opts.RemoveExtensions {
		args = append(args, "--remove-extensions")
	}
	if opts.RefreshMode {
		args = append(args, "--refresh")
	}
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		log.Err(err).Msg("Error creating stdin pipe: ")
		return err
	}
	t.stdin = stdinWriter
	defer func() {
		_ = stdinReader.Close()
		_ = t.stdin.Close()
		t.stdin = nil
	}()

	if err := t.runEngine(ctx, args, app.Config.Server.DataDir, runEnv, stdinReader); err != nil {
		return err
	}

	if provisionProfile, perr := model.ParseMobileProvisioningProfileFile(provisionPath); perr == nil {
		t.ProvisioningProfile = provisionProfile
	}

	return nil
}

// appleIDRunEnv gives one Apple ID signing engine run its own temporary root.
// PlumeImpactor creates plume_stage_* below TMPDIR, so isolating TMPDIR keeps
// concurrent installs from sharing staging directories while preserving the
// shared HOME that contains Apple ID sessions.
func appleIDRunEnv(base []string) ([]string, string, error) {
	tempDir, err := os.MkdirTemp("", "atvloadly-plumesign-*")
	if err != nil {
		return nil, "", err
	}
	return utils.MergeEnvs(base, []string{"TMPDIR=" + tempDir}), tempDir, nil
}

// withRunTimeout bounds ctx by installTimeout and makes it the context
// cancelled by Close.
func (t *InstallManager) withRunTimeout(ctx context.Context) context.Context {
	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	// Release the previous install context: overwriting t.cancel without
	// calling it would keep the old 60-minute timeout timer alive until it
	// fires, leaking the context and its timer on every repeated install.
	if t.cancel != nil {
		t.cancel()
	}
	t.cancel = cancel
	return ctx
}

// runEngine runs plumesign with args, streaming its output into the captured
// install output. A nil stdin connects the engine to the null device.
func (t *InstallManager) runEngine(ctx context.Context, args []string, dir string, env []string, stdin io.Reader) error {
	cmd := execx.CommandContext(ctx, "plumesign", args...).
		WithTimeout(installTimeout).
		WithDir(dir).
		WithEnv(env).
		WithStdout(t.outputStdout).
		WithStderr(t.outputStdout).
		WithStdin(stdin)

	log.Debugf("Install Command: %s", strings.Join(append([]string{cmd.Name}, cmd.Args...), " "))

	err := cmd.Run()
	if err != nil {
		if errors.Is(err, execx.ErrCommandTimeout) {
			log.Err(err).Msgf("Installation exceeded %d-minute timeout limit. %s", int(installTimeout.Minutes()), t.ErrorLog())
			return fmt.Errorf("installation exceeded %d-minute timeout limit: %w", int(installTimeout.Minutes()), err)
		}
		return err
	}
	return nil
}

func buildInstallArgs(opts InstallOptions, provisionPath string) []string {
	var args []string
	if opts.IP != "" && opts.Port != 0 && opts.UDID != "" {
		args = []string{"sign-rsd", "--apple-id", "--register-and-install", "--output-provision", provisionPath, "--ip", opts.IP, "--port", fmt.Sprintf("%d", opts.Port), "--udid", opts.UDID, "-u", opts.Account, "-p", opts.IpaPath}
	} else {
		args = []string{"sign", "--apple-id", "--register-and-install", "--output-provision", provisionPath, "--udid", opts.UDID, "-u", opts.Account, "-p", opts.IpaPath}
	}

	// Renames the app on the home screen. The bundle identifier is deliberately
	// left untouched, so a renamed install still replaces the previous one
	// instead of registering a second App ID.
	if opts.CustomName != "" {
		args = append(args, "--custom-name", opts.CustomName)
	}

	// The engine revokes nothing unless handed this one-shot authorization,
	// which the install page gives only after the user picked a certificate.
	if opts.RevokeCertificateSerial != "" {
		args = append(args, "--revoke-certificate", opts.RevokeCertificateSerial)
	}

	return args
}

func (t *InstallManager) GetMobileProvisionPath() string {
	return path.Join(os.TempDir(), fmt.Sprintf("embedded.mobileprovision.%d", time.Now().UnixNano()))
}

// reservedUploadTempFiles holds the canonical paths of upload temp files
// owned by a queued or running installation. Stale-file sweeps keep them
// until they are released.
var reservedUploadTempFiles sync.Map

// ReserveUploadTempFiles marks upload temp files as owned by a queued or
// running installation so that stale-file sweeps keep them. Remote URLs
// and unresolvable paths are ignored.
func ReserveUploadTempFiles(paths ...string) {
	for _, p := range paths {
		if canonical := canonicalTempPath(p); canonical != "" {
			reservedUploadTempFiles.Store(canonical, struct{}{})
		}
	}
}

// ReleaseUploadTempFiles drops the ownership marks of ReserveUploadTempFiles,
// letting stale-file sweeps remove the files again.
func ReleaseUploadTempFiles(paths ...string) {
	for _, p := range paths {
		if canonical := canonicalTempPath(p); canonical != "" {
			reservedUploadTempFiles.Delete(canonical)
		}
	}
}

// UploadTempFileReserved reports whether the upload temp file at path is
// owned by a queued or running installation and must be kept by stale-file
// sweeps.
func UploadTempFileReserved(path string) bool {
	canonical := canonicalTempPath(path)
	if canonical == "" {
		return false
	}
	_, reserved := reservedUploadTempFiles.Load(canonical)
	return reserved
}

// canonicalTempPath resolves p to an absolute path with symlinks resolved,
// or returns "" for remote URLs and unresolvable paths. Reservations and
// removals compare canonical paths, so callers may hand in either form.
func canonicalTempPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || isRemotePath(p) {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

// uploadTempDir returns the canonical absolute path of the upload temp
// directory.
func uploadTempDir() (string, error) {
	dir := canonicalTempPath(filepath.Join(app.Config.Server.DataDir, "tmp"))
	if dir == "" {
		return "", errors.New("resolve upload temp directory")
	}
	return dir, nil
}

func (t *InstallManager) CleanTempFiles(files ...string) {
	CleanUploadTempFiles(files...)
}

// CleanUploadTempFiles removes the given upload temp files without needing an
// InstallManager instance. Paths outside the upload temp directory are
// skipped.
func CleanUploadTempFiles(files ...string) {
	for _, filePath := range files {
		if err := removeUploadTempFile(filePath); err != nil {
			log.Warnf("failed to clean upload temp file %s: %v", filePath, err)
		}
	}
}

// removeUploadTempFile removes a single file created in the upload temp
// directory. Remote URLs, paths outside the temp directory and directories
// are skipped. Removing an already gone file is not an error.
func removeUploadTempFile(filePath string) error {
	target := canonicalTempPath(filePath)
	if target == "" {
		return nil
	}

	tempDir, err := uploadTempDir()
	if err != nil {
		return err
	}

	// Uploads and extracted icons are created as direct files in <data>/tmp.
	// Reject the directory itself, nested paths and every path outside it.
	if target == tempDir || filepath.Dir(target) != tempDir {
		return nil
	}

	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect upload temp file: %w", err)
	}
	if info.IsDir() {
		return nil
	}

	if err := os.Remove(target); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove upload temp file: %w", err)
	}
	return nil
}

func isRemotePath(p string) bool {
	return strings.HasPrefix(p, "http:") || strings.HasPrefix(p, "https:")
}

func (t *InstallManager) Close() {
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
	if t.em != nil {
		_ = t.em.CloseWait()
	}
}

func (t *InstallManager) OnOutput(fn func(string)) {
	t.em.On("output", event.ListenerFunc(func(e event.Event) error {
		fn(e.Get("text").(string))
		return nil
	}))
}

func (t *InstallManager) Write(p []byte) {
	if t.stdin != nil {
		_, _ = t.stdin.Write(p)
	}
}

func (t *InstallManager) ErrorLog() string {
	data := t.outputStdout.String()
	if data == "" {
		return ""
	}

	var lines []string
	for _, l := range strings.Split(data, "\n") {
		if strings.HasPrefix(strings.ToLower(l), "error") {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n")
}

func (t *InstallManager) IsAccountInvalid() bool {
	log := t.OutputLog()
	return strings.Contains(log, "plumesign account list") || strings.Contains(log, "Can't log-in") || strings.Contains(log, "DeveloperSession creation failed")
}

// certificateResetRequiredMarker is the stable token the signing engine puts on
// the error it returns when the account is at its certificate limit and no
// revocation was authorized. It matches the display of the engine's
// Error::CertificateResetRequired.
const certificateResetRequiredMarker = "[certificate_reset_required]"

// NeedsCertificateReset reports whether the run stopped because a certificate
// slot had to be freed and the user never authorized revoking one.
func (t *InstallManager) NeedsCertificateReset() bool {
	return strings.Contains(t.OutputLog(), certificateResetRequiredMarker)
}

func (t *InstallManager) IsSuccess() bool {
	log := t.OutputLog()
	return strings.Contains(log, "Installation Succeeded") || strings.Contains(log, "Installation complete")
}

func (t *InstallManager) OutputLog() string {
	return t.outputStdout.String()
}

func (t *InstallManager) WriteLog(msg string) {
	_, _ = t.outputStdout.Write([]byte(msg))
}

// ResetLog clears the captured output. Apple ID runs reset it at every
// attempt; external certificate installations reset it once per installation
// so their SIGNING_REPORT lines and every attempt stay in the task log.
func (t *InstallManager) ResetLog() {
	t.outputStdout.Reset()
}

func (t *InstallManager) SaveLog(id uint) {
	data := t.OutputLog()

	// Hide log password string
	// data = strings.Replace(data, v.Password, "******", -1)

	saveDir := filepath.Join(app.Config.Server.DataDir, "log")
	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		log.Error("failed to create directory :" + saveDir)
		return
	}

	path := filepath.Join(saveDir, fmt.Sprintf("task_%d.log", id))
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		log.Error("write log failed :" + path)
		return
	}
}

// outputWriter captures the install output, keeping only the most recent
// maxCapturedOutput bytes, and forwards every write to the output listeners.
type outputWriter struct {
	mu        sync.Mutex
	data      []byte
	written   int64
	truncated bool
	limit     int
	em        *event.Manager
}

func newOutputWriter(em *event.Manager) *outputWriter {
	return &outputWriter{
		limit: maxCapturedOutput,
		em:    em,
	}
}

func (w *outputWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	w.data = append(w.data, p...)
	w.written += int64(len(p))
	// Trim with some slack so the retained window is compacted in place once
	// in a while instead of on every write.
	if len(w.data) > w.limit+w.limit/4 {
		drop := len(w.data) - w.limit
		copy(w.data, w.data[drop:])
		w.data = w.data[:w.limit]
		w.truncated = true
	}
	w.mu.Unlock()

	w.em.MustFire("output", event.M{"text": string(p)})

	n = len(p)
	return n, nil
}

func (w *outputWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.truncated {
		return outputTruncatedMarker + string(w.data)
	}
	return string(w.data)
}

// Mark returns the position of the next written byte, for Since.
func (w *outputWriter) Mark() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.written
}

// Since returns the retained output written after mark, or all of it when the
// output was reset after mark was taken.
func (w *outputWriter) Since(mark int64) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if mark > w.written {
		return string(w.data)
	}
	start := int64(len(w.data)) - (w.written - mark)
	if start < 0 {
		start = 0
	}
	return string(w.data[start:])
}

func (w *outputWriter) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.data = []byte{}
	w.written = 0
	w.truncated = false
}

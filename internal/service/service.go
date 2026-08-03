package service

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	stdhttp "net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/i18n"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/model"
	ps "github.com/mitchellh/go-ps"
)

var (
	regValidName   = regexp.MustCompile("[^0-9a-zA-Z]+")
	regCustomName  = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} .-]*$`)
	avahiDaemonPid = "/var/run/avahi-daemon/pid"
)

// MaxCustomNameLength caps the home-screen name so an over-long value cannot
// reach Apple's App ID registration.
const MaxCustomNameLength = 64

// ValidateCustomName checks a user-supplied app name. Apple rejects characters
// such as underscores in the App ID name derived from the app name (see issue
// #21, "An invalid value 'tv_shelf' was provided for the parameter
// 'appIdName'"), so reject them here instead of failing during signing.
// Letters and digits of any script are allowed, so non-Latin app names work.
func ValidateCustomName(name string) error {
	if name == "" {
		return nil
	}
	if utf8.RuneCountInString(name) > MaxCustomNameLength {
		return fmt.Errorf("app name must be at most %d characters", MaxCustomNameLength)
	}
	if !regCustomName.MatchString(name) {
		return errors.New("app name may only contain letters, numbers, spaces, dots and hyphens, and must start with a letter or number")
	}
	return nil
}

func GetServiceStatus() []model.ServiceStatus {
	status := []model.ServiceStatus{}

	if runtime.GOOS == "linux" {
		proc := "avahi-daemon"
		_, err := os.Stat(avahiDaemonPid)
		status = append(status, model.ServiceStatus{
			Name:    proc,
			Running: err == nil,
		})
	}
	if runtime.GOOS == "darwin" {
		proc := "mDNSResponder"
		status = append(status, model.ServiceStatus{
			Name:    "Apple Bonjour",
			Running: checkProcessExists(proc),
		})
	}

	proc := "usbmuxd"
	status = append(status, model.ServiceStatus{
		Name:    proc,
		Running: checkProcessExists(proc),
	})

	return status
}

func checkProcessExists(name string) bool {
	processes, err := ps.Processes()
	if err != nil {
		return false
	}
	for _, p := range processes {
		n := p.Executable()
		if n == name {
			return true
		}
	}
	return false
}

// MountDeveloperDiskImage mounts the personalized developer disk image on
// the target device. The operation is idempotent within the lifetime of the
// process — subsequent calls for the same device are no-ops.
func MountDeveloperDiskImage(ctx context.Context, id string) error {
	device, ok := manager.GetDeviceByID(id)
	if !ok {
		return fmt.Errorf("device not found: %s", id)
	}

	if device.Connection != model.DeviceConnectionRemote {
		return fmt.Errorf("mount is only supported on remote paired (RSD) devices")
	}

	mgr := manager.NewScreenshotManager()
	if _, err := mgr.EnsureMounted(ctx, device); err != nil {
		return err
	}

	return nil
}

// TakeDeviceScreenshot captures a screenshot of the target device and
// returns it base64-encoded so it can be embedded directly in JSON.
func TakeDeviceScreenshot(ctx context.Context, id string) (string, error) {
	device, ok := manager.GetDeviceByID(id)
	if !ok {
		return "", fmt.Errorf("device not found: %s", id)
	}

	if device.Connection != model.DeviceConnectionRemote {
		return "", fmt.Errorf("screenshot is only supported on remote paired (RSD) devices")
	}

	mgr := manager.NewScreenshotManager()
	jpgBytes, err := mgr.TakeScreenshot(ctx, device)
	if err != nil {
		return "", err
	}
	return manager.ScreenshotDataBase64(jpgBytes), nil
}

func CheckAfcService(ctx context.Context, id string) error {
	device, ok := manager.GetDeviceByID(id)
	if !ok {
		return fmt.Errorf("device not found: %s", id)
	}

	var err error
	if err = manager.CheckAfcServiceStatus(device.UDID); err != nil {
		log.Infof("check afc service status error: %s", err)
		// try restart usbmuxd to fix afc connect issue
		if err = manager.Usbmuxd().Restart(); err == nil {
			time.Sleep(5 * time.Second)
			err = manager.CheckAfcServiceStatus(device.UDID)
		}
	}

	return err
}

func GetValidName(name string) string {
	return strings.ToLower(regValidName.ReplaceAllString(name, ""))
}

func SetLanguage(lang string) {
	if lang == "" {
		return
	}
	lang = strings.ToLower(lang)
	// get first one language from http [Accept-Language] header
	if strings.Contains(lang, ",") {
		lang = strings.Split(lang, ",")[0]
	}
	if strings.Contains(lang, ";") {
		lang = strings.Split(lang, ";")[0]
	}
	if app.Settings.App.Language != lang {
		app.Settings.App.Language = lang
		app.SaveSettings()

		i18n.SetLanguage(lang)
	}
}

// UpdateCoreADI downloads the latest Apple Music APK and saves it to DataDir/PlumeImpactor/lib
func UpdateCoreADI() error {
	// Determine package arch used inside the APK zip
	pkgArch := "x86_64"
	if runtime.GOARCH == "arm64" {
		pkgArch = "arm64-v8a"
	}

	url := "https://apps.mzstatic.com/content/android-apple-music-apk/applemusic.apk"
	resp, err := stdhttp.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != stdhttp.StatusOK {
		return fmt.Errorf("download failed, status: %d", resp.StatusCode)
	}

	tmpDir := filepath.Join(app.Config.Server.DataDir, "tmp")
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return fmt.Errorf("create tmp dir failed: %w", err)
	}
	tmpPath := filepath.Join(tmpDir, "applemusic.apk")

	tmpOut, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file failed: %w", err)
	}
	if _, err := io.Copy(tmpOut, resp.Body); err != nil {
		_ = tmpOut.Close()
		return fmt.Errorf("save temp file failed: %w", err)
	}
	_ = tmpOut.Close()

	defer func() {
		_ = os.Remove(tmpPath)
	}()

	// open zip and extract specific lib files
	zr, err := zip.OpenReader(tmpPath)
	if err != nil {
		return fmt.Errorf("open apk zip failed: %w", err)
	}
	defer func() {
		_ = zr.Close()
	}()

	destDir := filepath.Join(app.Config.Server.DataDir, "PlumeImpactor", "lib", pkgArch)
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return fmt.Errorf("create dir failed: %w", err)
	}

	targets := map[string]string{
		filepath.ToSlash(filepath.Join("lib", pkgArch, "libstoreservicescore.so")): "libstoreservicescore.so",
		filepath.ToSlash(filepath.Join("lib", pkgArch, "libCoreADI.so")):           "libCoreADI.so",
	}

	for _, f := range zr.File {
		name := f.Name
		// normalize slashes
		name = filepath.ToSlash(name)
		if destName, ok := targets[name]; ok {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("open zip entry failed: %w", err)
			}
			dstPath := filepath.Join(destDir, destName)
			outf, err := os.Create(dstPath)
			if err != nil {
				_ = rc.Close()
				return fmt.Errorf("create dest file failed: %w", err)
			}
			if _, err := io.Copy(outf, rc); err != nil {
				_ = rc.Close()
				_ = outf.Close()
				return fmt.Errorf("copy entry failed: %w", err)
			}
			_ = rc.Close()
			_ = outf.Close()
			delete(targets, name)
		}
	}

	if len(targets) > 0 {
		missing := []string{}
		for k := range targets {
			missing = append(missing, k)
		}
		return fmt.Errorf("missing files in apk: %v", missing)
	}

	log.Infof("Update CoreADI success. path: %s", destDir)
	return nil
}

func ImportPairingFile(ip string, port string, data []byte) error {
	// Call manager to process the file
	if err := manager.ImportPairingFile(ip, port, data); err != nil {
		return err
	}

	// force reload devices
	manager.StartDeviceManager()

	return nil
}

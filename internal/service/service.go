package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/artdarek/go-unzip/pkg/unzip"
	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/http"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/manager"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
	ps "github.com/mitchellh/go-ps"
)

var (
	regValidName   = regexp.MustCompile("[^0-9a-zA-Z]+")
	avahiDaemonPid = "/var/run/avahi-daemon/pid"
)

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

func MountDeveloperDiskImage(ctx context.Context, id string) error {
	device, ok := manager.GetDeviceByID(id)
	if !ok {
		return fmt.Errorf("Device not found: %s", id)
	}

	imageInfo, err := manager.GetDeviceMountImageInfo(device.UDID)
	if err != nil {
		log.Err(err).Msg("GetDeviceMountImageInfo error: ")
		return err
	}

	// Already mounted, return directly.
	if imageInfo.ImageMounted {
		return nil
	}

	// Download DeveloperDiskImage
	dmg, signature, err := downloadDeveloperDiskImage(imageInfo)
	if err != nil {
		log.Err(err).Msg("Download Developer disk image error: ")
		return err
	}

	// Start executing mounting DeveloperDiskImage
	cmd := exec.CommandContext(ctx, "ideviceimagemounter", "-u", device.UDID, "-n", dmg, signature)
	data, err := cmd.CombinedOutput()
	if err != nil {
		log.Err(err).Msgf("Run ideviceimagemounter error: %s", string(data))
		return fmt.Errorf("%s%s", string(data), err.Error())
	}

	return nil
}

func CheckAfcService(ctx context.Context, id string) error {
	device, ok := manager.GetDeviceByID(id)
	if !ok {
		return fmt.Errorf("device not found: %s", id)
	}

	var err error
	if err = manager.CheckAfcServiceStatus(device.ID); err != nil {
		// try restart usbmuxd to fix afc connect issue
		if err = manager.RestartUsbmuxd(); err == nil {
			time.Sleep(5 * time.Second)
			err = manager.CheckAfcServiceStatus(device.ID)
		}
	}

	return err
}

func downloadDeveloperDiskImage(imageInfo *model.UsbmuxdImage) (dmg string, signature string, reterr error) {
	// download current version DeveloperDiskImage
	err := downloadDeveloperDiskImageByVersion(imageInfo.DeveloperDiskImageUrl, imageInfo.DeveloperDiskImageVersion)
	if err == nil {
		dmg = filepath.Join(app.Config.Server.DataDir, "DeveloperDiskImage", imageInfo.DeveloperDiskImageVersion, "DeveloperDiskImage.dmg")
		signature = filepath.Join(app.Config.Server.DataDir, "DeveloperDiskImage", imageInfo.DeveloperDiskImageVersion, "DeveloperDiskImage.dmg.signature")
		return
	}

	if err != nil && imageInfo.VersionMinor > 0 {
		// current version DeveloperDiskImage not found, try fallback to last minor version
		for fallbackMinor := imageInfo.VersionMinor - 1; fallbackMinor > 0; fallbackMinor-- {
			fallbackVersion := fmt.Sprintf("%d.%d", imageInfo.VersionMajor, fallbackMinor)
			fallbackImageUrl := strings.Replace(app.Config.App.DeveloperDiskImage.ImageSource, "{0}", fallbackVersion, -1)
			log.Warnf("try downgrade developer disk image to version: %s", fallbackVersion)
			if err := downloadDeveloperDiskImageByVersion(fallbackImageUrl, fallbackVersion); err == nil {
				dmg = filepath.Join(app.Config.Server.DataDir, "DeveloperDiskImage", fallbackVersion, "DeveloperDiskImage.dmg")
				signature = filepath.Join(app.Config.Server.DataDir, "DeveloperDiskImage", fallbackVersion, "DeveloperDiskImage.dmg.signature")
				return
			}
		}
	}

	if err != nil {
		log.Err(err).Msg("Download developer disk image error: ")
		reterr = err
	}

	return
}

func downloadDeveloperDiskImageByVersion(url string, version string) error {
	imageVersionDir := filepath.Join(app.Config.Server.DataDir, "DeveloperDiskImage", version)
	dmg := filepath.Join(imageVersionDir, "DeveloperDiskImage.dmg")
	signature := filepath.Join(imageVersionDir, "DeveloperDiskImage.dmg.signature")
	if utils.Exists(dmg) && utils.Exists(signature) {
		return nil
	}

	tmpPath := filepath.Join(app.Config.Server.DataDir, "tmp", "DeveloperDiskImage.zip")
	tmpUnzipPath := filepath.Join(app.Config.Server.DataDir, "tmp", "DeveloperDiskImage")
	_ = os.RemoveAll(tmpUnzipPath)

	// download current version DeveloperDiskImage
	hasDownloaded := false
	if app.Config.App.DeveloperDiskImage.CNProxy != "" {
		// download by proxy
		cnProxyUrl := strings.TrimSuffix(app.Config.App.DeveloperDiskImage.CNProxy, "/") + "/" + url
		if resp, err := http.NewClient().R().SetOutput(tmpPath).Get(cnProxyUrl); err == nil && resp.IsSuccess() {
			hasDownloaded = true
		}
	}
	if !hasDownloaded {
		resp, err := http.NewClient().R().SetOutput(tmpPath).Get(url)
		if err != nil {
			return err
		}
		if !resp.IsSuccess() {
			return fmt.Errorf("developer disk image download failed.  url: %s status: %d", url, resp.StatusCode())
		}
	}

	// unzip
	uz := unzip.New()
	files, err := uz.Extract(tmpPath, tmpUnzipPath)
	if err != nil {
		log.Err(err).Msg("Unzip Developer disk image error: ")
		return err
	}
	_ = os.MkdirAll(imageVersionDir, os.ModePerm)
	for _, f := range files {
		if filepath.Base(f) == "DeveloperDiskImage.dmg" || filepath.Base(f) == "DeveloperDiskImage.dmg.signature" {
			if err = os.Rename(filepath.Join(tmpUnzipPath, f), filepath.Join(imageVersionDir, filepath.Base(f))); err != nil {
				return err
			}
		}
	}

	return nil
}

func GetValidName(name string) string {
	return strings.ToLower(regValidName.ReplaceAllString(name, ""))
}

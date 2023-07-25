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

	"github.com/artdarek/go-unzip/pkg/unzip"
	"github.com/bitxeno/atvloadly/internal/cfg"
	"github.com/bitxeno/atvloadly/internal/http"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/bitxeno/atvloadly/manager"
	"github.com/bitxeno/atvloadly/model"
	"github.com/shirou/gopsutil/v3/process"
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

	proc = "anisette-server"
	status = append(status, model.ServiceStatus{
		Name:    proc,
		Running: checkProcessExists(proc),
	})

	return status
}

func checkProcessExists(name string) bool {
	processes, err := process.Processes()
	if err != nil {
		return false
	}
	for _, p := range processes {
		n, err := p.Name()
		if err != nil {
			return false
		}
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
	imageVersionDir := filepath.Join(cfg.Server.WorkDir, "DeveloperDiskImage", imageInfo.DeveloperDiskImageVersion)
	dmg := filepath.Join(imageVersionDir, "DeveloperDiskImage.dmg")
	signature := filepath.Join(imageVersionDir, "DeveloperDiskImage.dmg.signature")
	fallback, err := downloadDeveloperDiskImage(imageInfo)
	if err != nil {
		log.Err(err).Msg("Download Developer disk image error: ")
		return err
	}
	if fallback {
		imageVersionDir = filepath.Join(cfg.Server.WorkDir, "DeveloperDiskImage", imageInfo.DeveloperDiskImageFallbackVersion)
		dmg = filepath.Join(imageVersionDir, "DeveloperDiskImage.dmg")
		signature = filepath.Join(imageVersionDir, "DeveloperDiskImage.dmg.signature")
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

func downloadDeveloperDiskImage(imageInfo *model.UsbmuxdImage) (fallback bool, reterr error) {
	// download current version DeveloperDiskImage
	err := downloadDeveloperDiskImageByVersion(imageInfo.DeveloperDiskImageUrl, imageInfo.DeveloperDiskImageVersion)
	if err != nil && imageInfo.DeveloperDiskImageFallbackVersion != "" {
		log.Warnf("try downgrade developer disk image to version: %s", imageInfo.DeveloperDiskImageFallbackVersion)
		// current version DeveloperDiskImage not exist, fallback to last minor version
		reterr = downloadDeveloperDiskImageByVersion(imageInfo.DeveloperDiskImageFallbackUrl, imageInfo.DeveloperDiskImageFallbackVersion)
		fallback = true
		return
	}

	if err != nil {
		log.Err(err).Msg("Download developer disk image error: ")
		reterr = err
	}

	return
}

func downloadDeveloperDiskImageByVersion(url string, version string) error {
	imageVersionDir := filepath.Join(cfg.Server.WorkDir, "DeveloperDiskImage", version)
	dmg := filepath.Join(imageVersionDir, "DeveloperDiskImage.dmg")
	signature := filepath.Join(imageVersionDir, "DeveloperDiskImage.dmg.signature")
	if utils.Exists(dmg) && utils.Exists(signature) {
		return nil
	}

	tmpPath := filepath.Join(cfg.Server.WorkDir, "tmp", "DeveloperDiskImage.zip")
	tmpUnzipPath := filepath.Join(cfg.Server.WorkDir, "tmp", "DeveloperDiskImage")
	_ = os.RemoveAll(tmpUnzipPath)

	// download current version DeveloperDiskImage
	resp, err := http.NewClient().R().SetOutput(tmpPath).Get(url)
	if err != nil {
		return err
	}
	if !resp.IsSuccess() {
		return fmt.Errorf("Developer disk image download failed.  url: %s status: %d", url, resp.StatusCode())
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

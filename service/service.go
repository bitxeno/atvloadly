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

	// 已挂载直接返回
	if imageInfo.ImageMounted {
		return nil
	}

	// 尝试挂载
	// 对应版本的DeveloperDiskImage不存在的话，尝试下载
	imageVersionDir := filepath.Join(cfg.Server.WorkDir, "DeveloperDiskImage", imageInfo.DeveloperDiskImageVersion)
	dmg := filepath.Join(imageVersionDir, "DeveloperDiskImage.dmg")
	signature := filepath.Join(imageVersionDir, "DeveloperDiskImage.dmg.signature")

	if _, err := os.Stat(dmg); os.IsNotExist(err) {
		if err := downloadDeveloperDiskImage(imageInfo, imageVersionDir); err != nil {
			log.Err(err).Msg("Download Developer disk image error: ")
			return err
		}
	}

	// 开始执行挂载
	cmd := exec.CommandContext(ctx, "ideviceimagemounter", "-u", device.UDID, "-n", dmg, signature)
	data, err := cmd.CombinedOutput()
	if err != nil {
		log.Err(err).Msg("Run ideviceimagemounter error: ")
		return fmt.Errorf("%s%s", string(data), err.Error())
	}

	output := string(data)
	if strings.Contains(output, "ERROR") {
		return fmt.Errorf("%s\n%s", "Run ideviceimagemounter error: ", output)
	}

	return nil
}

func downloadDeveloperDiskImage(imageInfo *model.UsbmuxdImage, imageVersionDir string) error {
	tmpPath := filepath.Join(cfg.Server.WorkDir, "tmp", "DeveloperDiskImage.zip")
	tmpUnzipPath := filepath.Join(cfg.Server.WorkDir, "tmp", "DeveloperDiskImage")
	_ = os.RemoveAll(tmpUnzipPath)

	resp, err := http.NewClient().R().SetOutput(tmpPath).Get(imageInfo.DeveloperDiskImageUrl)
	if err == nil && resp.IsSuccess() {
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

	// 镜像找不到，尝试降级获取上一版本（有时可以使用前一版本的镜像进行mount）
	if resp.StatusCode() == 404 {
		log.Warnf("try downgrade Developer disk image to version: %s", imageInfo.DowngradeDeveloperDiskImageVersion)
		resp, err = http.NewClient().R().SetOutput(tmpPath).Get(imageInfo.DowngradeDeveloperDiskImageUrl)
		if err == nil && resp.IsSuccess() {
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
	}

	if err != nil {
		log.Err(err).Msg("Download Developer disk image error: ")
		return err
	}
	if !resp.IsSuccess() {
		return fmt.Errorf("Developer disk image could not found. os: tvOS %s url: %s status: %d", imageInfo.Device.ProductVersion, imageInfo.DeveloperDiskImageUrl, resp.StatusCode())
	}

	return nil
}

func GetValidName(name string) string {
	return strings.ToLower(regValidName.ReplaceAllString(name, ""))
}

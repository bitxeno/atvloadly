package service

import (
	"testing"
)

func TestDownloadDeveloperDiskImageByVersion(t *testing.T) {
	err := downloadDeveloperDiskImageByVersion("https://github.com/haikieu/xcode-developer-disk-image-all-platforms/raw/master/DiskImages/AppleTVOS.platform/DeviceSupport/16.4.zip", "16.4")
	if err != nil {
		t.Error(err)
	}
}

package manager

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
	plist "howett.net/plist"
)

func loadLockdownDevices() (map[string]model.LockdownDevice, error) {
	files, err := os.ReadDir(app.LockdownDir())
	if err != nil {
		if !os.IsNotExist(err) {
			log.Debugf("Read lockdown dir error: %v", err)
		}
		devices := map[string]model.LockdownDevice{}
		return devices, nil
	}

	devices := map[string]model.LockdownDevice{}
	for _, file := range files {
		if file.IsDir() || strings.HasPrefix(file.Name(), ".") || file.Name() == "SystemConfiguration.plist" {
			continue
		}

		buf, err := os.ReadFile(filepath.Join(app.LockdownDir(), file.Name()))
		if err != nil {
			return nil, err
		}
		var lockdownDevice model.LockdownDevice
		decoder := plist.NewDecoder(bytes.NewReader(buf))
		if err = decoder.Decode(&lockdownDevice); err != nil {
			return nil, err
		}

		lockdownDevice.Name = utils.FileNameWithoutExt(file.Name())
		macAddr := strings.ToLower(lockdownDevice.WiFiMACAddress)
		devices[macAddr] = lockdownDevice
	}
	return devices, nil
}

func removeLockdownDevice(udid string) {
	lockdownFile := filepath.Join(app.LockdownDir(), udid+".plist")
	if _, err := os.Stat(lockdownFile); os.IsNotExist(err) {
		return
	}

	_ = os.Remove(lockdownFile)
}

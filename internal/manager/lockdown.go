package manager

import (
	"bytes"
	"fmt"
	"os"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
	"howett.net/plist"
)

func loadLockdownDevices() (map[string]model.LockdownDevice, error) {
	files, err := os.ReadDir(app.Config.App.LockdownDir)
	if err != nil {
		fmt.Println(err)
		// return nil, err

		devices := map[string]model.LockdownDevice{}
		return devices, nil
	}

	devices := map[string]model.LockdownDevice{}
	for _, file := range files {
		if file.IsDir() || file.Name() == "SystemConfiguration.plist" {
			continue
		}

		buf, err := os.ReadFile(fmt.Sprintf("%s/%s", app.Config.App.LockdownDir, file.Name()))
		if err != nil {
			return nil, err
		}
		var lockdownDevice model.LockdownDevice
		decoder := plist.NewDecoder(bytes.NewReader(buf))
		if err = decoder.Decode(&lockdownDevice); err != nil {
			return nil, err
		}

		lockdownDevice.Name = utils.FileNameWithoutExt(file.Name())
		devices[lockdownDevice.WiFiMACAddress] = lockdownDevice
	}
	return devices, nil
}

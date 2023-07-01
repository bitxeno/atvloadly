package manager

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/bitxeno/atvloadly/config"
	"github.com/bitxeno/atvloadly/internal/utils"
	"github.com/bitxeno/atvloadly/model"
	"howett.net/plist"
)

func loadLockdownDevices() (map[string]model.LockdownDevice, error) {
	files, err := ioutil.ReadDir(config.App.LockdownDir)
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

		buf, err := os.ReadFile(fmt.Sprintf("%s/%s", config.App.LockdownDir, file.Name()))
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

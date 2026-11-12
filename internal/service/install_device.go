package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bitxeno/atvloadly/internal/model"
)

// ValidateInstallDeviceType accepts an empty value or a supported device class.
func ValidateInstallDeviceType(deviceType string) error {
	if deviceType == "" {
		return nil
	}

	switch model.DeviceClass(deviceType) {
	case model.DeviceClassAppleTV, model.DeviceClassiPhone, model.DeviceClassiPad:
		return nil
	default:
		return fmt.Errorf("invalid device_type: %s; valid values are: %s, %s, %s", deviceType, model.DeviceClassAppleTV, model.DeviceClassiPhone, model.DeviceClassiPad)
	}
}

// SelectInstallDevice picks the install target: the device with deviceID, or
// the only device matching deviceType when deviceID is empty. Several matching
// devices without a deviceID is refused rather than guessed.
func SelectInstallDevice(devices []model.Device, deviceID, deviceType string) (model.Device, error) {
	if deviceType != "" {
		filtered := make([]model.Device, 0, len(devices))
		for _, device := range devices {
			if device.DeviceClass == deviceType {
				filtered = append(filtered, device)
			}
		}
		devices = filtered
	}

	if len(devices) == 0 {
		if deviceType != "" {
			return model.Device{}, fmt.Errorf("no available devices found for device_type: %s", deviceType)
		}
		return model.Device{}, fmt.Errorf("no available devices found")
	}
	if deviceID != "" {
		for _, device := range devices {
			if device.ID == deviceID {
				return device, nil
			}
		}
		return model.Device{}, fmt.Errorf("device_id not found: %s", deviceID)
	}
	if len(devices) == 1 {
		return devices[0], nil
	}

	choices := make([]string, 0, len(devices))
	for _, device := range devices {
		choices = append(choices, fmt.Sprintf("%s (%s, %s)", device.ID, device.Name, device.DeviceClass))
	}
	sort.Strings(choices)
	return model.Device{}, fmt.Errorf("several devices are available, device_id is required: %s", strings.Join(choices, ", "))
}

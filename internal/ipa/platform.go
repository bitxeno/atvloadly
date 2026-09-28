package ipa

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bitxeno/atvloadly/internal/model"
)

// ErrPlatformMismatch reports an IPA built for another platform than the device.
var ErrPlatformMismatch = errors.New("ipa platform does not match the device")

// CheckPlatform verifies that an IPA supporting platforms
// (CFBundleSupportedPlatforms) can run on a device of deviceClass. It returns
// nil when this cannot be verified.
func CheckPlatform(platforms []string, deviceClass string) error {
	var want string
	switch model.DeviceClass(deviceClass) {
	case model.DeviceClassAppleTV:
		want = "AppleTVOS"
	case model.DeviceClassiPhone, model.DeviceClassiPad:
		want = "iPhoneOS"
	}
	if want == "" || len(platforms) == 0 {
		return nil
	}

	for _, p := range platforms {
		if strings.EqualFold(p, want) {
			return nil
		}
	}
	return fmt.Errorf("%w: IPA is built for %s, device is %s", ErrPlatformMismatch, strings.Join(platforms, ", "), deviceClass)
}

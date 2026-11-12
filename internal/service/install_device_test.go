package service

import (
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/model"
)

func TestValidateInstallDeviceType(t *testing.T) {
	tests := []struct {
		deviceType string
		wantErr    string
	}{
		{deviceType: ""},
		{deviceType: string(model.DeviceClassAppleTV)},
		{deviceType: string(model.DeviceClassiPhone)},
		{deviceType: string(model.DeviceClassiPad)},
		{deviceType: "unknown", wantErr: "invalid device_type: unknown; valid values are: AppleTV, iPhone, iPad"},
		{deviceType: "iphone", wantErr: "invalid device_type: iphone; valid values are: AppleTV, iPhone, iPad"},
	}

	for _, tt := range tests {
		t.Run(tt.deviceType, func(t *testing.T) {
			err := ValidateInstallDeviceType(tt.deviceType)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateInstallDeviceType(%q) error = %v", tt.deviceType, err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateInstallDeviceType(%q) error = %v, want %q", tt.deviceType, err, tt.wantErr)
			}
		})
	}
}

func TestSelectInstallDevice(t *testing.T) {
	atv := model.Device{ID: "atv", Name: "Living Room", DeviceClass: string(model.DeviceClassAppleTV)}
	iphone := model.Device{ID: "phone", Name: "My iPhone", DeviceClass: string(model.DeviceClassiPhone)}
	ipad := model.Device{ID: "pad", Name: "My iPad", DeviceClass: string(model.DeviceClassiPad)}

	tests := []struct {
		name       string
		devices    []model.Device
		deviceID   string
		deviceType string
		wantID     string
		wantErr    string
	}{
		{name: "only Apple TV", devices: []model.Device{atv}, wantID: "atv"},
		{name: "only iPhone", devices: []model.Device{iphone}, wantID: "phone"},
		{name: "only iPad", devices: []model.Device{ipad}, wantID: "pad"},
		{name: "explicit iPhone among several", devices: []model.Device{atv, iphone, ipad}, deviceID: "phone", wantID: "phone"},
		{name: "explicit Apple TV among several", devices: []model.Device{iphone, atv}, deviceID: "atv", wantID: "atv"},
		{name: "device type selects matching device", devices: []model.Device{atv, iphone}, deviceType: string(model.DeviceClassAppleTV), wantID: "atv"},
		{name: "device type still requires ID when several match", devices: []model.Device{iphone, {ID: "phone2", Name: "Work iPhone", DeviceClass: string(model.DeviceClassiPhone)}, atv}, deviceType: string(model.DeviceClassiPhone), wantErr: "device_id is required: phone (My iPhone, iPhone), phone2 (Work iPhone, iPhone)"},
		{name: "device ID must match device type", devices: []model.Device{atv, iphone}, deviceID: "phone", deviceType: string(model.DeviceClassAppleTV), wantErr: "device_id not found: phone"},
		{name: "several without device_id", devices: []model.Device{iphone, atv}, wantErr: "device_id is required: atv (Living Room, AppleTV), phone (My iPhone, iPhone)"},
		{name: "unknown device_id", devices: []model.Device{atv}, deviceID: "missing", wantErr: "device_id not found: missing"},
		{name: "no devices", wantErr: "no available devices found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectInstallDevice(tt.devices, tt.deviceID, tt.deviceType)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != tt.wantID {
				t.Errorf("device = %q, want %q", got.ID, tt.wantID)
			}
		})
	}
}

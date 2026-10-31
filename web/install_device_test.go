package web

import (
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/model"
)

func TestSelectInstallDevice(t *testing.T) {
	atv := model.Device{ID: "atv", Name: "Living Room", DeviceClass: string(model.DeviceClassAppleTV)}
	iphone := model.Device{ID: "phone", Name: "My iPhone", DeviceClass: string(model.DeviceClassiPhone)}
	ipad := model.Device{ID: "pad", Name: "My iPad", DeviceClass: string(model.DeviceClassiPad)}

	tests := []struct {
		name     string
		devices  []model.Device
		deviceID string
		wantID   string
		wantErr  string
	}{
		{name: "only Apple TV", devices: []model.Device{atv}, wantID: "atv"},
		{name: "only iPhone", devices: []model.Device{iphone}, wantID: "phone"},
		{name: "only iPad", devices: []model.Device{ipad}, wantID: "pad"},
		{name: "explicit iPhone among several", devices: []model.Device{atv, iphone, ipad}, deviceID: "phone", wantID: "phone"},
		{name: "explicit Apple TV among several", devices: []model.Device{iphone, atv}, deviceID: "atv", wantID: "atv"},
		{name: "several without device_id", devices: []model.Device{iphone, atv}, wantErr: "device_id is required: atv (Living Room, AppleTV), phone (My iPhone, iPhone)"},
		{name: "unknown device_id", devices: []model.Device{atv}, deviceID: "missing", wantErr: "device_id not found: missing"},
		{name: "no devices", wantErr: "no available devices found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectInstallDevice(tt.devices, tt.deviceID)
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

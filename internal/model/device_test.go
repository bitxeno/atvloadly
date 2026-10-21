package model

import "testing"

func TestParseDeviceClass(t *testing.T) {
	cases := []struct {
		name   string
		device Device
		want   DeviceClass
	}{
		{
			name:   "product type overrides the add-flow placeholder name",
			device: Device{Name: "Manual device (100.98.236.43)", ProductType: "iPhone12,1", DeviceClass: string(DeviceClassAppleTV)},
			want:   DeviceClassiPhone,
		},
		{
			name:   "iPhone product type",
			device: Device{Name: "Torre's iPhone", ProductType: "iPhone12,1"},
			want:   DeviceClassiPhone,
		},
		{
			name:   "iPad product type",
			device: Device{Name: "irrelevant", ProductType: "iPad16,1"},
			want:   DeviceClassiPad,
		},
		{
			name:   "Apple TV product type",
			device: Device{Name: "irrelevant", ProductType: "AppleTV11,1"},
			want:   DeviceClassAppleTV,
		},
		{
			name:   "name suffix fallback for the iPhone",
			device: Device{Name: "Torre's iPhone"},
			want:   DeviceClassiPhone,
		},
		{
			name:   "name suffix fallback for the iPad",
			device: Device{Name: "kitchen iPad"},
			want:   DeviceClassiPad,
		},
		{
			name:   "unknown device defaults to the Apple TV",
			device: Device{Name: "Manual device (100.98.236.43)"},
			want:   DeviceClassAppleTV,
		},
	}

	for _, c := range cases {
		got := c.device
		got.ParseDeviceClass()
		if got.DeviceClass != string(c.want) {
			t.Errorf("%s: DeviceClass = %q, want %q", c.name, got.DeviceClass, c.want)
		}
	}
}

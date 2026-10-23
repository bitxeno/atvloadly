package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bitxeno/atvloadly/internal/model"
)

func writeJSON(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadManualDevicesFromDir(t *testing.T) {
	dir := t.TempDir()
	// A manual device (pairing file has manual + remotepairing_ip).
	writeJSON(t, dir, "AAA.json", `{"manual":true,"model":"iPhone14,4","name":"Phone","remotepairing_ip":"100.65.100.37","remotepairing_udid":"AAA"}`)
	// An mDNS-discovered device's record (no manual / no ip): must be skipped.
	writeJSON(t, dir, "BBB.json", `{"model":"AppleTV11,1","name":"AppleTV","remotepairing_udid":"BBB"}`)

	dm := newDeviceManager()
	dm.loadManualDevicesFromDir(dir)

	devices := dm.GetDevices()
	if len(devices) != 1 {
		t.Fatalf("want 1 manual device, got %d: %+v", len(devices), devices)
	}
	d := devices[0]
	if !d.Manual || d.IP != "100.65.100.37" || d.UDID != "AAA" || d.Name != "Phone" {
		t.Fatalf("unexpected manual device: %+v", d)
	}
	if d.DeviceClass != string(model.DeviceClassiPhone) {
		t.Fatalf("device class = %q", d.DeviceClass)
	}
}

func TestManualDeviceDedupedByDiscovered(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "AAA.json", `{"manual":true,"model":"iPhone14,4","name":"Phone","remotepairing_ip":"100.65.100.37","remotepairing_udid":"AAA"}`)

	dm := newDeviceManager()
	dm.loadManualDevicesFromDir(dir)
	// Simulate mDNS discovering the same UDID with a fresher IP.
	dm.devices.Store("AAA", model.Device{ID: "AAA", UDID: "AAA", Name: "Phone", IP: "192.168.1.9", Status: model.Paired})

	devices := dm.GetDevices()
	if len(devices) != 1 {
		t.Fatalf("want deduped to 1 device, got %d", len(devices))
	}
	if devices[0].IP != "192.168.1.9" {
		t.Fatalf("discovered device must win, got IP %q", devices[0].IP)
	}
}

func TestDeleteManualDeviceRemovesBothFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	pdir := filepath.Join(dir, ".config", "PlumeImpactor", "pairing_files")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, pdir, "AAA.json", `{"manual":true,"remotepairing_ip":"100.65.100.37","remotepairing_udid":"AAA"}`)
	writeJSON(t, pdir, "AAA.plist", "plist")

	if err := DeleteManualDevice("AAA"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	for _, ext := range []string{".json", ".plist"} {
		if _, err := os.Stat(filepath.Join(pdir, "AAA"+ext)); !os.IsNotExist(err) {
			t.Fatalf("%s should be deleted", ext)
		}
	}
}

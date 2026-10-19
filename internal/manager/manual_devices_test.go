package manager

import (
	"errors"
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/model"
)

// setupManualDeviceTest isolates the persistence file and the plumesign
// probe for one test, returning the fake probe result pointer used by
// fakeProbe.
func setupManualDeviceTest(t *testing.T, fakeProbe *model.DeviceInfo, fakeErr error) {
	t.Helper()

	app.Config.Server.DataDir = t.TempDir()

	originalProbe := probeDeviceInfo
	probeDeviceInfo = func(ip string, port uint16) (*model.DeviceInfo, error) {
		return fakeProbe, fakeErr
	}
	t.Cleanup(func() { probeDeviceInfo = originalProbe })
}

func completeDeviceInfo() *model.DeviceInfo {
	return &model.DeviceInfo{
		UniqueDeviceID: "S1**A01**ABC12",
		DeviceName:     "Living Room",
		DeviceClass:    "AppleTV",
		ProductType:    "AppleTV5,3",
		ProductVersion: "18.1",
		WiFiAddress:    "A1:B2:C3:D4:E5:F6",
	}
}

func TestAddManualDeviceRegistersPairableDevice(t *testing.T) {
	setupManualDeviceTest(t, completeDeviceInfo(), nil)

	dm := newDeviceManager()
	dev, err := dm.AddManualDevice("100.64.0.12", 62078, "")
	if err != nil {
		t.Fatalf("AddManualDevice failed: %v", err)
	}

	if dev.UDID != "S1**A01**ABC12" {
		t.Errorf("UDID = %q, want S1**A01**ABC12", dev.UDID)
	}
	if dev.Name != "Living Room" {
		t.Errorf("Name = %q, want the probed device name", dev.Name)
	}
	if dev.Status != model.Pairable {
		t.Errorf("Status = %q, want Pairable", dev.Status)
	}
	if dev.Connection != model.DeviceConnectionRemote {
		t.Errorf("Connection = %q, want RPPairing", dev.Connection)
	}
	if dev.DeviceClass != string(model.DeviceClassAppleTV) {
		t.Errorf("DeviceClass = %q, want AppleTV", dev.DeviceClass)
	}
	if _, ok := dm.GetDeviceByID(dev.ID); !ok {
		t.Error("device was not registered in the manager")
	}
}

func TestAddManualDeviceUsesSuppliedName(t *testing.T) {
	setupManualDeviceTest(t, completeDeviceInfo(), nil)

	dm := newDeviceManager()
	dev, err := dm.AddManualDevice("100.64.0.12", 62078, "My Apple TV")
	if err != nil {
		t.Fatalf("AddManualDevice failed: %v", err)
	}
	if dev.Name != "My Apple TV" {
		t.Errorf("Name = %q, want the supplied name", dev.Name)
	}
}

func TestAddManualDeviceFallsBackName(t *testing.T) {
	info := completeDeviceInfo()
	info.DeviceName = ""
	setupManualDeviceTest(t, info, nil)

	dm := newDeviceManager()
	dev, err := dm.AddManualDevice("100.64.0.12", 62078, "")
	if err != nil {
		t.Fatalf("AddManualDevice failed: %v", err)
	}
	if dev.Name != "Manual device (100.64.0.12)" {
		t.Errorf("Name = %q, want the fallback name", dev.Name)
	}
}

func TestAddManualDeviceRejectsUnreachableDevice(t *testing.T) {
	setupManualDeviceTest(t, nil, errFakeProbeFailed)

	dm := newDeviceManager()
	_, err := dm.AddManualDevice("100.64.0.12", 62078, "")
	if err == nil {
		t.Fatal("unreachable device should be rejected")
	}
}

func TestAddManualDeviceRejectsBadInput(t *testing.T) {
	dm := newDeviceManager()

	// Public address.
	if _, err := dm.AddManualDevice("8.8.8.8", 62078, ""); err == nil {
		t.Error("public address should be rejected")
	}
	// Port 0.
	if _, err := dm.AddManualDevice("100.64.0.12", 0, ""); err == nil {
		t.Error("port 0 should be rejected")
	}

	// Nothing must be registered after the rejections.
	if n := len(dm.GetDevices()); n != 0 {
		t.Errorf("devices registered = %d, want 0", n)
	}
}

func TestManualDevicesPersistAcrossRestart(t *testing.T) {
	setupManualDeviceTest(t, completeDeviceInfo(), nil)

	dm := newDeviceManager()
	dev, err := dm.AddManualDevice("100.64.0.12", 62078, "")
	if err != nil {
		t.Fatalf("AddManualDevice failed: %v", err)
	}

	// A "restart" loads the file into a fresh manager.
	fresh := newDeviceManager()
	fresh.LoadManualDevices()
	restored, ok := fresh.GetDeviceByID(dev.ID)
	if !ok {
		t.Fatal("manual device was not restored on startup")
	}
	if restored.UDID != dev.UDID || restored.Status != model.Pairable {
		t.Errorf("restored device mismatch: %+v", restored)
	}
}

func TestAddManualDeviceReplacesStaleEntry(t *testing.T) {
	setupManualDeviceTest(t, completeDeviceInfo(), nil)

	dm := newDeviceManager()
	if _, err := dm.AddManualDevice("100.64.0.12", 62078, ""); err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	// Same device moves to a new address: only one entry may remain.
	replaced, err := dm.AddManualDevice("100.113.5.5", 62078, "")
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}
	if replaced.IP != "100.113.5.5" {
		t.Errorf("IP = %q, want the new address", replaced.IP)
	}

	entries := readManualDevices()
	if len(entries) != 1 {
		t.Fatalf("manual entries = %d, want 1", len(entries))
	}
	if entries[0].IP != "100.113.5.5" {
		t.Errorf("persisted IP = %q, want the new address", entries[0].IP)
	}
}

func TestDeleteManualDevice(t *testing.T) {
	setupManualDeviceTest(t, completeDeviceInfo(), nil)

	dm := newDeviceManager()
	dev, err := dm.AddManualDevice("100.64.0.12", 62078, "")
	if err != nil {
		t.Fatalf("AddManualDevice failed: %v", err)
	}

	found, err := dm.DeleteManualDevice("no-such-id")
	if err != nil {
		t.Fatalf("DeleteManualDevice failed: %v", err)
	}
	if found {
		t.Error("unknown device should not be reported found")
	}

	found, err = dm.DeleteManualDevice(dev.ID)
	if err != nil {
		t.Fatalf("DeleteManualDevice failed: %v", err)
	}
	if !found {
		t.Error("registered device should be reported found")
	}
	if _, ok := dm.GetDeviceByID(dev.ID); ok {
		t.Error("device was not removed from the manager")
	}
	if n := len(readManualDevices()); n != 0 {
		t.Errorf("manual entries after delete = %d, want 0", n)
	}
}

func TestPairCompletedUpgradesManualDevice(t *testing.T) {
	setupManualDeviceTest(t, completeDeviceInfo(), nil)

	dm := newDeviceManager()
	dev, err := dm.AddManualDevice("100.64.0.12", 62078, "")
	if err != nil {
		t.Fatalf("AddManualDevice failed: %v", err)
	}
	if dev.Status != model.Pairable {
		t.Fatalf("device should start Pairable, got %q", dev.Status)
	}

	dm.PairCompleted(dev.ID)

	paired, ok := dm.GetDeviceByID(dev.ID)
	if !ok {
		t.Fatal("device vanished after pairing")
	}
	if paired.Status != model.Paired {
		t.Errorf("Status = %q, want Paired", paired.Status)
	}

	// The persisted record must reflect the upgrade too.
	entries := readManualDevices()
	if len(entries) != 1 || entries[0].Status != model.Paired {
		t.Errorf("persisted status should be Paired, got %+v", entries)
	}
}

// TestParseDeviceInfoOutput covers the fallbackUDID handling in the
// extracted parser.
func TestParseDeviceInfoOutput(t *testing.T) {
	output := strings.Join([]string{
		"UniqueDeviceID: S1**A01**ABC12",
		"DeviceName: Living Room",
		"ProductVersion: 18.1",
		"PersonalizedImageMounted: true",
		"nonsense line",
	}, "\n")

	info := parseDeviceInfoOutput(output, "fallback")
	if info.UniqueDeviceID != "S1**A01**ABC12" {
		t.Errorf("UniqueDeviceID = %q", info.UniqueDeviceID)
	}
	if info.DeviceName != "Living Room" {
		t.Errorf("DeviceName = %q", info.DeviceName)
	}
	if info.ProductVersion != "18.1" {
		t.Errorf("ProductVersion = %q", info.ProductVersion)
	}
	if !info.PersonalizedImageMounted {
		t.Error("PersonalizedImageMounted = false, want true")
	}

	// Without a usable output the fallback UDID is kept.
	info = parseDeviceInfoOutput("", "fallback")
	if info.UniqueDeviceID != "fallback" {
		t.Errorf("UniqueDeviceID = %q, want fallback", info.UniqueDeviceID)
	}
}

var errFakeProbeFailed = errors.New("probe failed")

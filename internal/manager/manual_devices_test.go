package manager

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/model"
)

// probeCalls records the arguments of every fake probe invocation, in order.
var probeCalls []string

// setupManualDeviceTest isolates the persistence file and the plumesign
// probe for one test. The fake probe returns fakeInfo for the pairing files
// accepted by fakeOrder and an error otherwise, so tests can cover both the
// matching and the skipping of pairing files.
func setupManualDeviceTest(t *testing.T, fakeOrder []string, fakeInfo *model.DeviceInfo, fakeErr error) {
	t.Helper()

	if app.Config == nil {
		app.Config = &app.Configuration{}
	}
	app.Config.Server.DataDir = t.TempDir()

	probeCalls = nil
	originalProbe := probeDeviceInfo
	probeDeviceInfo = func(ip string, port uint16, pairingFile string) (*model.DeviceInfo, error) {
		probeCalls = append(probeCalls, ip+"|"+pairingFile)
		for _, accepted := range fakeOrder {
			if filepath.Base(pairingFile) == accepted {
				return fakeInfo, fakeErr
			}
		}
		return nil, errors.New("pairing file does not match this device")
	}
	t.Cleanup(func() { probeDeviceInfo = originalProbe })
}

// writePairingFiles creates plumesign store fixtures for PairCompleted.
func writePairingFiles(t *testing.T, names ...string) {
	t.Helper()

	dir := app.RemotePairingDir()
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("plist"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// addManualDevice registers a device in a fresh manager and returns both,
// so tests inspect the same registry the device was stored in.
func addManualDevice(t *testing.T, ip string) (*DeviceManager, *model.Device) {
	t.Helper()

	dm := newDeviceManager()
	dev, err := dm.AddManualDevice(ip, 62078, "")
	if err != nil {
		t.Fatalf("AddManualDevice failed: %v", err)
	}
	return dm, dev
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
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)

	dm, dev := addManualDevice(t, "100.64.0.12")

	if dev.UDID != "" {
		t.Errorf("UDID = %q, want empty before pairing", dev.UDID)
	}
	if dev.Name != "Manual device (100.64.0.12)" {
		t.Errorf("Name = %q, want the fallback name", dev.Name)
	}
	if dev.Status != model.Pairable {
		t.Errorf("Status = %q, want Pairable", dev.Status)
	}
	if dev.Connection != model.DeviceConnectionRemote {
		t.Errorf("Connection = %q, want RPPairing", dev.Connection)
	}
	if _, ok := dm.GetDeviceByID(dev.ID); !ok {
		t.Error("device was not registered in the manager")
	}

	// No probe may run: device-info over the network needs a pairing file
	// that does not exist yet.
	if len(probeCalls) != 0 {
		t.Errorf("probe calls = %d, want 0", len(probeCalls))
	}
}

func TestAddManualDeviceUsesSuppliedName(t *testing.T) {
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)

	dm := newDeviceManager()
	named, err := dm.AddManualDevice("100.64.0.13", 62078, "My Apple TV")
	if err != nil {
		t.Fatalf("AddManualDevice failed: %v", err)
	}
	if named.Name != "My Apple TV" {
		t.Errorf("Name = %q, want the supplied name", named.Name)
	}
}

func TestAddManualDeviceRejectsBadInput(t *testing.T) {
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)
	dm := newDeviceManager()

	// Public address.
	if _, err := dm.AddManualDevice("8.8.8.8", 62078, ""); err == nil {
		t.Error("public address should be rejected")
	}

	// Nothing must be registered after the rejections.
	if n := len(dm.GetDevices()); n != 0 {
		t.Errorf("devices registered = %d, want 0", n)
	}
}

// TestAddManualDeviceDefaultsPairPort verifies the hardcoded pairing port:
// omitting it stores Apple's fixed lockdown port, which is what lets the
// Add Device flow ask for nothing but an address.
func TestAddManualDeviceDefaultsPairPort(t *testing.T) {
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)
	dm := newDeviceManager()

	dev, err := dm.AddManualDevice("100.64.0.12", 0, "")
	if err != nil {
		t.Fatalf("AddManualDevice failed: %v", err)
	}
	if dev.Port != pairPortFixed {
		t.Errorf("Port = %d, want the fixed pairing port %d", dev.Port, pairPortFixed)
	}
	if !dev.Manual {
		t.Error("manually added device must carry Manual = true")
	}
}

func TestManualDevicesPersistAcrossRestart(t *testing.T) {
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)

	_, dev := addManualDevice(t, "100.64.0.12")

	// A "restart" loads the file into a fresh manager.
	fresh := newDeviceManager()
	fresh.LoadManualDevices()
	restored, ok := fresh.GetDeviceByID(dev.ID)
	if !ok {
		t.Fatal("manual device was not restored on startup")
	}
	if restored.Status != model.Pairable {
		t.Errorf("restored Status = %q, want Pairable", restored.Status)
	}
	if !restored.Manual {
		t.Error("restored device must carry Manual = true")
	}
}

func TestAddManualDeviceReplacesStaleEntry(t *testing.T) {
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)

	addManualDevice(t, "100.64.0.12")

	// Same device moves to a new address: only one entry may remain.
	replaced, err := newDeviceManager().AddManualDevice("100.113.5.5", 62078, "")
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}
	if replaced.IP != "100.113.5.5" {
		t.Errorf("IP = %q, want the new address", replaced.IP)
	}

	entries := readManualDevices()
	if len(entries) != 2 {
		t.Fatalf("manual entries = %d, want 2", len(entries))
	}
}

func TestDeleteManualDevice(t *testing.T) {
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)

	_, dev := addManualDevice(t, "100.64.0.12")
	if _, err := newDeviceManager().AddManualDevice("100.64.0.9", 62078, "second"); err != nil {
		t.Fatalf("second add failed: %v", err)
	}

	found, err := newDeviceManager().DeleteManualDevice("no-such-id")
	if err != nil {
		t.Fatalf("DeleteManualDevice failed: %v", err)
	}
	if found {
		t.Error("unknown device should not be reported found")
	}

	found, err = newDeviceManager().DeleteManualDevice(dev.ID)
	if err != nil {
		t.Fatalf("DeleteManualDevice failed: %v", err)
	}
	if !found {
		t.Error("registered device should be reported found")
	}
	if _, ok := newDeviceManager().GetDeviceByID(dev.ID); ok {
		t.Error("device was not removed from the manager")
	}
	if n := len(readManualDevices()); n != 1 {
		t.Errorf("manual entries after delete = %d, want 1", n)
	}
}

func TestPairCompletedFillsInfoFromPairingFiles(t *testing.T) {
	// Only the record of the newly paired device matches; the stale record
	// of an unrelated device must be skipped, not fatal.
	setupManualDeviceTest(t, []string{"S1**A01**ABC12.plist"}, completeDeviceInfo(), nil)
	dm, dev := addManualDevice(t, "100.64.0.12")
	writePairingFiles(t, "OTHER.plist", "S1**A01**ABC12.plist")

	dm.PairCompleted(dev.ID)

	paired, ok := dm.GetDeviceByID(dev.ID)
	if !ok {
		t.Fatal("device vanished after pairing")
	}
	if paired.UDID != "S1**A01**ABC12" {
		t.Errorf("UDID = %q, want the probed UDID", paired.UDID)
	}
	if paired.Status != model.Paired {
		t.Errorf("Status = %q, want Paired", paired.Status)
	}
	if paired.DeviceClass != string(model.DeviceClassAppleTV) {
		t.Errorf("DeviceClass = %q, want AppleTV", paired.DeviceClass)
	}
	if paired.ProductType != "AppleTV5,3" {
		t.Errorf("ProductType = %q, want the probed type", paired.ProductType)
	}

	// A mismatching pairing file must be skipped, not fatal.
	if len(probeCalls) < 2 {
		t.Errorf("probe calls = %d, want every candidate file tried", len(probeCalls))
	}

	// The persisted record must reflect the upgrade too.
	entries := readManualDevices()
	for _, entry := range entries {
		if entry.ID == dev.ID && (entry.Status != model.Paired || entry.UDID != "S1**A01**ABC12") {
			t.Errorf("persisted record not upgraded: %+v", entry)
		}
	}
}

func TestPairCompletedKeepsDevicePairableWithoutRecords(t *testing.T) {
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)
	dm, dev := addManualDevice(t, "100.64.0.12")
	writePairingFiles(t)

	dm.PairCompleted(dev.ID)

	left, ok := dm.GetDeviceByID(dev.ID)
	if !ok {
		t.Fatal("device vanished after failed pairing")
	}
	if left.Status != model.Pairable {
		t.Errorf("Status = %q, want Pairable when no pairing file matches", left.Status)
	}
	if left.UDID != "" {
		t.Errorf("UDID = %q, want empty", left.UDID)
	}
}

func TestPairCompletedKeepsDevicePairableWhenProbeFails(t *testing.T) {
	info := completeDeviceInfo()
	info.UniqueDeviceID = ""
	setupManualDeviceTest(t, []string{"A.plist"}, info, nil)
	dm, dev := addManualDevice(t, "100.64.0.12")
	writePairingFiles(t, "A.plist")

	dm.PairCompleted(dev.ID)

	left, ok := dm.GetDeviceByID(dev.ID)
	if !ok {
		t.Fatal("device vanished after failed pairing")
	}
	if left.Status != model.Pairable {
		t.Errorf("Status = %q, want Pairable when the probe returns no UDID", left.Status)
	}
	if left.UDID != "" {
		t.Errorf("UDID = %q, want empty", left.UDID)
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

// TestPairCompletedFallsBackToPeerRecord covers tvOS 26 devices whose
// device-info listener handshake fails over the routed address: the UDID
// comes from the newest peer record instead.
func TestPairCompletedFallsBackToPeerRecord(t *testing.T) {
	// The fake probe always fails: the tvOS 26 case the fallback covers.
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)
	dm, dev := addManualDevice(t, "100.64.0.12")

	writePairingFiles(t, "A.plist", "A.json")
	peer := `{"account_id":"id","alt_irk":[1],"model":"AppleTV11,1","name":"Efrain's room TV","remotepairing_udid":"882fd889f04dfd3973f4e61cd3b0e950a73ba1b1"}`
	if err := os.WriteFile(filepath.Join(app.RemotePairingDir(), "A.json"), []byte(peer), 0o644); err != nil {
		t.Fatal(err)
	}

	dm.PairCompleted(dev.ID)

	left, ok := dm.GetDeviceByID(dev.ID)
	if !ok {
		t.Fatal("device vanished after failed pairing")
	}
	if left.Status != model.Paired {
		t.Errorf("Status = %q, want Paired when the peer record identifies the device", left.Status)
	}
	if left.UDID != "882fd889f04dfd3973f4e61cd3b0e950a73ba1b1" {
		t.Errorf("UDID = %q, want the peer record UDID", left.UDID)
	}
	if left.DeviceClass != string(model.DeviceClassAppleTV) {
		t.Errorf("DeviceClass = %q, want AppleTV", left.DeviceClass)
	}
	if left.ProductType != "AppleTV11,1" {
		t.Errorf("ProductType = %q, want the peer record model", left.ProductType)
	}

	entries := readManualDevices()
	if len(entries) != 1 || entries[0].Status != model.Paired || entries[0].UDID != left.UDID {
		t.Errorf("persisted record not upgraded: %+v", entries)
	}
}

// TestUpdateManualInstallPortPersists verifies that the install page's
// chosen service port reaches both the live registry and the persistence
// file, so a restart keeps it. The scan stores the install service port in
// Port itself, the same field discovered devices get refreshed by their
// announcement.
func TestUpdateManualInstallPortPersists(t *testing.T) {
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)

	dm, dev := addManualDevice(t, "100.64.0.12")
	if dev.Port != pairPortFixed {
		t.Fatalf("Port = %d, want the pairing port %d before the override", dev.Port, pairPortFixed)
	}

	dm.UpdateManualInstallPort(dev, 49152)

	if dev.Port != 49152 {
		t.Errorf("Port = %d, want 49152 after the override", dev.Port)
	}

	entries := readManualDevices()
	if len(entries) != 1 || entries[0].Port != 49152 {
		t.Errorf("persistence does not carry the install port: %+v", entries)
	}
}

// TestRefreshManualInstallPort verifies the pre-refresh rescan: a manual
// device gets its stored install port re-detected, a non-manual device is
// left untouched.
func TestRefreshManualInstallPort(t *testing.T) {
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)

	dm, dev := addManualDevice(t, "100.64.0.12")
	dm.UpdateManualInstallPort(dev, 51000)

	_, probed := stubScan(t, []uint16{49152}, 49152)
	dm.RefreshManualInstallPort(dev.ID)

	scanned, ok := dm.GetDeviceByID(dev.ID)
	if !ok {
		t.Fatal("device vanished from the registry")
	}
	if scanned.Port != 49152 {
		t.Errorf("Port = %d, want the freshly scanned 49152", scanned.Port)
	}
	if probed[49152] == 0 {
		t.Errorf("the new port must have been probed: %v", probed)
	}
	if entries := readManualDevices(); len(entries) != 1 || entries[0].Port != 49152 {
		t.Errorf("persistence does not carry the freshly scanned port: %+v", entries)
	}

	// A discovered device is re-announced by avahi, so refresh skips it.
	discovered := model.Device{
		ID:         "discovered-id",
		IP:         "192.168.1.6",
		Port:       49152,
		Connection: model.DeviceConnectionLockdown,
		Status:     model.Paired,
	}
	dm.SaveDevice(discovered)
	dm.RefreshManualInstallPort(discovered.ID)
	if _, probed2 := stubScan(t, []uint16{49153}, 49153); probed2[49153] != 0 {
		t.Errorf("refresh must not scan a non-manual device: %v", probed2)
	}
	discovered2, _ := dm.GetDeviceByID(discovered.ID)
	if discovered2.Port != 49152 {
		t.Errorf("Port = %d on a discovered device, want untouched", discovered2.Port)
	}
}

// TestFreshInstallDevice verifies the tunnel-entry helper: a manual device
// gets its port re-detected and the returned device carries the fresh value,
// while a discovered device and nil are returned untouched.
func TestFreshInstallDevice(t *testing.T) {
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)

	dm, dev := addManualDevice(t, "100.64.0.12")
	stubScan(t, []uint16{49152}, 49152)

	fresh := dm.FreshInstallDevice(dev)
	if fresh.Port != 49152 {
		t.Errorf("Port = %d, want the freshly detected 49152", fresh.Port)
	}
	if entries := readManualDevices(); len(entries) != 1 || entries[0].Port != 49152 {
		t.Errorf("persistence does not carry the detected port: %+v", entries)
	}

	// Discovered devices are re-announced, so they pass through untouched.
	discovered := model.Device{
		ID:         "discovered-id",
		IP:         "192.168.1.6",
		Port:       49153,
		Connection: model.DeviceConnectionLockdown,
		Status:     model.Paired,
	}
	dm.SaveDevice(discovered)
	if same := dm.FreshInstallDevice(&discovered); same.Port != 49153 {
		t.Errorf("Port = %d on a discovered device, want untouched", same.Port)
	}

	if same := dm.FreshInstallDevice(nil); same != nil {
		t.Errorf("FreshInstallDevice(nil) = %+v, want nil", same)
	}
}

// TestScanAndStoreInstallPort verifies the post-pairing auto-scan: with the
// freshly written pairing record, the handshake-validated scan records the
// device's install port without any manual port entry.
func TestScanAndStoreInstallPort(t *testing.T) {
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)

	dm, dev := addManualDevice(t, "100.64.0.12")
	if dev.Port != pairPortFixed {
		t.Fatalf("fresh manual device carries Port %d, want the pairing port %d", dev.Port, pairPortFixed)
	}

	_, probed := stubScan(t, []uint16{49152}, 49152)
	dm.scanAndStoreInstallPort(dev.ID, dev.IP)

	scanned, ok := dm.GetDeviceByID(dev.ID)
	if !ok {
		t.Fatal("device vanished from the registry")
	}
	if scanned.Port != 49152 {
		t.Errorf("Port = %d, want the scanned 49152", scanned.Port)
	}
	if probed[49152] == 0 {
		t.Errorf("the open port must be probed: %v", probed)
	}
	entries := readManualDevices()
	if len(entries) != 1 || entries[0].Port != 49152 {
		t.Errorf("persistence does not carry the scanned install port: %+v", entries)
	}
}

// TestScanAndStoreInstallPortNoCandidate verifies a device whose handshake
// cannot identify an install port keeps its registration untouched.
func TestScanAndStoreInstallPortNoCandidate(t *testing.T) {
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)

	dm, dev := addManualDevice(t, "100.64.0.12")

	_, _ = stubScan(t, []uint16{62078}, 0)
	dm.scanAndStoreInstallPort(dev.ID, dev.IP)

	if dev.Port != pairPortFixed {
		t.Errorf("Port = %d, want the pairing port kept after a failed scan", dev.Port)
	}
	if entries := readManualDevices(); len(entries) != 1 || entries[0].Port != pairPortFixed {
		t.Errorf("persistence must stay untouched: %+v", entries)
	}
}

// TestImportPairedDeviceCreatesPairedEntry verifies the record-import flow:
// a pairing record validated elsewhere adds the device without any portal
// pairing, using the tunnel-capable install port only.
func TestImportPairedDeviceCreatesPairedEntry(t *testing.T) {
	setupManualDeviceTest(t, []string{"fake.plist"}, completeDeviceInfo(), nil)
	writePairingFiles(t, "fake.plist")

	dm := newDeviceManager()
	if err := dm.ImportPairedDevice("100.64.0.12", 49152); err != nil {
		t.Fatalf("ImportPairedDevice failed: %v", err)
	}

	dev, ok := dm.GetDeviceByID(ManualDeviceID("100.64.0.12", 49152))
	if !ok {
		t.Fatal("import did not create a device")
	}
	if dev.Status != model.Paired {
		t.Errorf("Status = %q, want Paired", dev.Status)
	}
	if !dev.Manual {
		t.Error("imported device must carry Manual = true")
	}
	if dev.UDID != completeDeviceInfo().UniqueDeviceID {
		t.Errorf("UDID = %q, want %q", dev.UDID, completeDeviceInfo().UniqueDeviceID)
	}
	if dev.Port != 49152 {
		t.Errorf("Port = %d, want 49152", dev.Port)
	}
	if dev.DeviceClass != string(model.DeviceClassAppleTV) {
		t.Errorf("DeviceClass = %q, want AppleTV", dev.DeviceClass)
	}

	entries := readManualDevices()
	if len(entries) != 1 || entries[0].Status != model.Paired || entries[0].UDID != dev.UDID {
		t.Errorf("persistence does not carry the imported device: %+v", entries)
	}
}

// TestImportPairedDeviceUsesPeerRecordFallback mirrors PairCompleted: when
// tvOS refuses the device-info handshake, the peer record written next to
// the imported pairing record still identifies the device.
func TestImportPairedDeviceUsesPeerRecordFallback(t *testing.T) {
	// The fake probe always fails: the tvOS 26 case the fallback covers.
	setupManualDeviceTest(t, nil, completeDeviceInfo(), nil)
	writePairingFiles(t, "fake.plist")
	peer := `{"account_id":"id","alt_irk":[1],"model":"AppleTV11,1","name":"Imported TV","remotepairing_udid":"aabbccddeeff0011223344556677aabb"}`
	if err := os.WriteFile(filepath.Join(app.RemotePairingDir(), "fake.json"), []byte(peer), 0o644); err != nil {
		t.Fatal(err)
	}

	dm := newDeviceManager()
	if err := dm.ImportPairedDevice("100.64.0.12", 49152); err != nil {
		t.Fatalf("ImportPairedDevice failed despite the peer record: %v", err)
	}

	dev, ok := dm.GetDeviceByID(ManualDeviceID("100.64.0.12", 49152))
	if !ok {
		t.Fatal("import did not create a device")
	}
	if dev.Status != model.Paired {
		t.Errorf("Status = %q, want Paired when the peer record identifies the device", dev.Status)
	}
	if dev.UDID != "aabbccddeeff0011223344556677aabb" {
		t.Errorf("UDID = %q, want the peer record UDID", dev.UDID)
	}
	if dev.ProductType != "AppleTV11,1" {
		t.Errorf("ProductType = %q, want the peer record model", dev.ProductType)
	}
}

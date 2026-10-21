package manager

import (
	"testing"

	"github.com/bitxeno/atvloadly/internal/model"
)

// A saved pairing record must not make an arbitrary open TCP port appear
// validated when every tunnel handshake fails.
func TestScanInstallPortRejectsUnverifiedCandidate(t *testing.T) {
	_, probed := stubScan(t, []uint16{49152, 51000}, 0)
	pairingRecordPaths = func() []string { return []string{"existing-device.plist"} }

	port, open, err := ScanInstallPort("100.99.166.81")
	if err == nil || port != 0 {
		t.Fatalf("expected no selected port and an error after failed handshakes; got port=%d, err=%v", port, err)
	}
	if len(open) != 2 {
		t.Fatalf("expected open ports for diagnostics, got %v", open)
	}
	if probed[49152] == 0 || probed[51000] == 0 {
		t.Fatalf("expected both candidates to be tested, got %v", probed)
	}
}

// The attempt limit must count failed probes, not successful matches.
func TestScanInstallPortCapsHandshakeValidationAttempts(t *testing.T) {
	ports := make([]uint16, 0, maxValidatedPorts+5)
	for i := 0; i < maxValidatedPorts+5; i++ {
		ports = append(ports, scanPortFirst+uint16(i))
	}

	_, probed := stubScan(t, ports, 0)
	pairingRecordPaths = func() []string { return []string{"existing-device.plist"} }
	_, _, _ = ScanInstallPort("100.99.166.81")

	attempts := 0
	for _, count := range probed {
		attempts += count
	}
	if attempts > maxValidatedPorts {
		t.Fatalf("expected at most %d validation attempts, got %d", maxValidatedPorts, attempts)
	}
}

// Registration identity must not depend on the changing install port.
func TestImportPairedDeviceUpgradesManualRegistration(t *testing.T) {
	setupManualDeviceTest(t, []string{"fake.plist"}, completeDeviceInfo(), nil)
	writePairingFiles(t, "fake.plist")

	dm, existing := addManualDevice(t, "100.64.0.12")
	if existing.Port != pairPortFixed {
		t.Fatalf("unexpected initial pairing port: %d", existing.Port)
	}

	if err := dm.ImportPairedDevice("100.64.0.12", 49152); err != nil {
		t.Fatalf("ImportPairedDevice failed: %v", err)
	}

	entries := readManualDevices()
	if len(entries) != 1 {
		t.Fatalf("import created duplicate registrations for one IP: %+v", entries)
	}

	upgraded, found := dm.GetDeviceByID(existing.ID)
	if !found || upgraded.Status != model.Paired || upgraded.Port != 49152 {
		t.Fatalf("original manual device was not upgraded: found=%v, device=%+v", found, upgraded)
	}
}

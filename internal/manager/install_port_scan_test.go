package manager

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// stubScan controls the network sweep and the tunnel handshake: ports listed
// in openPorts answer dials, and answeredProbe is the one port whose
// handshake succeeds. When answeredProbe is set, the device has a saved
// pairing record; otherwise scanning cannot validate any port. The returned
// maps count, per port, the dials attempted and the handshakes probed.
func stubScan(t *testing.T, openPorts []uint16, answeredProbe uint16) (map[uint16]int, map[uint16]int) {
	t.Helper()
	open := make(map[uint16]bool, len(openPorts))
	for _, port := range openPorts {
		open[port] = true
	}
	dialed := map[uint16]int{}
	probed := map[uint16]int{}
	var mu sync.Mutex
	prevDial := netDial
	prevProbe := probeInstallPort
	prevRecords := pairingRecordPaths
	netDial = func(ip string, port uint16, timeout time.Duration) error {
		mu.Lock()
		dialed[port]++
		mu.Unlock()
		if open[port] {
			return nil
		}
		return errors.New("refused")
	}
	probeInstallPort = func(ip string, port uint16, records []string) error {
		mu.Lock()
		probed[port]++
		mu.Unlock()
		if port == answeredProbe {
			return nil
		}
		return errors.New("no handshake")
	}
	pairingRecordPaths = func() []string {
		if answeredProbe != 0 {
			return []string{"record.plist"}
		}
		return nil
	}
	t.Cleanup(func() {
		netDial = prevDial
		probeInstallPort = prevProbe
		pairingRecordPaths = prevRecords
	})
	return dialed, probed
}

// A handshake-validated port wins over every other open port, and the fixed
// pairing port inside the swept range is never probed.
func TestScanInstallPortValidatesTunnelPort(t *testing.T) {
	_, probed := stubScan(t, []uint16{62078, 49152, 51000, 60001}, 60001)

	port, open, err := ScanInstallPort("100.99.166.81")
	if err != nil {
		t.Fatalf("ScanInstallPort returned error: %v", err)
	}
	if port != 60001 {
		t.Fatalf("expected validated port 60001, got %d", port)
	}
	if len(open) != 4 {
		t.Fatalf("expected 4 open ports, got %v", open)
	}
	if probed[49152] == 0 || probed[51000] == 0 {
		t.Fatalf("lower open ports must be probed before the winner: %v", probed)
	}
	if probed[pairPortFixed] != 0 {
		t.Fatalf("pairing port must not be probed: %v", probed)
	}
}

// The pairing port is dialed by the sweep but must never be offered or
// probed as the install port.
func TestScanInstallPortNeverOffersPairingPort(t *testing.T) {
	dialed, probed := stubScan(t, []uint16{62078, 49152}, 49152)

	port, _, err := ScanInstallPort("100.99.166.81")
	if err != nil {
		t.Fatalf("ScanInstallPort returned error: %v", err)
	}
	if port != 49152 {
		t.Fatalf("expected 49152, got %d", port)
	}
	if dialed[62078] == 0 {
		t.Fatalf("the fixed pairing port sits inside the swept range and must be dialed: %v", dialed)
	}
	if probed[62078] != 0 {
		t.Fatalf("pairing port must not be probed: %v", probed)
	}
}

// Without a working handshake (no saved records), the first open port that
// is not the pairing port is offered, mirroring plain port-scan heuristics.
func TestScanInstallPortFallsBackToFirstOpenPort(t *testing.T) {
	_, _ = stubScan(t, []uint16{51000, 60001}, 0)

	port, _, err := ScanInstallPort("100.99.166.81")
	if err != nil {
		t.Fatalf("ScanInstallPort returned error: %v", err)
	}
	if port != 51000 {
		t.Fatalf("expected first open port 51000, got %d", port)
	}
}

// Ports must be probed in ascending order and the scan stops after the first
// validated handshake.
func TestScanInstallPortStopsAfterHandshake(t *testing.T) {
	_, probed := stubScan(t, []uint16{49152, 49153, 49154, 60001}, 49153)

	port, _, err := ScanInstallPort("100.99.166.81")
	if err != nil {
		t.Fatalf("ScanInstallPort returned error: %v", err)
	}
	if port != 49153 {
		t.Fatalf("expected 49153, got %d", port)
	}
	if probed[49154] != 0 || probed[60001] != 0 {
		t.Fatalf("scan must stop at the first validated port: %v", probed)
	}
}

// A device that only exposes the manual-pairing service has no install port
// yet; the scan must not offer the pairing port as one.
func TestScanInstallPortIgnoresPairingOnlyPort(t *testing.T) {
	_, _ = stubScan(t, []uint16{62078}, 0)

	port, _, err := ScanInstallPort("100.99.166.81")
	if err == nil {
		t.Fatalf("expected an error when only the pairing port is open, got port %d", port)
	}
}

// An unreadable device must be reported, not silently misread.
func TestScanInstallPortNoOpenPorts(t *testing.T) {
	_, _ = stubScan(t, nil, 0)

	port, open, err := ScanInstallPort("100.99.166.81")
	if err == nil {
		t.Fatal("expected an error when no port is open")
	}
	if port != 0 || open != nil {
		t.Fatalf("expected zeroed result, got %d %v", port, open)
	}
}

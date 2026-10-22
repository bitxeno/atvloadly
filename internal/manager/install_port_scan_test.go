package manager

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bitxeno/atvloadly/internal/model"
)

// stubScan controls the network sweep and the tunnel handshake: ports listed
// in openPorts answer dials, and answeredProbe is the one port whose
// handshake succeeds. When answeredProbe is set, the device has a saved
// pairing record; otherwise scanning cannot validate any port. Extra
// records override that decision, expressing a device whose record exists
// but whose every handshake fails. The returned maps count, per port, the
// dials attempted and the handshakes probed.
func stubScan(t *testing.T, openPorts []uint16, answeredProbe uint16, records ...string) (map[uint16]int, map[uint16]int) {
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
	prevRecord := pairingRecordForUDID
	netDial = func(ip string, port uint16, timeout time.Duration) error {
		mu.Lock()
		dialed[port]++
		mu.Unlock()
		if open[port] {
			return nil
		}
		return errors.New("refused")
	}
	probeInstallPort = func(ip string, port uint16, record string) error {
		mu.Lock()
		probed[port]++
		mu.Unlock()
		if port == answeredProbe {
			return nil
		}
		return errors.New("no handshake")
	}
	pairingRecordForUDID = func(udid string) (string, error) {
		if len(records) > 0 {
			return records[0], nil
		}
		if answeredProbe != 0 {
			return "record.plist", nil
		}
		return "", errors.New("no pairing record for device " + udid)
	}
	t.Cleanup(func() {
		netDial = prevDial
		probeInstallPort = prevProbe
		pairingRecordForUDID = prevRecord
	})
	return dialed, probed
}

// scanTarget builds the device the scan runs for: the address of a device
// whose UDID is stored after a pairing.
func scanTarget() *model.Device {
	return &model.Device{IP: "100.99.166.81", UDID: "00008031-000165D02AE3812E"}
}

// A handshake-validated port wins over every other open port, and the fixed
// pairing port inside the swept range is never probed. The open-port list is
// diagnostic only now: the early exit leaves it partial, so the test asserts
// the winner and the probed behavior, not a complete list.
func TestScanDeviceInstallPortValidatesTunnelPort(t *testing.T) {
	_, probed := stubScan(t, []uint16{62078, 49152, 51000, 60001}, 60001)

	port, _, err := ScanDeviceInstallPort(scanTarget())
	if err != nil {
		t.Fatalf("ScanDeviceInstallPort returned error: %v", err)
	}
	if port != 60001 {
		t.Fatalf("expected validated port 60001, got %d", port)
	}
	if probed[pairPortFixed] != 0 {
		t.Fatalf("pairing port must not be probed: %v", probed)
	}
}

// The pairing port is excluded from probing and from being offered; with
// early exit the recorded sweep may end before it reaches the pairing port,
// and its dial coverage belongs to the failed-handshakes test below.
func TestScanDeviceInstallPortNeverOffersPairingPort(t *testing.T) {
	_, probed := stubScan(t, []uint16{62078, 49152}, 49152)

	port, _, err := ScanDeviceInstallPort(scanTarget())
	if err != nil {
		t.Fatalf("ScanInstallPort returned error: %v", err)
	}
	if port != 49152 {
		t.Fatalf("expected 49152, got %d", port)
	}
	if probed[62078] != 0 {
		t.Fatalf("pairing port must not be probed: %v", probed)
	}
}

// A device whose pairing record cannot be resolved is refused before any
// dial: without its own record the scan cannot validate anything, so no
// sweep and no probe happens at all.
func TestScanDeviceInstallPortWithoutRecord(t *testing.T) {
	dialed, probed := stubScan(t, []uint16{51000, 60001, 62078}, 0)

	port, _, err := ScanDeviceInstallPort(scanTarget())
	if port != 0 {
		t.Fatalf("no record means no validated port, got %d", port)
	}
	if err == nil {
		t.Fatal("expected an error when the pairing record cannot be resolved")
	}
	if len(dialed) != 0 || len(probed) != 0 {
		t.Fatalf("the scan must not sweep or probe without a resolvable record: %v %v", dialed, probed)
	}
}

// Probes run at most a few candidates before the validated handshake stops
// the scan: with the winner one step behind the first open port, the total
// probe count stays tiny instead of reaching the cap.
func TestScanDeviceInstallPortStopsAfterHandshake(t *testing.T) {
	total := 0
	totalProbed := func(m map[uint16]int) int {
		sum := 0
		for _, count := range m {
			sum += count
		}
		return sum
	}
	_, probed := stubScan(t, []uint16{49152, 49153, 49154, 60001}, 49153)

	port, _, err := ScanDeviceInstallPort(scanTarget())
	if err != nil {
		t.Fatalf("ScanInstallPort returned error: %v", err)
	}
	if port != 49153 {
		t.Fatalf("expected 49153, got %d", port)
	}
	total = totalProbed(probed)
	if total < 1 || total > 3 {
		t.Fatalf("scan must stop probing quickly at the validated port, got %d probes: %v", total, probed)
	}
}

// The record-driven scan never offers the manual-pairing port: a device
// whose only open port is the pairing port has no install port yet.
func TestScanDeviceInstallPortIgnoresPairingOnlyPort(t *testing.T) {
	_, _ = stubScan(t, []uint16{62078}, 0, "record.plist")

	port, _, err := ScanDeviceInstallPort(scanTarget())
	if err == nil {
		t.Fatalf("expected an error when only the pairing port is open, got port %d", port)
	}
	if !strings.Contains(err.Error(), "pairing-record handshake") {
		t.Fatalf("expected the handshake failure, got: %v", err)
	}
}

// A saved pairing record must not make an arbitrary open TCP port appear
// validated: when every handshake fails, no port is offered and the scan
// reports the failure. The sweep keeps running to the end (no handshake can
// stop it early), so the pairing port inside the range is dialed, and
// probing continues past the failures.
func TestScanDeviceInstallPortRejectsFailedHandshakes(t *testing.T) {
	dialed, probed := stubScan(t, []uint16{49152, 51000, 60001, 62078}, 0, "record.plist")

	port, _, err := ScanDeviceInstallPort(scanTarget())
	if port != 0 {
		t.Fatalf("failed handshakes must not offer a port, got %d", port)
	}
	if err == nil {
		t.Fatal("expected an error when no port passed the handshake")
	}
	if dialed[62078] == 0 {
		t.Fatalf("a scan without a validated handshake must sweep the whole range and dial the fixed pairing port: %v", dialed)
	}
	if probed[49152] == 0 || probed[51000] == 0 {
		t.Fatalf("probing must continue past failed handshakes, not stop at the first: %v", probed)
	}
	if probed[62078] != 0 {
		t.Fatalf("pairing port must not be probed: %v", probed)
	}
}

// An unreadable device must be reported, not silently misread: with a
// resolvable record but no answering port, the scan fails emptily.
func TestScanDeviceInstallPortNoOpenPorts(t *testing.T) {
	_, _ = stubScan(t, nil, 0, "record.plist")

	port, open, err := ScanDeviceInstallPort(scanTarget())
	if err == nil {
		t.Fatal("expected an error when no port is open")
	}
	if port != 0 || open != nil {
		t.Fatalf("expected zeroed result, got %d %v", port, open)
	}
}

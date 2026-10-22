package manager

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
)

// Remote pairing spans two Apple services: the manual-pairing service runs
// the PIN flow on Apple's fixed lockdown port 62078, and the regular
// remote-pairing service carries the install tunnel and the device-info
// handshake. Darwin allocates the install listener from the ephemeral range
// 49152-65535, and 62078 sits inside that range, so the sweep dials it too
// but must never offer it as the install port.
const (
	// pairPortFixed is Apple's fixed manual-pairing (lockdown) port.
	pairPortFixed uint16 = 62078
	// scanPortFirst is the first port of the dynamic service range.
	scanPortFirst uint16 = 49152
	// scanPortLast is the last port of the dynamic service range.
	scanPortLast uint16 = 65535
	// scanConcurrency keeps the sweep bounded even behind slow connections.
	// A phone behind an overlay VPN often silently drops SYNs to closed
	// ports instead of answering with a reset, so worker count and the per
	// dial wait set the sweep's worst case: (65535-49152)/concurrency * wait.
	scanConcurrency = 200
	// networkDialWait bounds a single port probe and stays a full second:
	// an open port answers within a round trip of the used path, and cloud
	// relay round trips approach that second, so tightening it loses the
	// open-port discovery the rest of the scan exists for.
	networkDialWait = time.Second
	// installProbeWait bounds one plumesign device-info attempt.
	installProbeWait = 6 * time.Second
	// maxValidatedPorts caps how many open ports the tunnel handshake tries:
	// plenty for a direct device address, and a guard against a subnet router
	// that forwards every port.
	maxValidatedPorts = 8
)

// netDial is replaceable so tests never dial real endpoints.
var netDial = defaultNetDial

func defaultNetDial(ip string, port uint16, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, strconv.Itoa(int(port))), timeout)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// probeInstallPort is replaceable so tests never run plumesign.
var probeInstallPort = defaultProbeInstallPort

// defaultProbeInstallPort tries the tunnel handshake with the device's
// pairing record: only the regular remote-pairing service completes it.
func defaultProbeInstallPort(ip string, port uint16, record string) error {
	cmd := exec.Command("plumesign", "device-info", "--ip", ip, "--port", fmt.Sprintf("%d", port), "-f", record).
		WithTimeout(installProbeWait)
	if _, err := cmd.CombinedOutput(); err == nil {
		return nil
	}
	return fmt.Errorf("the pairing record does not complete a tunnel on port %d", port)
}

// pairingRecordForUDID is replaceable so tests list curated records.
var pairingRecordForUDID = defaultPairingRecordForUDID

// defaultPairingRecordForUDID resolves the saved pairing record of a paired
// device: plumesign stores every record under the device UDID, both the
// plist next to the tunnel and the peer record repeating it.
func defaultPairingRecordForUDID(udid string) (string, error) {
	if udid == "" {
		return "", fmt.Errorf("the device has no stored UDID yet; run the pairing flow or import a record first")
	}
	record := filepath.Join(app.RemotePairingDir(), udid+".plist")
	if _, err := os.Stat(record); err != nil {
		return "", fmt.Errorf("no pairing record for device %s; run the pairing flow or import a record first", udid)
	}
	return record, nil
}

// sweepOpenPorts is the one dial sweep of the dynamic service range. Dial
// workers stream reachable ports to a conveyor: each port is recorded for
// diagnostics in ascending order, and, when a probe is given, every
// reachable non-pairing port gets its handshake probed as its dial
// completes, capped at maxValidatedPorts candidates with a serialized
// conveyor. The first probe success cancels the remaining sweep and is
// reported as found; with a nil probe nothing can validate a port and the
// complete sorted open-port list is collected instead. Dial workers
// cancelled by an early exit are joined before return, so callers observe
// no lingering probes.
func sweepOpenPorts(ip string, probe func(port uint16) error) (uint16, []uint16) {
	stop := make(chan struct{})
	// Large enough for the entire range, dial workers never block on it.
	opened := make(chan uint16, scanPortLast-scanPortFirst+1)
	validated := make(chan uint16, 1)
	probeDone := make(chan struct{})

	var open []uint16
	dialWG := sync.WaitGroup{}
	dialWG.Add(scanConcurrency)
	jobs := make(chan uint16, scanPortLast-scanPortFirst+1)
	for i := 0; i < scanConcurrency; i++ {
		go func() {
			defer dialWG.Done()
			for port := range jobs {
				select {
				case <-stop:
					return
				default:
				}
				if netDial(ip, port, networkDialWait) == nil {
					opened <- port
				}
			}
		}()
	}
	for port := int(scanPortFirst); port <= int(scanPortLast); port++ {
		jobs <- uint16(port)
	}
	close(jobs)
	go func() {
		dialWG.Wait()
		close(opened)
	}()

	// A single conveyor gives the probes a deterministic ascending order and
	// their cap a simple count, while the dial workers stay ahead of it.
	go func() {
		defer close(probeDone)
		probedTotal := 0
		for port := range opened {
			// The manual-pairing service never carries the install tunnel.
			if port == pairPortFixed {
				open = append(open, port)
				continue
			}
			open = append(open, port)
			if probe == nil || probedTotal >= maxValidatedPorts {
				continue
			}
			probedTotal++
			if probeErr := probe(port); probeErr == nil {
				validated <- port
				return
			} else {
				log.Warnf("install port scan: %s:%d did not validate: %v", ip, port, probeErr)
			}
		}
	}()

	select {
	case port := <-validated:
		// Terminate the sweep and let its workers finish their in-flight
		// dials before returning, so callers observe no lingering probes.
		close(stop)
		dialWG.Wait()
		sort.Slice(open, func(i, j int) bool { return open[i] < open[j] })
		return port, open
	case <-probeDone:
		// The sweep ended without a validated handshake; the conveyor only
		// closes that channel before returning empty, so a written port
		// must be picked up non-blockingly here.
	}
	select {
	case port := <-validated:
		sort.Slice(open, func(i, j int) bool { return open[i] < open[j] })
		return port, open
	default:
	}
	sort.Slice(open, func(i, j int) bool { return open[i] < open[j] })
	return 0, open
}

// ScanDeviceInstallPort finds the install-service port of a paired device.
// The sweep dials the port range while the device's own saved pairing
// record — resolved from its stored UDID — gets its tunnel handshake probed
// against every reachable non-pairing port as its dial completes; the first
// validated handshake terminates the remaining sweep and answers
// immediately, which behind slow overlay paths (cellular relays in
// particular) runs in the time of finding the port instead of the time of
// sweeping the range. Records of unrelated devices are never tried, and
// only a handshake-validated port is ever offered: a non-validated port
// means no install port was found, every other opener is an error. The
// open-port list is returned for diagnostics.
func ScanDeviceInstallPort(dev *model.Device) (uint16, []uint16, error) {
	record, err := pairingRecordForUDID(dev.UDID)
	if err != nil {
		return 0, nil, err
	}
	found, open := sweepOpenPorts(dev.IP, func(port uint16) error {
		return probeInstallPort(dev.IP, port, record)
	})
	if found > 0 {
		return found, open, nil
	}
	if len(open) == 0 {
		return 0, nil, fmt.Errorf("no open developer ports found; check that the device is reachable and that Remote Access is enabled")
	}
	return 0, open, fmt.Errorf("no open port passed the pairing-record handshake on %s", dev.IP)
}

// ScanAndImportPairingFile selects an install port by checking the uploaded
// pairing record itself. Saved records may be for unrelated devices and must
// not determine which port is selected during the first import.
// A successful check also imports the record into the plumesign store.
func ScanAndImportPairingFile(ip string, record []byte) (uint16, error) {
	return scanAndImportPairingFile(ip, record, ImportPairingFile)
}

// The check function is injected so candidate selection can be tested
// without invoking plumesign or writing pairing credentials.
func scanAndImportPairingFile(ip string, record []byte, check func(string, string, []byte) error) (uint16, error) {
	_, open := sweepOpenPorts(ip, nil)
	attempts := 0
	for _, port := range open {
		if port == pairPortFixed {
			continue
		}
		if attempts >= maxValidatedPorts {
			break
		}
		attempts++
		if err := check(ip, strconv.Itoa(int(port)), record); err == nil {
			return port, nil
		}
	}
	if attempts == 0 {
		return 0, fmt.Errorf("no install-port candidates reachable at %s", ip)
	}
	return 0, fmt.Errorf("uploaded pairing record did not validate against any of %d candidate ports at %s", attempts, ip)
}

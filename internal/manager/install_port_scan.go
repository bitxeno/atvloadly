package manager

import (
	"fmt"
	"net"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
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
	scanConcurrency = 200
	// networkDialWait bounds a single port probe.
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

// scanOpenPorts sweeps the dynamic service range and returns all reachable
// ports in ascending order.
func scanOpenPorts(ip string) ([]uint16, error) {
	scanJobs := make(chan uint16, int(scanPortLast)-int(scanPortFirst)+1)
	for port := int(scanPortFirst); port <= int(scanPortLast); port++ {
		scanJobs <- uint16(port)
	}
	close(scanJobs)

	open := make(chan uint16, cap(scanJobs))
	jobs := scanJobs

	var wg sync.WaitGroup
	wg.Add(scanConcurrency)
	for i := 0; i < scanConcurrency; i++ {
		go func() {
			defer wg.Done()
			for port := range jobs {
				if netDial(ip, port, networkDialWait) == nil {
					open <- port
				}
			}
		}()
	}
	wg.Wait()
	close(open)

	var found []uint16
	for port := range open {
		found = append(found, port)
	}
	sort.Slice(found, func(i, j int) bool { return found[i] < found[j] })
	return found, nil
}

// probeInstallPort is replaceable so tests never run plumesign.
var probeInstallPort = defaultProbeInstallPort

// defaultProbeInstallPort tries the tunnel handshake against the given
// pairing records: only the regular remote-pairing service completes it.
func defaultProbeInstallPort(ip string, port uint16, records []string) error {
	if len(records) == 0 {
		return fmt.Errorf("no matching pairing record")
	}
	for _, file := range records {
		cmd := exec.Command("plumesign", "device-info", "--ip", ip, "--port", fmt.Sprintf("%d", port), "-f", file).
			WithTimeout(installProbeWait)
		if _, err := cmd.CombinedOutput(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no saved pairing record completes a tunnel on port %d", port)
}

// pairingRecordPaths is replaceable so tests list curated records.
var pairingRecordPaths = defaultPairingRecordPaths

// defaultPairingRecordPaths lists the pairing files saved on this server.
func defaultPairingRecordPaths() []string {
	paths, _ := filepath.Glob(filepath.Join(app.RemotePairingDir(), "*.plist"))
	return paths
}

// ScanInstallPort finds the port of the regular remote-pairing service on a
// device reachable over the network. The tunnel handshake identifies the
// install port when a saved pairing record validates it; otherwise the first
// open port in the dynamic range is offered, mirroring plain port-scan
// heuristics. The full open-port list is returned for diagnostics.
func ScanInstallPort(ip string) (uint16, []uint16, error) {
	open, err := scanOpenPorts(ip)
	if err != nil {
		return 0, nil, fmt.Errorf("port scan of %s failed: %w", ip, err)
	}
	if len(open) == 0 {
		return 0, nil, fmt.Errorf("no open developer ports found; check that the device is reachable and that Remote Access is enabled")
	}

	records := pairingRecordPaths()
	if len(records) > 0 {
		attempts := 0
		for _, port := range open {
			// The manual-pairing service never carries the install tunnel.
			if port == pairPortFixed {
				continue
			}
			if attempts >= maxValidatedPorts {
				break
			}
			attempts++
			if probeErr := probeInstallPort(ip, port, records); probeErr == nil {
				return port, open, nil
			} else {
				log.Warnf("install port scan: %s:%d did not validate: %v", ip, port, probeErr)
			}
		}
		return 0, open, fmt.Errorf("no install port passed the pairing-record handshake for %s", ip)
	}

	// Without saved records, preserve the candidate-port heuristic for
	// the initial pairing-record import/bootstrap flow.
	for _, port := range open {
		if port != pairPortFixed {
			return port, open, nil
		}
	}
	return 0, open, fmt.Errorf("only the manual-pairing port is open; run the pairing flow first")
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
	open, err := scanOpenPorts(ip)
	if err != nil {
		return 0, fmt.Errorf("port scan of %s failed: %w", ip, err)
	}

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

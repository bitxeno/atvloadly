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

// Remote devices advertise two services: the manual-pairing service accepts
// the PIN flow on its own port, and the regular remote-pairing service
// carries createListener installs. The install port lives somewhere in the
// dynamic range; the manual-pairing port of tvOS devices historically sits
// at one fixed value.
const (
	// pairPortTvOS is the manual-pairing service port of Apple TVs.
	pairPortTvOS uint16 = 65300
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

// scanOpenPorts sweeps the dynamic service range plus the tvOS pairing port
// and returns all reachable ports in ascending order.
func scanOpenPorts(ip string) ([]uint16, error) {
	scanJobs := make(chan uint16, int(scanPortLast)-int(scanPortFirst)+1)
	for port := int(scanPortFirst); port <= int(scanPortLast); port++ {
		if port == int(pairPortTvOS) {
			continue
		}
		scanJobs <- uint16(port)
	}
	scanJobs <- pairPortTvOS
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

	var installPorts []uint16
	records := pairingRecordPaths()
	if len(records) > 0 {
		for _, port := range open {
			// The tvOS pairing port sits inside the dynamic range but is
			// never the install port.
			if port == pairPortTvOS {
				continue
			}
			if probeErr := probeInstallPort(ip, port, records); probeErr == nil {
				installPorts = append(installPorts, port)
				break
			} else {
				log.Warnf("install port scan: %s:%d did not validate: %v", ip, port, probeErr)
			}
			if len(installPorts) >= maxValidatedPorts {
				break
			}
		}
	}

	if len(installPorts) > 0 {
		return installPorts[0], open, nil
	}
	// Without a validated handshake (no records, or validation refused),
	// offer the first open port in the dynamic range.
	for _, port := range open {
		if port >= scanPortFirst && port != pairPortTvOS {
			return port, open, nil
		}
	}
	return 0, open, fmt.Errorf("only the manual-pairing port is open; run the pairing flow first")
}

package manager

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const rpPairSetupResponse = `{"message":{"plain":{"_0":{"response":{"forRequestIdentifier":0,"_1":{"handshake":{"_0":{"wireProtocolVersion":22,"minimumSupportedWireProtocolVersion":8,"deviceOptions":{"allowsPairSetup":true,"allowsIncomingTunnelConnections":false,"allowsPinlessPairing":false},"peerDeviceInfo":{"name":"AppleTV","model":"AppleTV11,1"}}}}}}}},"originatedBy":"device","sequenceNumber":0}`

const rpTunnelResponse = `{"message":{"plain":{"_0":{"response":{"forRequestIdentifier":0,"_1":{"handshake":{"_0":{"wireProtocolVersion":22,"minimumSupportedWireProtocolVersion":8,"deviceOptions":{"allowsPairSetup":false,"allowsIncomingTunnelConnections":true,"allowsSharingSensitiveInfo":true}}}}}}}},"originatedBy":"device","sequenceNumber":0}`

func TestRPHandshakeFrame(t *testing.T) {
	frame := rpHandshakeFrame()

	if len(frame) < 11 {
		t.Fatalf("frame too short: %d", len(frame))
	}
	if got := string(frame[:9]); got != "RPPairing" {
		t.Fatalf("magic = %q, want RPPairing", got)
	}
	bodyLen := int(binary.BigEndian.Uint16(frame[9:11]))
	if bodyLen != len(frame)-11 {
		t.Fatalf("declared body len %d, actual %d", bodyLen, len(frame)-11)
	}

	var body map[string]any
	if err := json.Unmarshal(frame[11:], &body); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	raw := string(frame[11:])
	if !strings.Contains(raw, `"attemptPairVerify":true`) {
		t.Fatalf("handshake request must contain attemptPairVerify: %s", raw)
	}
	if !strings.Contains(raw, `"wireProtocolVersion":19`) {
		t.Fatalf("handshake request must contain wireProtocolVersion: %s", raw)
	}
}

func TestParseHandshakeBodyClassifiesServices(t *testing.T) {
	pair, err := parseHandshakeBody([]byte(rpPairSetupResponse), 61108)
	if err != nil {
		t.Fatalf("parse pair-setup: %v", err)
	}
	if !pair.PairSetup || pair.Tunnel {
		t.Fatalf("pair-setup classification wrong: %+v", pair)
	}
	if pair.Model != "AppleTV11,1" || pair.DeviceName != "AppleTV" {
		t.Fatalf("pair-setup identity wrong: %+v", pair)
	}

	tunnel, err := parseHandshakeBody([]byte(rpTunnelResponse), 49152)
	if err != nil {
		t.Fatalf("parse tunnel: %v", err)
	}
	if tunnel.PairSetup || !tunnel.Tunnel {
		t.Fatalf("tunnel classification wrong: %+v", tunnel)
	}
}

// rpMockServer answers one RPPairing handshake per connection with the given
// body, then closes (mirroring the device behaviour).
func rpMockServer(t *testing.T, body string) (uint16, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				magic := make([]byte, len(rpPairingMagic))
				if _, err := io.ReadFull(c, magic); err != nil {
					return
				}
				lenBuf := make([]byte, 2)
				if _, err := io.ReadFull(c, lenBuf); err != nil {
					return
				}
				n := binary.BigEndian.Uint16(lenBuf)
				if _, err := io.ReadFull(c, make([]byte, n)); err != nil {
					return
				}
				out := []byte(body)
				frame := append([]byte(rpPairingMagic), byte(len(out)>>8), byte(len(out)))
				frame = append(frame, out...)
				_, _ = c.Write(frame)
			}(conn)
		}
	}()
	port := uint16(ln.Addr().(*net.TCPAddr).Port)
	return port, func() { _ = ln.Close() }
}

func TestProbePairingServiceAgainstMock(t *testing.T) {
	port, stop := rpMockServer(t, rpPairSetupResponse)
	defer stop()

	info, err := ProbePairingService(context.Background(), "127.0.0.1", port, 2*time.Second)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !info.PairSetup || info.Port != port {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestProbePairingServiceRejectsNonRPPairing(t *testing.T) {
	// A plain TCP server that answers with garbage is not an RPPairing service.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				_, _ = c.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
			}(conn)
		}
	}()
	port := uint16(ln.Addr().(*net.TCPAddr).Port)
	if _, err := ProbePairingService(context.Background(), "127.0.0.1", port, time.Second); err == nil {
		t.Fatal("expected error probing a non-RPPairing server")
	}
}

func TestDiscoverManualPairingPortFindsPairSetup(t *testing.T) {
	port, stop := rpMockServer(t, rpPairSetupResponse)
	defer stop()

	var stages []string
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pairPort, info, err := discoverManualPairingPort(ctx, "127.0.0.1", port, port, true, func(stage string, _ int) {
		stages = append(stages, stage)
	})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if pairPort != port || info == nil || !info.PairSetup {
		t.Fatalf("unexpected result: port=%d info=%+v", pairPort, info)
	}
	if len(stages) == 0 || stages[len(stages)-1] != DiscoverStageFound {
		t.Fatalf("expected to end in found stage, got %v", stages)
	}
}

func TestDiscoverManualPairingPortNotFound(t *testing.T) {
	// Nothing listens in this range on 127.0.0.1: expect not found.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := DiscoverManualPairingPort(ctx, "127.0.0.1", nil)
	if err != ErrPairingPortNotFound {
		t.Fatalf("want ErrPairingPortNotFound, got %v", err)
	}
}

func TestDiscoverTunnelPortFindsTunnel(t *testing.T) {
	port, stop := rpMockServer(t, rpTunnelResponse)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var stages []string
	gotPort, info, err := discoverPortByService(ctx, "127.0.0.1", rpTunnelFQDN, port, port, true, func(si *PairingServiceInfo) bool {
		return si.Tunnel
	}, func(stage string, _ int) {
		stages = append(stages, stage)
	})
	if err != nil {
		t.Fatalf("discover tunnel: %v", err)
	}
	if gotPort != port || info == nil || !info.Tunnel {
		t.Fatalf("unexpected result: port=%d info=%+v", gotPort, info)
	}
	if len(stages) == 0 || stages[len(stages)-1] != DiscoverStageFound {
		t.Fatalf("expected to end in found stage, got %v", stages)
	}
}

func TestDiscoverTunnelPortIgnoresPairSetupOnly(t *testing.T) {
	// A pair-setup-only service must not be selected as a tunnel port.
	port, stop := rpMockServer(t, rpPairSetupResponse)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	gotPort, _, err := discoverPortByService(ctx, "127.0.0.1", rpTunnelFQDN, port, port, true, func(si *PairingServiceInfo) bool {
		return si.Tunnel
	}, nil)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if gotPort != 0 {
		t.Fatalf("pair-setup service must not be picked as tunnel, got port %d", gotPort)
	}
}

func TestScanProbedPortsStopsEarlyOnLowPort(t *testing.T) {
	// The tunnel service sits on the first port of a wide range: the scan
	// must return without dialing anywhere near the whole range.
	port, stop := rpMockServer(t, rpTunnelResponse)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := time.Now()
	si, _ := scanProbedPorts(ctx, "127.0.0.1", port, 65535, func(si *PairingServiceInfo) bool {
		return si.Tunnel
	}, nil)
	elapsed := time.Since(start)
	if si == nil || si.Port != port {
		t.Fatalf("unexpected result: %+v", si)
	}
	// A full 16k-port scan takes seconds; an early win on the first port
	// finishes in well under a second.
	if elapsed > 2*time.Second {
		t.Fatalf("scan did not stop early: took %v", elapsed)
	}
}

func TestSkipMDNSForTailnetIPs(t *testing.T) {
	cases := map[string]bool{
		"100.65.100.37": true,  // Tailscale CGNAT range
		"100.100.100.1": true,  // Tailscale control plane range
		"192.168.1.229": false, // LAN
		"10.0.0.5":      false,
		"not-an-ip":     false,
	}
	for ip, want := range cases {
		if got := skipMDNSForIP(ip); got != want {
			t.Errorf("skipMDNSForIP(%q) = %v, want %v", ip, got, want)
		}
	}
}

// stubScanDial controls the per-port dial outcome so the scan's reachability
// bookkeeping can be tested without real sockets: open ports accept, and every
// other port fails with the given error (refused or timeout).
func stubScanDial(t *testing.T, openPorts []uint16, otherErr func() error) {
	t.Helper()
	prev := scanDialFunc
	open := make(map[uint16]bool, len(openPorts))
	for _, p := range openPorts {
		open[p] = true
	}
	scanDialFunc = func(ctx context.Context, ip string, port uint16, timeout time.Duration) error {
		if open[port] {
			return nil
		}
		return otherErr()
	}
	t.Cleanup(func() { scanDialFunc = prev })
}

// A sweep where every dial is refused (RST) still proves the host is up: the
// scan reports hostAnswered and the caller returns "port not found", not
// unreachable.
func TestScanProbedPortsRefusedMeansHostAnswered(t *testing.T) {
	stubScanDial(t, nil, func() error { return &net.OpError{Err: errors.New("connection refused")} })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	si, answered := scanProbedPorts(ctx, "100.65.100.37", rpPortFirst, rpPortFirst+8, func(*PairingServiceInfo) bool {
		return false
	}, nil)
	if si != nil {
		t.Fatalf("no service should be found, got %+v", si)
	}
	if !answered {
		t.Fatal("refused dials prove the host answered")
	}
}

// A sweep where every dial times out means the host is offline: hostAnswered
// stays false.
func TestScanProbedPortsAllTimeoutsMeansUnreachable(t *testing.T) {
	stubScanDial(t, nil, func() error {
		return &net.OpError{Err: &timeoutError{}}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, answered := scanProbedPorts(ctx, "100.65.100.37", rpPortFirst, rpPortFirst+8, func(*PairingServiceInfo) bool {
		return false
	}, nil)
	if answered {
		t.Fatal("a host that never answers must not count as reachable")
	}
}

// timeoutError is a net.Error whose Timeout() is true, mirroring a dial that
// silently drops the SYN.
type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// discoverPortByService turns an all-timeout sweep into ErrDeviceUnreachable so
// the caller reports an offline device instead of a misleading "port not found".
func TestDiscoverPortByServiceReportsUnreachable(t *testing.T) {
	stubScanDial(t, nil, func() error { return &net.OpError{Err: &timeoutError{}} })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := discoverPortByService(ctx, "100.65.100.37", rpTunnelFQDN, rpPortFirst, rpPortFirst+8, true, func(si *PairingServiceInfo) bool {
		return si.Tunnel
	}, nil)
	if !errors.Is(err, ErrDeviceUnreachable) {
		t.Fatalf("want ErrDeviceUnreachable, got %v", err)
	}
}

// An offline host (every dial times out) must abort after roughly one silent
// wave instead of sweeping the whole 16k-port range, so the user learns the
// device is unreachable quickly.
func TestScanProbedPortsAbortsEarlyWhenUnreachable(t *testing.T) {
	var dials int64
	prev := scanDialFunc
	scanDialFunc = func(context.Context, string, uint16, time.Duration) error {
		atomic.AddInt64(&dials, 1)
		return &net.OpError{Err: &timeoutError{}}
	}
	t.Cleanup(func() { scanDialFunc = prev })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := time.Now()
	si, answered := scanProbedPorts(ctx, "100.65.100.37", rpPortFirst, rpPortLast, func(*PairingServiceInfo) bool {
		return true
	}, nil)
	if si != nil {
		t.Fatalf("no service expected, got %+v", si)
	}
	if answered {
		t.Fatal("all-timeout host must not be reported as reachable")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("offline sweep should abort early, took %v", elapsed)
	}
	// It must not have dialed anywhere near the full range.
	if got := atomic.LoadInt64(&dials); got > int64(unreachableAbortAfter)+int64(rpDialConcurrency) {
		t.Fatalf("dialed %d ports, expected ~%d", got, unreachableAbortAfter)
	}
}

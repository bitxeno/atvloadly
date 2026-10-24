package manager

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

// Discovery of the Apple device Remote Pairing service when only the IP is
// known (manual entry / Tailscale). See doc/misc/remote-pairing-port-discovery.md.
//
// The manual-pairing port runs the PIN flow (allowsPairSetup); it is dynamic
// and never 62078 (that is lockdownd). This file only probes ports, it never
// pairs, so it is safe to run repeatedly.

const (
	rpPairingMagic = "RPPairing" // 9 bytes, must be read exactly

	// rpHandshakeRequest is the compact handshake request. hostOptions must
	// contain attemptPairVerify and wireProtocolVersion must be present, or
	// the device closes the connection without answering.
	rpHandshakeRequest = `{"message":{"plain":{"_0":{"request":{"_0":{"handshake":{"_0":{"hostOptions":{"attemptPairVerify":true},"wireProtocolVersion":19}}}}}}},"originatedBy":"host","sequenceNumber":0}`

	// rpManualPairingFQDN is the manual-pairing service type for the unicast
	// DNS-SD query.
	rpManualPairingFQDN = "_remotepairing-manual-pairing._tcp.local."

	// rpTunnelFQDN is the tunnel/install service type (used for iPhone, which
	// is validated over the _remotepairing tunnel port).
	rpTunnelFQDN = "_remotepairing._tcp.local."

	// Dynamic service ports live in the Darwin ephemeral range.
	rpPortFirst = 49152
	rpPortLast  = 65535

	// Tuned from the design doc's measurements: 1024 concurrent dials scan
	// the whole range in a few seconds without exhausting local ports. The
	// 600ms dial timeout leaves room for high-latency overlay networks
	// (Tailscale relay RTT is 100-250ms): at 250ms the open service ports
	// that need a full handshake often exceed the budget and are missed,
	// which makes tunnel discovery fail intermittently.
	rpDialConcurrency  = 1024
	rpDialTimeout      = 600 * time.Millisecond
	rpProbeConcurrency = 16
	rpProbeTimeout     = 2 * time.Second
	rpMDNSTimeout      = 1500 * time.Millisecond
)

// ErrPairingPortNotFound is returned when no port answers allowsPairSetup.
var ErrPairingPortNotFound = errors.New("manual pairing port not found")

// ErrTunnelPortNotFound is returned when no port answers
// allowsIncomingTunnelConnections.
var ErrTunnelPortNotFound = errors.New("manual tunnel port not found")

// Discovery stages reported to the caller so the UI can show progress.
const (
	DiscoverStageMDNS     = "mdns"
	DiscoverStageScanning = "scanning"
	DiscoverStageProbing  = "probing"
	DiscoverStageFound    = "found"
	DiscoverStageFailed   = "failed"
)

// PairingServiceInfo describes one RPPairing endpoint discovered by handshake.
type PairingServiceInfo struct {
	Port        uint16
	PairSetup   bool // allowsPairSetup: runs the PIN pairing flow
	Tunnel      bool // allowsIncomingTunnelConnections: install/tunnel port
	Model       string
	DeviceName  string
	ProtocolVer uint64
}

// DiscoveryProgress reports discovery progress. stage is one of the
// DiscoverStage* constants; percent (0-100) is only meaningful while scanning.
type DiscoveryProgress func(stage string, percent int)

// rpHandshakeFrame returns the framed handshake request.
func rpHandshakeFrame() []byte {
	body := []byte(rpHandshakeRequest)
	frame := make([]byte, 0, len(rpPairingMagic)+2+len(body))
	frame = append(frame, rpPairingMagic...)
	frame = append(frame, byte(len(body)>>8), byte(len(body)))
	frame = append(frame, body...)
	return frame
}

// rpHandshakeResponse mirrors the nested handshake response envelope.
type rpHandshakeResponse struct {
	Message struct {
		Plain struct {
			Zero struct {
				Response struct {
					ForRequestIdentifier int `json:"forRequestIdentifier"`
					Result               struct {
						Handshake struct {
							Zero rpHandshakeResult `json:"_0"`
						} `json:"handshake"`
					} `json:"_1"`
				} `json:"response"`
			} `json:"_0"`
		} `json:"plain"`
	} `json:"message"`
}

type rpHandshakeResult struct {
	WireProtocolVersion                 uint64 `json:"wireProtocolVersion"`
	MinimumSupportedWireProtocolVersion uint64 `json:"minimumSupportedWireProtocolVersion"`
	DeviceOptions                       struct {
		AllowsPairSetup                          bool `json:"allowsPairSetup"`
		AllowsIncomingTunnelConnections          bool `json:"allowsIncomingTunnelConnections"`
		AllowsPinlessPairing                     bool `json:"allowsPinlessPairing"`
		AllowsPromptlessAutomationPairingUpgrade bool `json:"allowsPromptlessAutomationPairingUpgrade"`
		AllowsSharingSensitiveInfo               bool `json:"allowsSharingSensitiveInfo"`
	} `json:"deviceOptions"`
	PeerDeviceInfo struct {
		Name  string `json:"name"`
		Model string `json:"model"`
	} `json:"peerDeviceInfo"`
}

// parseHandshakeBody decodes a handshake response body into PairingServiceInfo.
func parseHandshakeBody(body []byte, port uint16) (*PairingServiceInfo, error) {
	var resp rpHandshakeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode handshake response: %w", err)
	}
	hs := resp.Message.Plain.Zero.Response.Result.Handshake.Zero
	return &PairingServiceInfo{
		Port:        port,
		PairSetup:   hs.DeviceOptions.AllowsPairSetup,
		Tunnel:      hs.DeviceOptions.AllowsIncomingTunnelConnections,
		Model:       hs.PeerDeviceInfo.Model,
		DeviceName:  hs.PeerDeviceInfo.Name,
		ProtocolVer: hs.WireProtocolVersion,
	}, nil
}

// ProbePairingService dials one port and performs the RPPairing handshake.
// A nil error with a non-nil result means the port is an RPPairing service;
// any other outcome (dial error, wrong magic, timeout) returns an error.
func ProbePairingService(ctx context.Context, ip string, port uint16, timeout time.Duration) (*PairingServiceInfo, error) {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(int(port))))
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	if _, err := conn.Write(rpHandshakeFrame()); err != nil {
		return nil, err
	}

	magic := make([]byte, len(rpPairingMagic))
	if _, err := io.ReadFull(conn, magic); err != nil {
		return nil, err
	}
	if string(magic) != rpPairingMagic {
		return nil, fmt.Errorf("port %d is not an RPPairing service", port)
	}

	lenBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint16(lenBuf)
	if n == 0 {
		return nil, fmt.Errorf("port %d returned an empty handshake", port)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}
	return parseHandshakeBody(body, port)
}

// discoverPairingPortMDNS sends a unicast DNS-SD query to the device's 5353 and
// returns the manual-pairing SRV port when the device answers. This is the fast
// path on the same LAN; it fails across overlay networks.
func discoverPairingPortMDNS(ctx context.Context, ip string, timeout time.Duration) (uint16, bool) {
	return discoverPortMDNS(ctx, ip, rpManualPairingFQDN, timeout)
}

// discoverPortMDNS sends a unicast DNS-SD PTR query for fqdn and returns the
// first SRV port in the answer.
func discoverPortMDNS(ctx context.Context, ip, fqdn string, timeout time.Duration) (uint16, bool) {
	m := new(dns.Msg)
	m.SetQuestion(fqdn, dns.TypePTR)
	m.RecursionDesired = false

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(ip, "5353"))
	if err != nil {
		return 0, false
	}
	defer func() { _ = conn.Close() }()

	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	dnsConn := &dns.Conn{Conn: conn}
	if err := dnsConn.WriteMsg(m); err != nil {
		return 0, false
	}
	resp, err := dnsConn.ReadMsg()
	if err != nil {
		return 0, false
	}
	for _, ans := range resp.Answer {
		if srv, ok := ans.(*dns.SRV); ok && srv.Port != 0 {
			return srv.Port, true
		}
	}
	return 0, false
}

// scanOpenPorts dials every port in [first,last] and returns the open ones.
// onProgress receives a monotonically increasing done count for the UI.
func scanOpenPorts(ctx context.Context, ip string, first, last uint16, concurrency int, dialTimeout time.Duration, onProgress func(done, total int)) []uint16 {
	total := int(last) - int(first) + 1
	if total <= 0 {
		return nil
	}

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		open   []uint16
		done   int64
		report int32
	)
	sem := make(chan struct{}, concurrency)

	for port := int(first); port <= int(last); port++ {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			defer func() { <-sem }()

			dialer := net.Dialer{Timeout: dialTimeout}
			conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(p)))
			if err == nil {
				_ = conn.Close()
				mu.Lock()
				open = append(open, uint16(p))
				mu.Unlock()
			}

			n := atomic.AddInt64(&done, 1)
			if onProgress != nil {
				pct := int32(n * 100 / int64(total))
				prev := atomic.LoadInt32(&report)
				if (pct >= prev+5 || n == int64(total)) && atomic.CompareAndSwapInt32(&report, prev, pct) {
					onProgress(int(n), total)
				}
			}
		}(port)
	}
	wg.Wait()

	sort.Slice(open, func(i, j int) bool { return open[i] < open[j] })
	return open
}

// probePorts handshakes every candidate port and returns the RPPairing services.
func probePorts(ctx context.Context, ip string, ports []uint16, concurrency int, timeout time.Duration) []*PairingServiceInfo {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []*PairingServiceInfo
	)
	sem := make(chan struct{}, concurrency)
	for _, port := range ports {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(p uint16) {
			defer wg.Done()
			defer func() { <-sem }()
			info, err := ProbePairingService(ctx, ip, p, timeout)
			if err == nil && info != nil {
				mu.Lock()
				results = append(results, info)
				mu.Unlock()
			}
		}(port)
	}
	wg.Wait()
	return results
}

// portMatcher reports whether a handshaked service is the wanted one.
type portMatcher func(*PairingServiceInfo) bool

// discoverPortByService is the testable core shared by the manual-pairing and
// tunnel discovery: it tries the mDNS fast path (unless skipped) then the
// two-stage TCP scan over [first,last], returning the first port that satisfies
// pick.
func discoverPortByService(ctx context.Context, ip, fqdn string, first, last uint16, skipMDNS bool, pick portMatcher, progress DiscoveryProgress) (port uint16, info *PairingServiceInfo, err error) {
	report := func(stage string, pct int) {
		if progress != nil {
			progress(stage, pct)
		}
	}

	if !skipMDNS {
		report(DiscoverStageMDNS, 0)
		if p, ok := discoverPortMDNS(ctx, ip, fqdn, rpMDNSTimeout); ok {
			if si, perr := ProbePairingService(ctx, ip, p, rpProbeTimeout); perr == nil && pick(si) {
				report(DiscoverStageFound, 100)
				return p, si, nil
			}
		}
	}

	report(DiscoverStageScanning, 0)
	open := scanOpenPorts(ctx, ip, first, last, rpDialConcurrency, rpDialTimeout, func(done, total int) {
		report(DiscoverStageScanning, done*100/total)
	})
	if ctx.Err() != nil {
		return 0, nil, ctx.Err()
	}

	report(DiscoverStageProbing, 0)
	for _, si := range probePorts(ctx, ip, open, rpProbeConcurrency, rpProbeTimeout) {
		if pick(si) {
			report(DiscoverStageFound, 100)
			return si.Port, si, nil
		}
	}

	report(DiscoverStageFailed, 100)
	return 0, nil, nil
}

// discoverManualPairingPort is the manual-pairing variant: it also captures the
// tunnel port seen during the same scan.
func discoverManualPairingPort(ctx context.Context, ip string, first, last uint16, skipMDNS bool, progress DiscoveryProgress) (pairPort, tunnelPort uint16, info *PairingServiceInfo, err error) {
	report := func(stage string, pct int) {
		if progress != nil {
			progress(stage, pct)
		}
	}

	if !skipMDNS {
		report(DiscoverStageMDNS, 0)
		if p, ok := discoverPairingPortMDNS(ctx, ip, rpMDNSTimeout); ok {
			if si, perr := ProbePairingService(ctx, ip, p, rpProbeTimeout); perr == nil && si.PairSetup {
				report(DiscoverStageFound, 100)
				return p, 0, si, nil
			}
		}
	}

	report(DiscoverStageScanning, 0)
	open := scanOpenPorts(ctx, ip, first, last, rpDialConcurrency, rpDialTimeout, func(done, total int) {
		report(DiscoverStageScanning, done*100/total)
	})
	if ctx.Err() != nil {
		return 0, 0, nil, ctx.Err()
	}

	report(DiscoverStageProbing, 0)
	for _, si := range probePorts(ctx, ip, open, rpProbeConcurrency, rpProbeTimeout) {
		if si.PairSetup && pairPort == 0 {
			pairPort = si.Port
			info = si
		}
		if si.Tunnel && tunnelPort == 0 {
			tunnelPort = si.Port
		}
	}

	if pairPort == 0 {
		report(DiscoverStageFailed, 100)
		return 0, tunnelPort, nil, ErrPairingPortNotFound
	}
	report(DiscoverStageFound, 100)
	return pairPort, tunnelPort, info, nil
}

// DiscoverManualPairingPort finds the manual-pairing port for a manually
// specified IP: mDNS unicast fast path, then a full two-stage TCP scan.
func DiscoverManualPairingPort(ctx context.Context, ip string, progress DiscoveryProgress) (pairPort, tunnelPort uint16, info *PairingServiceInfo, err error) {
	return discoverManualPairingPort(ctx, ip, rpPortFirst, rpPortLast, false, progress)
}

// DiscoverManualTunnelPort finds the _remotepairing tunnel port for a manually
// specified iPhone: mDNS unicast fast path, then a two-stage TCP scan. The
// tunnel port is where `plumesign check pairing` validates the pairing file.
func DiscoverManualTunnelPort(ctx context.Context, ip string, progress DiscoveryProgress) (port uint16, info *PairingServiceInfo, err error) {
	port, info, err = discoverPortByService(ctx, ip, rpTunnelFQDN, rpPortFirst, rpPortLast, false, func(si *PairingServiceInfo) bool {
		return si.Tunnel
	}, progress)
	if err != nil {
		return 0, nil, err
	}
	if port == 0 {
		return 0, nil, ErrTunnelPortNotFound
	}
	return port, info, nil
}

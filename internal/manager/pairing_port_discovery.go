package manager

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
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

	// Tuned against a real device over a Tailscale relay (see
	// doc/misc/remote-pairing-port-discovery.md): a burst of thousands of
	// SYNs saturates the relay queue and the open service ports themselves
	// start timing out, so at 1024+ concurrency most full scans miss the
	// tunnel port entirely. A small wave (128 dials) keeps the relay
	// responsive: closed ports answer RST fast and only ~600 of 16k dials
	// hit the timeout, and every full sweep finds the service. The 1.5s
	// timeout covers the relay's bursty RTT (single dials measure 0.3-1.1s
	// under no load), which a 600ms budget cannot. Lowest ports are found
	// in the first wave thanks to the ascending order and early exit.
	rpDialConcurrency = 128
	rpDialTimeout     = 1500 * time.Millisecond
	// Host-local services inside the range (rapportd, the app's own HTTP
	// listener) accept connections but never answer the handshake, so they
	// hold a probe slot until the probe timeout. Keep enough slots that a
	// handful of silent listeners cannot starve the real service.
	rpProbeConcurrency = 32
	rpProbeTimeout     = 2 * time.Second
	// Accepted-but-silent listeners (rapportd, the app's own HTTP server on
	// 127.0.0.1) answer nothing, so a probe against them runs the full probe
	// timeout. Their handshake response deadline is bounded separately so
	// they release their probe slot quickly.
	rpProbeResponseTimeout = 800 * time.Millisecond
	rpMDNSTimeout          = 1500 * time.Millisecond
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
	return probePairingService(ctx, ip, port, timeout, rpProbeResponseTimeout)
}

// probePairingService splits the probe budget: timeout bounds the dial (which
// needs full RTT headroom on overlay networks), responseTimeout bounds the
// handshake exchange once connected (real devices answer immediately, silent
// local listeners are abandoned early).
func probePairingService(ctx context.Context, ip string, port uint16, timeout, responseTimeout time.Duration) (*PairingServiceInfo, error) {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(int(port))))
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	deadline := time.Now().Add(responseTimeout)
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

// scanProbedPorts dials ports in [first,last] in ascending order and probes
// every open port with the RPPairing handshake as soon as its dial succeeds.
// The first probed service satisfying pick wins: discovery returns
// immediately and cancels the remaining work, so services on low ports
// (the common case, since Darwin allocates ephemeral ports ascending) are
// found without scanning the rest of the range. It returns nil when the
// range is exhausted or ctx is cancelled.
func scanProbedPorts(ctx context.Context, ip string, first, last uint16, pick portMatcher, progress DiscoveryProgress) *PairingServiceInfo {
	report := func(stage string, pct int) {
		if progress != nil {
			progress(stage, pct)
		}
	}

	report(DiscoverStageScanning, 0)
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		found    *PairingServiceInfo
		probed   int64
		reported int32
	)
	total := int(last) - int(first) + 1
	// Closes when the winner is decided; every goroutine path selects on it
	// so in-flight work stops promptly.
	won := make(chan struct{})
	sem := make(chan struct{}, rpDialConcurrency)
	// Handshake probes are slower than dials; bound them separately so dials
	// keep flowing ahead of the probes.
	probeSem := make(chan struct{}, rpProbeConcurrency)

dialLoop:
	for port := int(first); port <= int(last); port++ {
		select {
		case <-won:
			break dialLoop
		case <-ctx.Done():
			break dialLoop
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(p uint16) {
			defer wg.Done()
			defer func() { <-sem }()

			dialer := net.Dialer{Timeout: rpDialTimeout}
			conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(int(p))))
			if err != nil {
				n := atomic.AddInt64(&probed, 1)
				reportPct(report, &reported, int(n), total)
				return
			}
			_ = conn.Close()

			select {
			case <-won:
				return
			case probeSem <- struct{}{}:
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-probeSem }()

				si, perr := ProbePairingService(ctx, ip, p, rpProbeTimeout)
				n := atomic.AddInt64(&probed, 1)
				reportPct(report, &reported, int(n), total)
				if perr != nil || si == nil || !pick(si) {
					return
				}
				mu.Lock()
				if found == nil {
					found = si
					close(won)
				}
				mu.Unlock()
			}()
		}(uint16(port))
	}
	wg.Wait()

	if ctx.Err() != nil && found == nil {
		return nil
	}
	return found
}

// reportPct throttles scanning progress to 5% steps; report may be nil.
func reportPct(report func(string, int), reported *int32, done, total int) {
	if report == nil || total <= 0 {
		return
	}
	pct := done * 100 / total
	prev := atomic.LoadInt32(reported)
	if (int32(pct) >= prev+5 || done == total) && atomic.CompareAndSwapInt32(reported, prev, int32(pct)) {
		report(DiscoverStageScanning, pct)
	}
}

// portMatcher reports whether a handshaked service is the wanted one.
type portMatcher func(*PairingServiceInfo) bool

// skipMDNSForIP reports whether the unicast mDNS fast path is pointless for
// ip. Overlay VPNs (Tailscale uses 100.x.x.x CGNAT addresses) cannot answer
// link-local mDNS, so the 1.5s query would always time out before the scan.
func skipMDNSForIP(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	return err == nil && addr.Is4() && addr.As4()[0] == 100
}

// discoverPortByService is the testable core shared by the manual-pairing and
// tunnel discovery: it tries the mDNS fast path (unless skipped), then the
// pipelined ascending scan over [first,last], returning the first port that
// satisfies pick.
func discoverPortByService(ctx context.Context, ip, fqdn string, first, last uint16, skipMDNS bool, pick portMatcher, progress DiscoveryProgress) (port uint16, info *PairingServiceInfo, err error) {
	report := func(stage string, pct int) {
		if progress != nil {
			progress(stage, pct)
		}
	}

	if !skipMDNS && !skipMDNSForIP(ip) {
		report(DiscoverStageMDNS, 0)
		if p, ok := discoverPortMDNS(ctx, ip, fqdn, rpMDNSTimeout); ok {
			if si, perr := ProbePairingService(ctx, ip, p, rpProbeTimeout); perr == nil && pick(si) {
				report(DiscoverStageFound, 100)
				return p, si, nil
			}
		}
	}

	if si := scanProbedPorts(ctx, ip, first, last, pick, progress); si != nil {
		report(DiscoverStageFound, 100)
		return si.Port, si, nil
	}
	if ctx.Err() != nil {
		return 0, nil, ctx.Err()
	}

	report(DiscoverStageFailed, 100)
	return 0, nil, nil
}

// discoverManualPairingPort is the manual-pairing variant: it stops at the
// first service that allows the PIN pair-setup flow.
func discoverManualPairingPort(ctx context.Context, ip string, first, last uint16, skipMDNS bool, progress DiscoveryProgress) (uint16, *PairingServiceInfo, error) {
	return discoverPortByService(ctx, ip, rpManualPairingFQDN, first, last, skipMDNS, func(si *PairingServiceInfo) bool {
		return si.PairSetup
	}, progress)
}

// DiscoverManualPairingPort finds the manual-pairing port for a manually
// specified IP: mDNS unicast fast path (unless skipped for overlay IPs), then
// a pipelined ascending scan that stops at the first pair-setup service.
func DiscoverManualPairingPort(ctx context.Context, ip string, progress DiscoveryProgress) (pairPort uint16, info *PairingServiceInfo, err error) {
	pairPort, info, err = discoverManualPairingPort(ctx, ip, rpPortFirst, rpPortLast, false, progress)
	if err != nil {
		return 0, nil, err
	}
	if pairPort == 0 {
		return 0, nil, ErrPairingPortNotFound
	}
	return pairPort, info, nil
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

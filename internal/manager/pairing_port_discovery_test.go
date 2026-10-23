package manager

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"strings"
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
	pairPort, _, info, err := discoverManualPairingPort(ctx, "127.0.0.1", port, port, true, func(stage string, _ int) {
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
	_, _, _, err := discoverManualPairingPort(ctx, "127.0.0.1", 1, 1, true, nil)
	if err != ErrPairingPortNotFound {
		t.Fatalf("want ErrPairingPortNotFound, got %v", err)
	}
}

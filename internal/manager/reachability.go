package manager

import (
	"fmt"
	"math/rand"
	"net"
	"net/netip"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// defaultProbeHostReachability sends one ICMP echo request to the address
// and waits up to timeout for the matching reply. No port is involved on
// purpose: iPhones (unlike Apple TVs) never expose the remote-pairing port
// over the network, and pairing itself only starts after registration, so
// an echo request is the only address-wide reachability test. The raw
// socket needs CAP_NET_RAW, which Docker's default capability set includes
// (no privileged mode required) — only a setup that drops all capabilities
// would need cap_add: [NET_RAW].
func defaultProbeHostReachability(ip string, timeout time.Duration) error {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return err
	}

	var conn net.PacketConn
	if addr.Is4() {
		conn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	} else {
		conn, err = icmp.ListenPacket("ip6:icmp", "::")
	}
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	payload := []byte("atvloadly reachability probe")
	echoID := rand.Intn(1 << 16)
	message := icmp.Message{
		Code: 0,
		Body: &icmp.Echo{
			ID:   echoID,
			Seq:  1,
			Data: payload,
		},
	}
	if addr.Is4() {
		message.Type = ipv4.ICMPTypeEcho
	} else {
		message.Type = ipv6.ICMPTypeEchoRequest
	}
	wire, err := message.Marshal(nil)
	if err != nil {
		return err
	}

	dest := &net.IPAddr{IP: net.IP(addr.AsSlice()), Zone: addr.Zone()}
	if _, err := conn.WriteTo(wire, dest); err != nil {
		return err
	}

	buf := make([]byte, 1500)
	deadline := time.Now().Add(timeout)
	for {
		if err := conn.SetReadDeadline(deadline); err != nil {
			return err
		}
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return fmt.Errorf("no echo reply from %s within %s", ip, timeout)
			}
			return err
		}
		// The raw socket sees every ICMP packet on the host; only our own
		// echo reply, recognized by ID and payload, proves reachability.
		msg, parseErr := icmp.ParseMessage(ianaProtocol(addr.Is4()), buf[:n])
		if parseErr != nil {
			continue
		}
		echo, ok := msg.Body.(*icmp.Echo)
		if !ok || echo.ID != echoID || string(echo.Data) != string(payload) {
			continue
		}
		if addr.Is4() && msg.Type == ipv4.ICMPTypeEchoReply {
			return nil
		}
		if !addr.Is4() && msg.Type == ipv6.ICMPTypeEchoReply {
			return nil
		}
	}
}

func ianaProtocol(v4 bool) int {
	if v4 {
		return 1
	}
	return 58
}

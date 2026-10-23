package manager

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/bitxeno/atvloadly/internal/model"
	plist "howett.net/plist"
)

// usbmuxdSocketPath is the system usbmuxd socket. On macOS the pair records
// below /var/db/lockdown belong to _usbmuxd and cannot be read by ordinary
// users, but usbmuxd serves them to any local client through this socket.
var usbmuxdSocketPath = "/var/run/usbmuxd"

const (
	usbmuxdTimeout        = 5 * time.Second
	usbmuxdHeaderSize     = 16
	usbmuxdVersionPlist   = 1
	usbmuxdMessagePlist   = 8
	usbmuxdMaxMessageSize = 1 << 20
)

type usbmuxdClient struct {
	conn net.Conn
	tag  uint32
}

type usbmuxdRequest struct {
	MessageType         string `plist:"MessageType"`
	PairRecordID        string `plist:"PairRecordID,omitempty"`
	ClientVersionString string `plist:"ClientVersionString"`
	ProgName            string `plist:"ProgName"`
}

type usbmuxdDeviceList struct {
	DeviceList []struct {
		Properties struct {
			SerialNumber string `plist:"SerialNumber"`
		} `plist:"Properties"`
	} `plist:"DeviceList"`
}

type usbmuxdPairRecordResponse struct {
	PairRecordData []byte `plist:"PairRecordData"`
}

// usbmuxdPairRecord decodes only the Wi-Fi address; the record also holds
// host private keys, which are never decoded or kept.
type usbmuxdPairRecord struct {
	WiFiMACAddress string `plist:"WiFiMACAddress"`
}

// loadLockdownDevicesFromUsbmuxd returns the paired devices known to usbmuxd,
// keyed by lowercase Wi-Fi MAC address like loadLockdownDevices. Devices
// without a pair record or a Wi-Fi address cannot be matched to mobdev2
// announcements and are skipped.
func loadLockdownDevicesFromUsbmuxd() (map[string]model.LockdownDevice, error) {
	client, err := dialUsbmuxd()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var list usbmuxdDeviceList
	if err := client.call(usbmuxdRequest{MessageType: "ListDevices"}, &list); err != nil {
		return nil, fmt.Errorf("usbmuxd ListDevices: %w", err)
	}

	devices := map[string]model.LockdownDevice{}
	seen := map[string]bool{}
	for _, entry := range list.DeviceList {
		udid := entry.Properties.SerialNumber
		// A device attached over both USB and Wi-Fi is listed twice.
		if udid == "" || seen[udid] {
			continue
		}
		seen[udid] = true

		var resp usbmuxdPairRecordResponse
		if err := client.call(usbmuxdRequest{MessageType: "ReadPairRecord", PairRecordID: udid}, &resp); err != nil {
			return nil, fmt.Errorf("usbmuxd ReadPairRecord: %w", err)
		}
		if len(resp.PairRecordData) == 0 {
			continue // not paired with this host
		}
		var record usbmuxdPairRecord
		if _, err := plist.Unmarshal(resp.PairRecordData, &record); err != nil {
			return nil, fmt.Errorf("decode pair record of %s: %w", udid, err)
		}
		macAddr := strings.ToLower(record.WiFiMACAddress)
		if macAddr == "" {
			continue
		}
		devices[macAddr] = model.LockdownDevice{Name: udid, WiFiMACAddress: record.WiFiMACAddress}
	}
	return devices, nil
}

func dialUsbmuxd() (*usbmuxdClient, error) {
	conn, err := net.DialTimeout("unix", usbmuxdSocketPath, usbmuxdTimeout)
	if err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Now().Add(usbmuxdTimeout)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &usbmuxdClient{conn: conn}, nil
}

func (c *usbmuxdClient) Close() {
	_ = c.conn.Close()
}

// call sends one plist request and decodes the reply. Requests on a
// connection are sequential, so the reply is the next message.
func (c *usbmuxdClient) call(req usbmuxdRequest, resp any) error {
	req.ClientVersionString = "atvloadly"
	req.ProgName = "atvloadly"
	body, err := plist.Marshal(req, plist.XMLFormat)
	if err != nil {
		return err
	}
	c.tag++
	header := make([]byte, usbmuxdHeaderSize)
	binary.LittleEndian.PutUint32(header[0:], uint32(usbmuxdHeaderSize+len(body)))
	binary.LittleEndian.PutUint32(header[4:], usbmuxdVersionPlist)
	binary.LittleEndian.PutUint32(header[8:], usbmuxdMessagePlist)
	binary.LittleEndian.PutUint32(header[12:], c.tag)
	if _, err := c.conn.Write(append(header, body...)); err != nil {
		return err
	}

	if _, err := io.ReadFull(c.conn, header); err != nil {
		return err
	}
	size := binary.LittleEndian.Uint32(header[0:])
	if size < usbmuxdHeaderSize || size > usbmuxdMaxMessageSize {
		return fmt.Errorf("invalid usbmuxd message size %d", size)
	}
	reply := make([]byte, size-usbmuxdHeaderSize)
	if _, err := io.ReadFull(c.conn, reply); err != nil {
		return err
	}
	_, err = plist.Unmarshal(reply, resp)
	return err
}

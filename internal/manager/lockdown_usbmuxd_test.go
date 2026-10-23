package manager

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/bitxeno/atvloadly/internal/model"
	plist "howett.net/plist"
)

// fakeUsbmuxd serves ListDevices and ReadPairRecord like the macOS usbmuxd.
// udids returns the attached devices at each request; records maps a UDID to
// its pair record, and a missing UDID has no record.
func fakeUsbmuxd(t *testing.T, udids func() []string, records map[string]map[string]any) {
	t.Helper()
	// Unix socket paths are limited to 104 bytes on macOS; t.TempDir is too long.
	dir, err := os.MkdirTemp("", "mux")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "s")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	previous := usbmuxdSocketPath
	usbmuxdSocketPath = path
	t.Cleanup(func() { usbmuxdSocketPath = previous })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveFakeUsbmuxd(conn, udids, records)
		}
	}()
}

func serveFakeUsbmuxd(conn net.Conn, udids func() []string, records map[string]map[string]any) {
	defer func() { _ = conn.Close() }()
	header := make([]byte, usbmuxdHeaderSize)
	for {
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		body := make([]byte, binary.LittleEndian.Uint32(header)-usbmuxdHeaderSize)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		var req usbmuxdRequest
		if _, err := plist.Unmarshal(body, &req); err != nil {
			return
		}
		var reply any
		switch req.MessageType {
		case "ListDevices":
			list := []any{}
			for _, udid := range udids() {
				list = append(list, map[string]any{
					"MessageType": "Attached",
					"Properties":  map[string]any{"SerialNumber": udid, "ConnectionType": "Network"},
				})
			}
			reply = map[string]any{"DeviceList": list}
		case "ReadPairRecord":
			record, ok := records[req.PairRecordID]
			if !ok {
				reply = map[string]any{"MessageType": "Result", "Number": 2}
				break
			}
			data, _ := plist.Marshal(record, plist.XMLFormat)
			reply = map[string]any{"PairRecordData": data}
		}
		data, _ := plist.Marshal(reply, plist.XMLFormat)
		out := make([]byte, usbmuxdHeaderSize)
		binary.LittleEndian.PutUint32(out[0:], uint32(usbmuxdHeaderSize+len(data)))
		binary.LittleEndian.PutUint32(out[4:], usbmuxdVersionPlist)
		binary.LittleEndian.PutUint32(out[8:], usbmuxdMessagePlist)
		copy(out[12:], header[12:16])
		if _, err := conn.Write(append(out, data...)); err != nil {
			return
		}
	}
}

func TestLoadLockdownDevicesFromUsbmuxd(t *testing.T) {
	fakeUsbmuxd(t,
		// paired is attached over USB and Wi-Fi, so usbmuxd lists it twice.
		func() []string { return []string{"paired", "paired", "unpaired", "no-wifi"} },
		map[string]map[string]any{
			"paired":  {"WiFiMACAddress": "AA:BB:CC:DD:EE:FF", "HostPrivateKey": []byte("secret")},
			"no-wifi": {"HostID": "host"},
		})

	devices, err := loadLockdownDevicesFromUsbmuxd()
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 {
		t.Fatalf("devices = %v, want only the paired device", devices)
	}
	dev, ok := devices["aa:bb:cc:dd:ee:ff"]
	if !ok || dev.Name != "paired" {
		t.Fatalf("paired device not keyed by lowercase MAC: %v", devices)
	}
}

func TestLoadLockdownDevicesFromUsbmuxdUnavailable(t *testing.T) {
	previous := usbmuxdSocketPath
	usbmuxdSocketPath = filepath.Join(t.TempDir(), "missing")
	t.Cleanup(func() { usbmuxdSocketPath = previous })

	if _, err := loadLockdownDevicesFromUsbmuxd(); err == nil {
		t.Fatal("a missing usbmuxd socket must be reported")
	}
}

// Listing the devices on every device list request must not query usbmuxd
// for remote pairable devices, and must query it at most once otherwise.
func TestReloadDevicesReadsPairRecordsOnce(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("pair records are read from usbmuxd only on macOS")
	}
	var lists atomic.Int32
	fakeUsbmuxd(t, func() []string { lists.Add(1); return nil }, nil)

	dm := newDeviceManager()
	dm.SaveDevice(model.Device{ID: "remote", Status: model.Pairable, Connection: model.DeviceConnectionRemote})
	dm.ReloadDevices()
	if n := lists.Load(); n != 0 {
		t.Fatalf("usbmuxd queried %d times for a device without a MAC address", n)
	}

	dm.SaveDevice(model.Device{ID: "a", Status: model.Pairable, MacAddr: "aa:aa:aa:aa:aa:aa"})
	dm.SaveDevice(model.Device{ID: "b", Status: model.Pairable, MacAddr: "bb:bb:bb:bb:bb:bb"})
	dm.ReloadDevices()
	if n := lists.Load(); n != 1 {
		t.Fatalf("usbmuxd queried %d times, want once per reload", n)
	}
}

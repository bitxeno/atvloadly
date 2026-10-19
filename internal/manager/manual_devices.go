package manager

import (
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
)

// manualDevicesFile persists manually added devices so they survive a
// restart. Avahi-discovered devices reappear on their own; manual devices
// have no discovery mechanism to recreate them.
const manualDevicesFile = "manual_devices.json"

var manualMu sync.Mutex

// probeDeviceInfo is replaceable so unit tests never invoke the plumesign
// binary.
var probeDeviceInfo = defaultProbeDeviceInfo

func manualDevicesPath() string {
	return filepath.Join(app.Config.Server.DataDir, manualDevicesFile)
}

func readManualDevices() []model.Device {
	data, err := os.ReadFile(manualDevicesPath())
	if err != nil {
		return nil
	}

	devices := []model.Device{}
	if err := utils.FromJSON(data, &devices); err != nil {
		log.Warnf("Failed to parse %s: %v", manualDevicesFile, err)
		return nil
	}
	return devices
}

func writeManualDevices(devices []model.Device) error {
	data := utils.ToJSON(devices).Bytes()

	path := manualDevicesPath()
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// defaultProbeDeviceInfo queries the device over the network with
// plumesign device-info. The device reports its UDID unauthenticated, so
// the returned info carries enough data to register the device without
// mDNS discovery.
func defaultProbeDeviceInfo(ip string, port uint16) (*model.DeviceInfo, error) {
	cmd := exec.Command("plumesign", "device-info", "--ip", ip, "--port", fmt.Sprintf("%d", port)).
		WithTimeout(10 * time.Second)

	data, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("device not reachable at %s:%d: %s%s", ip, port, string(data), err.Error())
	}

	info := parseDeviceInfoOutput(string(data), "")
	if info.UniqueDeviceID == "" {
		return nil, fmt.Errorf("device at %s:%d did not report a UDID. output: %s", ip, port, string(data))
	}
	return info, nil
}

// AddManualDevice registers a device that avahi cannot discover, typically
// reached over a mesh VPN such as Tailscale. The device is probed over the
// network first: an unreachable or unrecognized address is rejected and
// never registered. The device starts Pairable, so the normal pairing flow
// upgrades it to Paired.
func (dm *DeviceManager) AddManualDevice(ip string, port uint16, name string) (*model.Device, error) {
	if ip == "" || port == 0 {
		return nil, fmt.Errorf("ip and port are required")
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil || !isLanAddr(addr) {
		return nil, fmt.Errorf("ip %s is not a private LAN or CGNAT address", ip)
	}

	info, err := probeDeviceInfo(ip, port)
	if err != nil {
		return nil, err
	}

	udid := info.UniqueDeviceID
	if name == "" {
		if info.DeviceName == "" {
			name = fmt.Sprintf("Manual device (%s)", ip)
		} else {
			name = info.DeviceName
		}
	}

	device := model.Device{
		ID:          utils.Md5(udid),
		Name:        name,
		IP:          ip,
		Port:        port,
		UDID:        udid,
		Connection:  model.DeviceConnectionRemote,
		Status:      model.Pairable,
		DiscoveryAt: time.Now(),
	}
	if info.DeviceClass != "" {
		device.DeviceClass = info.DeviceClass
	} else {
		device.ParseDeviceClass()
	}
	device.ProductType = info.ProductType
	device.ProductVersion = info.ProductVersion
	device.DeveloperModeStatus = info.DeveloperModeStatus
	device.PersonalizedImageMounted = info.PersonalizedImageMounted

	dm.DeleteDeviceByUDID(udid)
	dm.SaveDevice(device)

	manualMu.Lock()
	entries := readManualDevices()
	kept := entries[:0]
	for _, entry := range entries {
		if entry.UDID != udid {
			kept = append(kept, entry)
		}
	}
	kept = append(kept, device)
	if err := writeManualDevices(kept); err != nil {
		log.Err(err).Msgf("Failed to persist manual device %s:", udid)
	}
	manualMu.Unlock()

	log.Infof("Manual device added: %s (%s) at %s:%d", device.Name, udid, ip, port)
	dm.onDeviceConnected(device)
	return &device, nil
}

// LoadManualDevices restores the manually added devices registered by
// AddManualDevice. It runs at manager startup, because no discovery
// mechanism announces these devices.
func (dm *DeviceManager) LoadManualDevices() {
	manualMu.Lock()
	entries := readManualDevices()
	manualMu.Unlock()

	for _, device := range entries {
		if device.ID == "" || device.UDID == "" || device.IP == "" {
			continue
		}
		dm.SaveDevice(device)
		log.Debugf("Manual device restored: %s (%s)", device.Name, device.UDID)
	}
}

// DeleteManualDevice removes a manually added device from the registry and
// the persistence file. Deleting a device that was discovered by avahi
// instead reports not registered: avahi would re-add it.
func (dm *DeviceManager) DeleteManualDevice(id string) (bool, error) {
	manualMu.Lock()
	entries := readManualDevices()
	manualMu.Unlock()

	found := false
	kept := entries[:0]
	for _, entry := range entries {
		if entry.ID == id {
			found = true
			continue
		}
		kept = append(kept, entry)
	}

	manualMu.Lock()
	err := writeManualDevices(kept)
	manualMu.Unlock()
	if err != nil {
		return found, err
	}

	dm.devices.Delete(id)
	return found, nil
}

// PairCompleted marks a device Paired after the pairing flow succeeded.
// Over mDNS the device's own announcement re-registers it faster than this
// function reads the success output; for manual devices there is no
// announcement, so this is the only path that upgrades the status.
func (dm *DeviceManager) PairCompleted(id string) {
	device, ok := dm.GetDeviceByID(id)
	if !ok {
		return
	}

	device.Status = model.Paired
	if info, err := GetDeviceInfo(device); err == nil {
		dm.AppendProductInfo(device, *info)
	}
	dm.SaveDevice(*device)

	manualMu.Lock()
	entries := readManualDevices()
	updated := false
	for i, entry := range entries {
		if entry.UDID == device.UDID {
			entries[i] = *device
			updated = true
			break
		}
	}
	if updated {
		_ = writeManualDevices(entries)
	}
	manualMu.Unlock()

	dm.onDeviceConnected(*device)
}

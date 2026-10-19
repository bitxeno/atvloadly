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
// defaultProbeDeviceInfo queries the device information with the pairing
// file produced by the pairing flow: plumesign device-info over the network
// is refused without one.
func defaultProbeDeviceInfo(ip string, port uint16, pairingFile string) (*model.DeviceInfo, error) {
	cmd := exec.Command("plumesign", "device-info", "--ip", ip, "--port", fmt.Sprintf("%d", port), "-f", pairingFile).
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

// AddManualDevice registers a device address that avahi cannot discover,
// typically reached over a mesh VPN such as Tailscale. No network probe
// runs: plumesign device-info over the network requires a pairing file,
// which only exists after the pairing flow succeeds for this host. The
// device therefore starts Pairable with an empty UDID; PairCompleted fills
// in the rest once the pairing flow succeeds.
func (dm *DeviceManager) AddManualDevice(ip string, port uint16, name string) (*model.Device, error) {
	if ip == "" || port == 0 {
		return nil, fmt.Errorf("ip and port are required")
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil || !isLanAddr(addr) {
		return nil, fmt.Errorf("ip %s is not a private LAN or CGNAT address", ip)
	}

	if name == "" {
		name = fmt.Sprintf("Manual device (%s)", ip)
	}

	device := model.Device{
		ID:          utils.Md5(fmt.Sprintf("%s:%d", ip, port)),
		Name:        name,
		IP:          ip,
		Port:        port,
		Connection:  model.DeviceConnectionRemote,
		Status:      model.Pairable,
		DiscoveryAt: time.Now(),
	}
	device.ParseDeviceClass()

	dm.SaveDevice(device)

	manualMu.Lock()
	entries := readManualDevices()
	kept := entries[:0]
	for _, entry := range entries {
		if entry.ID != device.ID {
			kept = append(kept, entry)
		}
	}
	kept = append(kept, device)
	if err := writeManualDevices(kept); err != nil {
		log.Err(err).Msgf("Failed to persist manual device %s:", device.ID)
	}
	manualMu.Unlock()

	log.Infof("Manual device added: %s at %s:%d", device.Name, ip, port)
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
		// The UDID stays empty until the pairing flow succeeds, so devices
		// may be restored before they are Paired.
		if device.ID == "" || device.IP == "" {
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
// announcement, so this is the only path that upgrades the status. The
// freshly written pairing files supply the network probe, which fills the
// UDID and product details the manual registration could not know.
func (dm *DeviceManager) PairCompleted(id string) {
	device, ok := dm.GetDeviceByID(id)
	if !ok {
		return
	}

	pairingFile, info, err := pairFileDeviceInfo(*device)
	if err != nil {
		// Pairing succeeded but the records are not usable: leave the
		// device Pairable so the pairing flow can run again.
		log.Err(err).Msgf("Pairing completed for %s, but the device could not be queried:", device.Name)
		return
	}
	log.Infof("Pairing record for %s: %s", device.Name, pairingFile)

	if device.UDID == "" {
		device.UDID = info.UniqueDeviceID
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
	device.Status = model.Paired
	dm.SaveDevice(*device)

	manualMu.Lock()
	entries := readManualDevices()
	updated := false
	for i, entry := range entries {
		if entry.ID == device.ID {
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

// pairFileDeviceInfo probes the device over the network with the pairing
// files written by the pairing flow. The plumesign store keeps records for
// every device this host has paired with, so each candidate file is tried
// until one identifies the device.
func pairFileDeviceInfo(device model.Device) (string, *model.DeviceInfo, error) {
	candidates, err := filepath.Glob(filepath.Join(app.RemotePairingDir(), "*.plist"))
	if err != nil {
		return "", nil, fmt.Errorf("pairing files unavailable: %v", err)
	}
	if len(candidates) == 0 {
		return "", nil, fmt.Errorf("no pairing files in %s", app.RemotePairingDir())
	}

	var lastErr error
	for _, file := range candidates {
		info, err := probeDeviceInfo(device.IP, device.Port, file)
		if err == nil && info != nil && info.UniqueDeviceID != "" {
			return file, info, nil
		}
		if err == nil {
			err = fmt.Errorf("pairing file %s did not yield a UDID", filepath.Base(file))
		}
		lastErr = err
	}
	return "", nil, lastErr
}

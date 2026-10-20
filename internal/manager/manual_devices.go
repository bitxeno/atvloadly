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

// ManualDeviceID returns the device ID assigned to a manually registered
// address. Pairing-file imports use it to upgrade the device at this
// address after the imported record makes it queryable.
func ManualDeviceID(ip string, port uint16) string {
	return utils.Md5(fmt.Sprintf("%s:%d", ip, port))
}

// AddManualDevice registers a device address that avahi cannot discover,
// typically reached over a mesh VPN such as Tailscale. No network probe
// runs: plumesign device-info over the network requires a pairing file,
// which only exists after the pairing flow succeeds for this host. The
// device therefore starts Pairable with an empty UDID; PairCompleted fills
// in the rest once the pairing flow succeeds, including the install-port
// scan that needs the freshly written pairing record.
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
		ID:          ManualDeviceID(ip, port),
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

// UpdateManualInstallPort records the install service port chosen on the
// install page so tunnel operations (installs, probes, screenshots) target
// the regular remote-pairing service. The value is persisted for manual
// devices; discovered devices keep it in memory until their next
// announcement refresh replaces them.
func (dm *DeviceManager) UpdateManualInstallPort(dev *model.Device, port uint16) {
	dev.InstallPort = port
	dm.SaveDevice(*dev)

	manualMu.Lock()
	defer manualMu.Unlock()
	entries := readManualDevices()
	for i, entry := range entries {
		if entry.ID == dev.ID {
			entries[i] = *dev
			_ = writeManualDevices(entries)
			break
		}
	}
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

	var udid, productType, productVersion string
	var developerModeStatus, personalizedImageMounted bool

	pairingFile, info, err := pairFileDeviceInfo(*device)
	if err == nil {
		log.Infof("Pairing record for %s: %s", device.Name, pairingFile)

		udid = info.UniqueDeviceID
		if info.DeviceClass != "" {
			device.DeviceClass = info.DeviceClass
		} else {
			device.ParseDeviceClass()
		}
		productType = info.ProductType
		productVersion = info.ProductVersion
		developerModeStatus = info.DeveloperModeStatus
		personalizedImageMounted = info.PersonalizedImageMounted
	} else {
		// tvOS 26 answers the device-info listener handshake with a payload
		// plumesign cannot reconcile ("missing port in createListener
		// response"), even though the pairing record is valid. The peer
		// record written next to the pairing file carries the device's
		// identity, so accept the newest one written by the pair that just
		// completed.
		peer, ok := newestPairingPeerRecord()
		if !ok {
			// Pairing succeeded but the records are not usable: leave the
			// device Pairable so the pairing flow can run again.
			log.Err(err).Msgf("Pairing completed for %s, but the device could not be queried:", device.Name)
			return
		}
		log.Warnf("Pairing record for %s: %s via peer record (device-info unavailable: %v)", device.Name, peer.RemotePairingUDID, err)

		udid = peer.RemotePairingUDID
		productType = peer.Model
		if class := peer.GetDeviceClass(); class != "" {
			device.DeviceClass = class
		} else {
			device.ParseDeviceClass()
		}
	}

	if device.UDID == "" {
		device.UDID = udid
	}
	device.ProductType = productType
	device.ProductVersion = productVersion
	device.DeveloperModeStatus = developerModeStatus
	device.PersonalizedImageMounted = personalizedImageMounted
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

	// The pairing record just written makes the install-service handshake
	// possible; let it find the install port while the user heads to the
	// install page, so manual devices need no repeated port entry there.
	if updated && device.InstallPort == 0 {
		go dm.scanAndStoreInstallPort(device.ID, device.IP)
	}
}

// scanAndStoreInstallPort detects the device's install-service port with the
// handshake-validated port scan and records it for the device. Runs as a
// background step after pairing completes; failure is non-fatal because the
// install page offers the same scan manually.
func (dm *DeviceManager) scanAndStoreInstallPort(id string, ip string) {
	port, _, err := ScanInstallPort(ip)
	if err != nil || port == 0 {
		log.Warnf("Install port scan for %s found nothing usable: %v", ip, err)
		return
	}
	if device, ok := dm.GetDeviceByID(id); ok {
		dm.UpdateManualInstallPort(device, port)
		log.Infof("Install port scan for %s: %d recorded", ip, port)
	}
}

// ImportPairedDevice marks a device Paired from an imported pairing record,
// which plumesign check pairing has already validated over the given port
// and saved into its store. The record import is the way to add a device
// whose pairing already happened somewhere else: no pair port and no portal
// pairing is needed, only the tunnel-capable service port the record can be
// queried over. An existing registration for this address is upgraded;
// without one, a new paired device is created from the probed identity.
func (dm *DeviceManager) ImportPairedDevice(ip string, port uint16) error {
	probe := model.Device{
		IP:          ip,
		Port:        port,
		InstallPort: port,
		Connection:  model.DeviceConnectionRemote,
	}

	var udid, productType, productVersion, name string
	var developerModeStatus, personalizedImageMounted bool
	if _, info, err := pairFileDeviceInfo(probe); err == nil {
		udid = info.UniqueDeviceID
		productType = info.ProductType
		productVersion = info.ProductVersion
		developerModeStatus = info.DeveloperModeStatus
		personalizedImageMounted = info.PersonalizedImageMounted
		name = info.DeviceName
	} else {
		// tvOS 26 can refuse the device-info listener handshake even though
		// the imported record is valid; the peer record check pairing wrote
		// alongside it carries the device's identity.
		peer, ok := newestPairingPeerRecord()
		if !ok {
			log.Err(err).Msgf("Imported pairing record for %s validated, but the device could not be queried:", ip)
			return err
		}
		log.Warnf("Imported pairing record for %s: %s via peer record (device-info unavailable: %v)", ip, peer.RemotePairingUDID, err)

		udid = peer.RemotePairingUDID
		productType = peer.Model
		name = peer.Name
	}

	manualMu.Lock()
	entries := readManualDevices()
	idx := -1
	for i, entry := range entries {
		if entry.IP == ip && (entry.Port == port || entry.InstallPort == port) {
			idx = i
			break
		}
	}

	device := model.Device{
		IP:                        ip,
		Port:                      port,
		InstallPort:               port,
		Connection:                model.DeviceConnectionRemote,
		Status:                    model.Paired,
		UDID:                      udid,
		ProductType:               productType,
		ProductVersion:            productVersion,
		DeveloperModeStatus:       developerModeStatus,
		PersonalizedImageMounted:  personalizedImageMounted,
	}
	device.ParseDeviceClass()
	if name != "" {
		device.Name = name
	}
	if idx >= 0 {
		// Keep the entry's own identifier so a stable address keeps one
		// registry record; only the device details refresh.
		device.ID = entries[idx].ID
		if entries[idx].Name != "" {
			device.Name = entries[idx].Name
		}
		device.MacAddr = entries[idx].MacAddr
		entries[idx] = device
	} else {
		device.ID = ManualDeviceID(ip, port)
		device.DiscoveryAt = time.Now()
		entries = append(entries, device)
	}
	err := writeManualDevices(entries)
	manualMu.Unlock()
	if err != nil {
		return err
	}

	dm.SaveDevice(device)
	dm.onDeviceConnected(device)
	return nil
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
		info, err := probeDeviceInfo(device.IP, device.ServicePort(), file)
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

// newestPairingPeerRecord reads the peer record plumesign writes next to
// every pairing file. The pair that just completed wrote its own record
// last, so the newest entry identifies the freshly paired device. It is
// the only identity source when the network probe cannot run over the
// routed address of the device.
func newestPairingPeerRecord() (*model.RemoteDevice, bool) {
	files, err := filepath.Glob(filepath.Join(app.RemotePairingDir(), "*.json"))
	if err != nil || len(files) == 0 {
		return nil, false
	}

	var newest *model.RemoteDevice
	var newestTime time.Time
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var peer model.RemoteDevice
		if utils.FromJSON(data, &peer) != nil || peer.RemotePairingUDID == "" {
			continue
		}
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if newest == nil || info.ModTime().After(newestTime) {
			newest = &peer
			newestTime = info.ModTime()
		}
	}
	return newest, newest != nil
}

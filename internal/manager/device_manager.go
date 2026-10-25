package manager

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/utils"
)

var deviceManager = newDeviceManager()

// pairingCheckInterval throttles remote-pairing mDNS events for the same
// device. Some iPhones re-announce and withdraw the _remotepairing service
// every few seconds, and each Add event would otherwise spawn a slow
// `plumesign check find-pairing` subprocess inside the avahi event loop.
// Without throttling the event loop falls behind, the avahi signal channel
// backs up, and godbus leaks one goroutine per undelivered signal.
const pairingCheckInterval = 60 * time.Second

// cgnatPrefix is the RFC 6598 shared address space (100.64.0.0/10) that
// mesh-VPN overlays such as Tailscale assign. Devices reachable only through
// an overlay announce themselves from this range; plumesign connects over it
// fine, so it is accepted alongside RFC 1918 LAN space.
var cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")

// isLanAddr reports whether addr is a private IPv4 address that the
// plumesign tooling can connect to. Global IPv6 addresses (e.g. an
// ISP-delegated prefix announced via AAAA) and public addresses are
// rejected: LAN devices are reached over private IPv4 or over an overlay
// network such as Tailscale (CGNAT 100.64.0.0/10).
func isLanAddr(addr netip.Addr) bool {
	if !addr.IsValid() || !addr.Is4() {
		return false
	}
	return addr.IsPrivate() || cgnatPrefix.Contains(addr)
}

// IsManualDeviceAddr reports whether ip is an address a manually added device
// may use: a private LAN IPv4 or the CGNAT range mesh VPNs assign. Manual
// entry triggers a full-range TCP scan, so rejecting public addresses keeps
// the portal from being turned into a scanner for arbitrary hosts.
func IsManualDeviceAddr(ip string) bool {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	return err == nil && isLanAddr(addr)
}

// pairingCheck identifies one in-flight check so an older check cannot
// remove the cancellation function belonging to a newer check.
type pairingCheck struct {
	cancel context.CancelFunc
}

type DeviceManager struct {
	devices sync.Map
	// manualDevices holds devices paired by a manually entered IP (e.g. over
	// Tailscale) that mDNS cannot discover. They are loaded from the pairing
	// files and merged into GetDevices, and survive Scan() (which only clears
	// mDNS devices).
	manualDevices sync.Map
	// manualPortMu/manualPortLocks serialize tunnel-port discovery per manual
	// device so a concurrent install and refresh do not scan simultaneously.
	manualPortMu    sync.Mutex
	manualPortLocks map[string]*sync.Mutex
	// probeMu guards probeStates: per-UDID liveness tracking for mDNS devices
	// that also have a manual (pairing-file) record. A device that fails
	// repeated TCP probes is marked stale and hidden from GetDevices so its
	// manual entry takes over.
	probeMu              sync.Mutex
	probeStates          map[string]*deviceProbeState
	ctx                  context.Context
	cancel               context.CancelFunc
	mu                   sync.Mutex
	onDeviceConnected    func(device model.Device) // 设备连接时的回调函数
	onDeviceDisconnected func(device model.Device) // 设备断开时的回调函数

	// pairingCheckedAt remembers the last time a remote-pairing event for a
	// given device identifier was processed, so duplicate mDNS events within
	// pairingCheckInterval are ignored instead of re-running the slow check.
	pairingMu        sync.Mutex
	pairingCheckedAt map[string]time.Time
	// pairingCancel tracks the in-flight remote-pairing check for each
	// device identifier, so a disconnect (Remove event) can cancel the
	// slow plumesign subprocess immediately and release the goroutine.
	pairingCancel map[string]*pairingCheck
	// remotePairingServices connects the service instance name supplied by
	// Avahi's ItemRemove signal to the identifier supplied only by ItemNew.
	// Both pairingCancel and pairingCheckedAt are keyed by the identifier.
	remotePairingServices map[string]string
}

func newDeviceManager() *DeviceManager {
	return &DeviceManager{
		onDeviceConnected:     func(device model.Device) {},
		onDeviceDisconnected:  func(device model.Device) {},
		pairingCheckedAt:      make(map[string]time.Time),
		pairingCancel:         make(map[string]*pairingCheck),
		remotePairingServices: make(map[string]string),
		manualPortLocks:       make(map[string]*sync.Mutex),
		probeStates:           make(map[string]*deviceProbeState),
	}
}

// checkPairingThrottle returns true when an event for identifier should be
// skipped because the same device was already processed recently.
func (dm *DeviceManager) checkPairingThrottle(identifier string) bool {
	dm.pairingMu.Lock()
	defer dm.pairingMu.Unlock()

	if last, ok := dm.pairingCheckedAt[identifier]; ok && time.Since(last) < pairingCheckInterval {
		return true
	}
	dm.pairingCheckedAt[identifier] = time.Now()
	return false
}

// updateRemotePairingDevice refreshes the connection metadata (IP, port,
// discovery time) of a remote-pairing device without re-running the slow
// pairing check. It is called even when an event is throttled, so a device
// that reconnects without a goodbye (no Remove event) reflects its new
// address immediately instead of keeping stale data for the whole window.
func (dm *DeviceManager) updateRemotePairingDevice(serviceName, ip string, port uint16) {
	dm.devices.Range(func(k, v any) bool {
		dev := v.(model.Device)
		if dev.ServiceName == serviceName && dev.Connection == model.DeviceConnectionRemote {
			dev.IP = ip
			dev.Port = port
			dev.DiscoveryAt = time.Now()
			dm.devices.Store(k, dev)
			dm.markDeviceAnnounced(dev)
			return false
		}
		return true
	})
}

// cancelPairingCheck cancels the in-flight pairing check for a device and
// removes it from the tracking map. Safe to call when no check is running.
func (dm *DeviceManager) cancelPairingCheck(identifier string) {
	dm.pairingMu.Lock()
	defer dm.pairingMu.Unlock()
	if check, ok := dm.pairingCancel[identifier]; ok {
		check.cancel()
		delete(dm.pairingCancel, identifier)
	}
}

// finishPairingCheck removes only the check that is finishing. An older
// check must not erase a newer check's cancellation function.
func (dm *DeviceManager) finishPairingCheck(identifier string, check *pairingCheck) {
	check.cancel()
	dm.pairingMu.Lock()
	defer dm.pairingMu.Unlock()
	if dm.pairingCancel[identifier] == check {
		delete(dm.pairingCancel, identifier)
	}
}

// clearPairingThrottle removes the throttle record for a device, so its
// next Add event is processed even within the throttle window.
func (dm *DeviceManager) clearPairingThrottle(identifier string) {
	dm.pairingMu.Lock()
	defer dm.pairingMu.Unlock()
	delete(dm.pairingCheckedAt, identifier)
}

// rememberRemotePairingService records the identifier from an ItemNew event.
// Avahi's matching ItemRemove event includes only the service name, not TXT
// records, so this mapping is needed to cancel and unthrottle the correct
// pairing check when the service disappears.
func (dm *DeviceManager) rememberRemotePairingService(serviceName, identifier string) {
	dm.pairingMu.Lock()
	defer dm.pairingMu.Unlock()
	dm.remotePairingServices[serviceName] = identifier
}

// removeRemotePairingService handles a remote-pairing ItemRemove event. It
// cancels only the check associated with that service and clears the matching
// throttle entry so an immediate reconnect can be checked again.
func (dm *DeviceManager) removeRemotePairingService(serviceName string) {
	dm.pairingMu.Lock()
	defer dm.pairingMu.Unlock()

	identifier, ok := dm.remotePairingServices[serviceName]
	if !ok {
		return
	}
	delete(dm.remotePairingServices, serviceName)

	if check, ok := dm.pairingCancel[identifier]; ok {
		check.cancel()
		delete(dm.pairingCancel, identifier)
	}
	delete(dm.pairingCheckedAt, identifier)
}

// checkRemotePairingAsync runs the slow plumesign pairing check off the
// mDNS/avahi event loop so bursts of Add/Remove events can never stall
// signal consumption. The check is cancelled when the device disconnects
// (Remove event) or a newer Add supersedes it; cancellation kills the
// plumesign subprocess and the goroutine returns promptly.
func (dm *DeviceManager) checkRemotePairingAsync(ctx context.Context, identifier, authTag, serviceName, name, ip string, port uint16) {
	if ctx == nil {
		ctx = context.Background()
	}
	checkCtx, cancel := context.WithCancel(ctx)
	check := &pairingCheck{cancel: cancel}
	dm.pairingMu.Lock()
	if prev, ok := dm.pairingCancel[identifier]; ok {
		prev.cancel() // a newer event supersedes the previous check
	}
	dm.pairingCancel[identifier] = check
	dm.pairingMu.Unlock()
	defer dm.finishPairingCheck(identifier, check)

	log.Debugf("pairing check started: identifier=%s service=%s ip=%s port=%d", identifier, serviceName, ip, port)
	v, err := dm.CheckDevicePairedContext(checkCtx, identifier, authTag)
	if err != nil || v == nil {
		return
	}
	if checkCtx.Err() != nil {
		return // cancelled by a Remove event or a newer Add
	}

	if v.Name != "" {
		name = v.Name
	}
	device := model.Device{
		ID:          v.ID,
		Name:        name,
		ServiceName: serviceName,
		MacAddr:     "",
		IP:          ip,
		Port:        port,
		UDID:        v.RemotePairingUDID,
		Connection:  model.DeviceConnectionRemote,
		Status:      model.Paired,
		DiscoveryAt: time.Now(),
	}
	if v.GetDeviceClass() != "" {
		device.DeviceClass = v.GetDeviceClass()
	} else {
		device.ParseDeviceClass()
	}

	// remove old lockdown connection device, avoid duplicate devices with both lockdown and remote connection
	removeLockdownDevice(device.UDID)
	dm.DeleteDeviceByUDID(device.UDID)
	dm.SaveDevice(device)
	dm.markDeviceAnnounced(device)

	// Trigger device connection callback
	dm.onDeviceConnected(device)
}

func (dm *DeviceManager) GetDevices() []model.Device {
	devices := []model.Device{}
	seen := map[string]bool{}
	dm.devices.Range(func(k, v any) bool {
		dev := v.(model.Device)
		// A stale mDNS entry is hidden so its manual (pairing-file) record
		// takes over. Liveness is only tracked for dual-source devices.
		if dm.isDeviceStale(dev) {
			return true
		}
		devices = append(devices, dev)
		seen[deviceKey(dev)] = true
		return true
	})

	// Merge manual (pairing-file) devices, skipping any already discovered by
	// mDNS so the fresher discovered entry wins.
	dm.manualDevices.Range(func(k, v any) bool {
		dev := v.(model.Device)
		if !seen[deviceKey(dev)] {
			devices = append(devices, dev)
		}
		return true
	})

	// Sort devices: AppleTV type first, then by DeviceClass, then by Name
	sort.Slice(devices, func(i, j int) bool {
		classI := devices[i].DeviceClass
		classJ := devices[j].DeviceClass
		nameI := devices[i].Name
		nameJ := devices[j].Name

		// Detect AppleTV by DeviceClass or fallback to name if empty
		lowerClassI := strings.ToLower(classI)
		lowerClassJ := strings.ToLower(classJ)

		isAppleTVI := strings.Contains(lowerClassI, "appletv")
		isAppleTVJ := strings.Contains(lowerClassJ, "appletv")

		// AppleTV type has highest priority
		if isAppleTVI != isAppleTVJ {
			return isAppleTVI
		}

		// If classes differ, sort by DeviceClass string
		if classI != classJ {
			return classI < classJ
		}

		// Finally sort by name
		return nameI < nameJ
	})

	return devices
}

func (dm *DeviceManager) GetDeviceByID(id string) (*model.Device, bool) {
	devices := dm.GetDevices()
	for _, d := range devices {
		if d.ID == id {
			return &d, true
		}
	}

	return nil, false
}

func (dm *DeviceManager) GetDeviceByUDID(udid string) (*model.Device, bool) {
	devices := dm.GetDevices()
	for _, d := range devices {
		if d.UDID == udid {
			return &d, true
		}
	}

	return nil, false
}

func (dm *DeviceManager) GetDeviceInfo(dev *model.Device) (*model.DeviceInfo, error) {
	cmd := exec.Command("plumesign", "device-info", "-u", dev.UDID).WithTimeout(5 * time.Second)
	if dev.Connection == model.DeviceConnectionRemote {
		cmd = exec.Command("plumesign", "device-info", "--ip", dev.IP, "--port", fmt.Sprintf("%d", dev.Port), "-u", dev.UDID).WithTimeout(5 * time.Second)
	}

	data, err := cmd.CombinedOutput()
	if err != nil {
		log.Err(err).Msgf("Error getting device info for %s (%s): %s", dev.Name, dev.UDID, string(data))
		return nil, fmt.Errorf("%s%s", string(data), err.Error())
	}
	output := string(data)
	lines := strings.Split(string(output), "\n")

	devInfo := &model.DeviceInfo{UniqueDeviceID: dev.UDID}
	for _, line := range lines {
		var parts = strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "UniqueDeviceID":
			devInfo.UniqueDeviceID = value
		case "ProductName":
			devInfo.ProductName = value
		case "ProductType":
			devInfo.ProductType = value
		case "ProductVersion":
			devInfo.ProductVersion = value
		case "DeviceClass":
			devInfo.DeviceClass = value
		case "DeviceName":
			devInfo.DeviceName = value
		case "WiFiAddress":
			devInfo.WiFiAddress = value
		case "SerialNumber":
			devInfo.SerialNumber = value
		case "PersonalizedImageMounted":
			devInfo.PersonalizedImageMounted = value == "true"
		case "DeveloperModeStatus":
			devInfo.DeveloperModeStatus = value == "true"
		}
	}
	return devInfo, nil
}

func (dm *DeviceManager) AppendProductInfo(dev *model.Device, devInfo model.DeviceInfo) {
	if dev.Name != devInfo.DeviceName ||
		dev.ProductVersion != devInfo.ProductVersion ||
		dev.DeviceClass != devInfo.DeviceClass ||
		dev.PersonalizedImageMounted != devInfo.PersonalizedImageMounted ||
		dev.DeveloperModeStatus != devInfo.DeveloperModeStatus {

		dev.Name = devInfo.DeviceName
		dev.ProductType = devInfo.ProductType
		dev.ProductVersion = devInfo.ProductVersion
		dev.DeviceClass = devInfo.DeviceClass
		dev.PersonalizedImageMounted = devInfo.PersonalizedImageMounted
		dev.DeveloperModeStatus = devInfo.DeveloperModeStatus

		dm.SaveDevice(*dev)
	}
}

func (dm *DeviceManager) SaveDevice(dev model.Device) {
	dm.devices.Store(dev.ID, dev)
}

func (dm *DeviceManager) DeleteDevice(id string) {
	dm.devices.Delete(id)
}

func (dm *DeviceManager) DeleteDeviceByUDID(udid string) {
	dm.devices.Range(func(k, v any) bool {
		if v.(model.Device).UDID == udid {
			dm.devices.Delete(k)
			return false
		}
		return true
	})
	dm.clearProbeState(udid)
}

func (dm *DeviceManager) DeleteDeviceByMacAddr(macAddr string) {
	dm.devices.Range(func(k, v any) bool {
		if v.(model.Device).MacAddr == macAddr {
			dm.devices.Delete(k)
			return false
		}
		return true
	})
}

func (dm *DeviceManager) HasCheckedDevice(ip string, port uint16, name string) bool {
	hasChecked := false
	dm.devices.Range(func(k, v any) bool {
		dev := v.(model.Device)
		if dev.IP == ip && dev.Port == port && dev.Status == model.Paired {
			hasChecked = true
			return false
		}
		return true
	})
	return hasChecked
}

func (dm *DeviceManager) DeleteDeviceByServiceName(serviceName string, conection model.DeviceConnection) {
	dm.devices.Range(func(k, v any) bool {
		dev := v.(model.Device)
		if dev.Connection == conection && dev.ServiceName == serviceName {
			dm.devices.Delete(k)
			dm.clearProbeState(probeKey(dev))
			return false
		}
		return true
	})
}

func (dm *DeviceManager) ReloadDevices() {
	dm.devices.Range(func(k, v any) bool {
		dev := v.(model.Device)
		if dev.Status == model.Pairable {
			// 检查是否已连接
			lockdownDevices, err := loadLockdownDevices()
			if err != nil {
				log.Println(err)
				return false
			}

			if lockdownDev, ok := lockdownDevices[dev.MacAddr]; ok {
				udid := lockdownDev.Name

				// 判断是否已存在连接状态
				if _, ok := dm.devices.Load(udid); ok {
					return false
				}

				// 添加新状态
				dev.ID = utils.Md5(udid)
				dev.Status = model.Paired
				dev.UDID = udid
				dm.devices.Store(udid, dev)
			}
		}

		return true
	})

	// Refresh the manual (pairing-file) devices on every reload.
	dm.ReloadManualDevices()
}

// deviceKey deduplicates devices by UDID, falling back to IP when the UDID is
// unknown.
func deviceKey(dev model.Device) string {
	if dev.UDID != "" {
		return "udid:" + dev.UDID
	}
	return "ip:" + dev.IP
}

// ReloadManualDevices rebuilds manualDevices from the pairing files. A manual
// device is one paired by a manually entered IP (json has manual=true and a
// remotepairing_ip); mDNS-discovered devices are not written here.
func (dm *DeviceManager) ReloadManualDevices() {
	dm.loadManualDevicesFromDir(app.RemotePairingDir())
}

// loadManualDevicesFromDir rebuilds manualDevices from the .json pairing files
// in dir (split out so tests can point at a temp directory).
func (dm *DeviceManager) loadManualDevicesFromDir(dir string) {
	// Preserve a resolved tunnel port across reloads: ports are dynamic, but a
	// freshly probed one stays valid for the cache TTL.
	previous := sync.Map{}
	dm.manualDevices.Range(func(k, v any) bool {
		previous.Store(k, v)
		return true
	})
	dm.manualDevices = sync.Map{}
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		log.Err(err).Msg("Failed to list pairing files")
		return
	}
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Err(err).Msgf("Failed to read pairing file %s", path)
			continue
		}
		var rd model.RemoteDevice
		if err := utils.FromJSON(data, &rd); err != nil {
			continue
		}
		if !rd.Manual || rd.RemotePairingIP == "" {
			continue
		}
		udid := rd.RemotePairingUDID
		id := utils.Md5(udid)
		dev := model.Device{
			ID:          id,
			Name:        rd.Name,
			IP:          rd.RemotePairingIP,
			UDID:        udid,
			Manual:      true,
			Connection:  model.DeviceConnectionRemote,
			Status:      model.Paired,
			DeviceClass: rd.GetDeviceClass(),
			DiscoveryAt: time.Now(),
		}
		if dev.Name == "" {
			dev.Name = rd.RemotePairingIP
		}
		if old, ok := previous.Load(id); ok {
			o := old.(model.Device)
			dev.Port = o.Port
			dev.PortResolvedAt = o.PortResolvedAt
		}
		dm.manualDevices.Store(id, dev)
	}
}

// manualPortTTL bounds how long a discovered manual-device tunnel port is
// reused before it is probed again.
const manualPortTTL = 60 * time.Second

// manualPortWarmTimeout bounds the post-pairing background port warm-up: long
// enough for one full-range scan over a slow overlay, short enough not to leak
// a goroutine forever.
const manualPortWarmTimeout = 45 * time.Second

// discoverTunnelPortFunc resolves a manual device's _remotepairing tunnel
// port. It is a variable so tests can replace the network probe.
var discoverTunnelPortFunc = DiscoverManualTunnelPort

// Remembered-port re-check tuning. The remembered port is the cheap path that
// avoids a full-range scan, so it must not fail on a healthy device merely
// because the overlay relay is slow: a single handshake gets the full dial
// budget as its response deadline (the 800ms scan deadline is too tight for
// the 0.3-1.1s relay RTT measured in doc/misc/remote-pairing-port-discovery.md)
// and a transient loss is retried a few times before falling back to a scan.
const (
	rememberedProbeAttempts   = 3
	rememberedProbeBackoff    = 200 * time.Millisecond
	rememberedProbeRespBudget = rpProbeTimeout
)

// probeRememberedPortFunc re-checks a remembered tunnel port with a single
// handshake. It is a variable so tests can replace the network probe.
var probeRememberedPortFunc = func(ctx context.Context, ip string, port uint16) (*PairingServiceInfo, error) {
	return probePairingService(ctx, ip, port, rpProbeTimeout, rememberedProbeRespBudget)
}

// probeRememberedPort retries probeRememberedPortFunc a few times with a short
// backoff, so one dropped SYN or delayed handshake on a slow relay does not
// escalate to a multi-second full-range scan. It returns as soon as a probe
// confirms the port is the tunnel service, or the last error otherwise.
func probeRememberedPort(ctx context.Context, ip string, port uint16) (*PairingServiceInfo, error) {
	var lastErr error
	for attempt := 0; attempt < rememberedProbeAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(rememberedProbeBackoff):
			}
		}
		si, err := probeRememberedPortFunc(ctx, ip, port)
		if err == nil && si != nil && si.Tunnel {
			return si, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("port %d did not answer as a tunnel service", port)
		}
	}
	return nil, lastErr
}

// manualPortLock returns the per-device mutex used to serialize port discovery.
func (dm *DeviceManager) manualPortLock(key string) *sync.Mutex {
	dm.manualPortMu.Lock()
	defer dm.manualPortMu.Unlock()
	l, ok := dm.manualPortLocks[key]
	if !ok {
		l = &sync.Mutex{}
		dm.manualPortLocks[key] = l
	}
	return l
}

// PrepareManualDevice ensures a manual device has a usable _remotepairing
// tunnel port before an install/refresh/mount. Non-manual devices are a no-op.
// The port is dynamic, so it is re-probed when missing or older than
// manualPortTTL. It never retries automatically; callers decide what to do on
// error.
func (dm *DeviceManager) PrepareManualDevice(ctx context.Context, dev *model.Device) error {
	if dev == nil || !dev.Manual || dev.IP == "" {
		return nil
	}
	if dev.Port != 0 && time.Since(dev.PortResolvedAt) < manualPortTTL {
		return nil
	}

	key := dev.UDID
	if key == "" {
		key = dev.IP
	}
	lock := dm.manualPortLock(key)
	lock.Lock()
	defer lock.Unlock()

	// Another caller may have resolved the port while we waited; the cache is
	// shared, so check it even when this caller's copy is still empty.
	if dev.Port != 0 && time.Since(dev.PortResolvedAt) < manualPortTTL {
		return nil
	}
	if dev.UDID != "" {
		if cached, ok := dm.manualDevices.Load(utils.Md5(dev.UDID)); ok {
			c := cached.(model.Device)
			if c.Port != 0 && time.Since(c.PortResolvedAt) < manualPortTTL {
				dev.Port = c.Port
				dev.PortResolvedAt = c.PortResolvedAt
				return nil
			}
		}
	}

	// The tunnel port only changes when the device restarts its services, so
	// a remembered port usually survives the TTL. Probe it first with a
	// single handshake (retried a few times, cheap and relay-friendly) before
	// falling back to the expensive full-range scan.
	if remembered := dm.rememberedManualPort(dev); remembered != 0 {
		if si, err := probeRememberedPort(ctx, dev.IP, remembered); err == nil && si != nil && si.Tunnel {
			dev.Port = remembered
			dev.PortResolvedAt = time.Now()
			if dev.UDID != "" {
				dm.manualDevices.Store(utils.Md5(dev.UDID), *dev)
			}
			return nil
		}
	}

	port, _, err := discoverTunnelPortFunc(ctx, dev.IP, nil)
	if err != nil {
		return fmt.Errorf("manual device tunnel port not found for %s: %w", dev.IP, err)
	}
	dev.Port = port
	dev.PortResolvedAt = time.Now()
	if dev.UDID != "" {
		dm.manualDevices.Store(utils.Md5(dev.UDID), *dev)
	}
	return nil
}

// rememberedManualPort returns the last tunnel port seen for the device, from
// the caller's copy first, then the shared cache. A port survives in the
// cache after its TTL expires: it is stale for direct use, but still the
// best first candidate to re-probe.
func (dm *DeviceManager) rememberedManualPort(dev *model.Device) uint16 {
	if dev.Port != 0 {
		return dev.Port
	}
	if dev.UDID == "" {
		return 0
	}
	if cached, ok := dm.manualDevices.Load(utils.Md5(dev.UDID)); ok {
		c := cached.(model.Device)
		return c.Port
	}
	return 0
}

// WarmManualDevicePort resolves a manual device's tunnel port in the
// background right after its pairing completes, so the install page does not
// pay for the full-range scan on the first install. It looks the device up by
// IP (the pairing record that makes the scan possible was just written, and
// the freshly rebuilt manual entry carries its UDID) and runs the same
// PrepareManualDevice path a foreground install would use. Failures are only
// logged: the install page resolves the port on demand anyway.
func (dm *DeviceManager) WarmManualDevicePort(ip string) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return
	}
	// Reload so the manual entry picks up the pairing record the flow just
	// wrote (UDID, model, name); without it the scan cannot run.
	dm.ReloadManualDevices()

	var target *model.Device
	dm.manualDevices.Range(func(_, v any) bool {
		dev := v.(model.Device)
		if dev.IP == ip {
			target = &dev
			return false
		}
		return true
	})
	if target == nil || target.UDID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), manualPortWarmTimeout)
	defer cancel()
	if err := dm.PrepareManualDevice(ctx, target); err != nil {
		log.Debugf("Manual device %s port warm-up failed: %v", ip, err)
		return
	}
	log.Infof("Manual device %s tunnel port ready: %d", ip, target.Port)
}

// RemoveManualDevice drops a manual device from memory (after its pairing
// files are deleted).
func (dm *DeviceManager) RemoveManualDevice(id string) {
	dm.manualDevices.Delete(id)
}

func (dm *DeviceManager) CheckAfcServiceStatus(dev *model.Device) error {
	cmd := exec.Command("plumesign", "check", "afc", "--udid", dev.UDID).WithTimeout(10 * time.Second)
	if dev.Connection == model.DeviceConnectionRemote {
		cmd = exec.Command("plumesign", "check", "afc", "--ip", dev.IP, "--port", fmt.Sprintf("%d", dev.Port), "--udid", dev.UDID).WithTimeout(10 * time.Second)
	}

	data, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	output := string(data)
	if strings.Contains(output, "Error") {
		return fmt.Errorf("%s", output)
	}

	if !strings.Contains(output, "SUCCESS") {
		return fmt.Errorf("%s", output)
	}

	return nil
}

func (dm *DeviceManager) CheckDevicePaired(identifier string, authTag string) (*model.RemoteDevice, error) {
	return dm.CheckDevicePairedContext(context.Background(), identifier, authTag)
}

// CheckDevicePairedContext is CheckDevicePaired with a cancellable
// context: when ctx is cancelled (e.g. the device disconnects), the
// plumesign subprocess is killed and the goroutine returns promptly.
func (dm *DeviceManager) CheckDevicePairedContext(ctx context.Context, identifier string, authTag string) (*model.RemoteDevice, error) {
	if !utils.ExistFiles(app.RemotePairingDir(), "*.plist") {
		return nil, nil
	}

	cmd := exec.CommandContext(ctx, "plumesign", "check", "find-pairing", "--identifier", identifier, "--auth-tag", authTag).WithTimeout(10 * time.Second).WithSecret(authTag)

	data, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}

	re := regexp.MustCompile("UDID: `([^`]+)`")
	matches := re.FindStringSubmatch(string(data))
	if len(matches) > 1 {
		udid := matches[1]

		remoteDev := model.RemoteDevice{
			ID:                utils.Md5(udid),
			RemotePairingUDID: udid,
		}

		peerDevicePath := filepath.Join(app.RemotePairingDir(), fmt.Sprintf("%s.json", udid))
		if utils.Exists(peerDevicePath) {
			if data, err := os.ReadFile(peerDevicePath); err == nil {
				if err := utils.FromJSON(data, &remoteDev); err == nil {
					remoteDev.ID = utils.Md5(remoteDev.RemotePairingUDID)
				}
			}
		}

		return &remoteDev, nil
	}

	return nil, fmt.Errorf("failed to parse device information")
}

func (dm *DeviceManager) parseName(host string) string {
	name := strings.TrimSuffix(host, ".")
	name = strings.TrimSuffix(name, ".local")
	return name
}

// SetOnDeviceConnected Set the callback function for device connection
func (dm *DeviceManager) SetOnDeviceConnected(callback func(device model.Device)) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if callback != nil {
		dm.onDeviceConnected = callback
	}
}

// SetOnDeviceDisconnected Set the callback function for device disconnection
func (dm *DeviceManager) SetOnDeviceDisconnected(callback func(device model.Device)) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if callback != nil {
		dm.onDeviceDisconnected = callback
	}
}

// Stop Stop the device manager
func (dm *DeviceManager) Stop() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.cancel != nil {
		dm.cancel()
		dm.cancel = nil
		dm.ctx = nil
		log.Info("Device manager stopped")
	}
}

// Exported functions for external package usage

// SetDeviceConnectedCallback Set the callback function for device connection (exported function)
func SetDeviceConnectedCallback(callback func(device model.Device)) {
	deviceManager.SetOnDeviceConnected(callback)
}

// SetDeviceDisconnectedCallback Set the callback function for device disconnection (exported function)
func SetDeviceDisconnectedCallback(callback func(device model.Device)) {
	deviceManager.SetOnDeviceDisconnected(callback)
}

package manager

import (
	"fmt"
	"runtime"
	"time"

	"github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
)

var usbmuxdManager = newUsbmuxdManager()

type UsbmuxdManager struct {
	isSupport bool
}

func newUsbmuxdManager() *UsbmuxdManager {
	return &UsbmuxdManager{
		isSupport: runtime.GOOS == "linux",
	}
}

func (m *UsbmuxdManager) TryWaitReady(d time.Duration) {
	if !m.isSupport {
		return
	}
	time.Sleep(d)
}

func (m *UsbmuxdManager) Restart() error {
	if !m.isSupport {
		log.Warnf("restarting usbmuxd is only supported on Linux")
		return nil
	}

	cmd := exec.Command("/etc/init.d/usbmuxd", "restart").WithTimeout(time.Minute)
	data, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s%s", string(data), err.Error())
	}

	return nil
}

//go:build windows

package exec

import (
	"os/exec"
	"syscall"
)

// createNoWindow keeps Windows from giving a console program a console of
// its own. The desktop shell is a GUI program and has none, so without it
// every plumesign call would open a console window.
const createNoWindow = 0x08000000

// hideWindow keeps the console window of a console program off the screen:
// no window is created for the child, and its standard handles still reach
// the pipes of the caller.
func hideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}

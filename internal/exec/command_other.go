//go:build !windows

package exec

import "os/exec"

// hideWindow does nothing: only Windows opens a console window for a child
// process of a GUI app.
func hideWindow(*exec.Cmd) {}

package manager

import (
	"errors"
	"testing"
)

func TestUnsupportedUsbmuxdRestartDoesNotReportSuccess(t *testing.T) {
	m := &UsbmuxdManager{}
	if err := m.Restart(); !errors.Is(err, ErrUsbmuxdRestartUnsupported) {
		t.Fatalf("Restart = %v, want ErrUsbmuxdRestartUnsupported", err)
	}
}

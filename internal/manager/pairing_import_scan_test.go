package manager

import (
	"errors"
	"strconv"
	"testing"
)

// Previously saved pairing records may belong to other devices. The uploaded
// record must be used to check which port accepts the import.
func TestScanAndImportPairingFileUsesUploadedRecord(t *testing.T) {
	_, _ = stubScan(t, []uint16{pairPortFixed, 49152, 51000, 60001}, 0)
	pairingRecordPaths = func() []string { return []string{"unrelated.plist"} }

	record := []byte("uploaded-record")
	var checked []uint16
	port, err := scanAndImportPairingFile("100.99.166.81", record,
		func(ip, portString string, supplied []byte) error {
			if ip != "100.99.166.81" || string(supplied) != string(record) {
				t.Fatalf("wrong device or pairing record: %s %q", ip, supplied)
			}
			p, err := strconv.ParseUint(portString, 10, 16)
			if err != nil {
				t.Fatal(err)
			}
			checked = append(checked, uint16(p))
			if p == 51000 {
				return nil
			}
			return errors.New("unrelated port")
		})
	if err != nil || port != 51000 {
		t.Fatalf("wanted imported record validated on port 51000, got %d (%v)", port, err)
	}
	if len(checked) != 2 || checked[0] != 49152 || checked[1] != 51000 {
		t.Fatalf("unexpected candidate validation order: %v", checked)
	}
}

func TestScanAndImportPairingFileRejectsUnvalidatedPorts(t *testing.T) {
	_, _ = stubScan(t, []uint16{pairPortFixed, 49152, 51000}, 0)
	checked := 0
	port, err := scanAndImportPairingFile("100.99.166.81", []byte("invalid"),
		func(ip, candidate string, supplied []byte) error {
			checked++
			return errors.New("invalid pairing record")
		})
	if err == nil || port != 0 || checked != 2 {
		t.Fatalf("invalid record must be rejected: port=%d, error=%v, attempts=%d", port, err, checked)
	}
}

func TestScanAndImportPairingFileCapsValidationAttempts(t *testing.T) {
	ports := make([]uint16, maxValidatedPorts+5)
	for i := range ports {
		ports[i] = scanPortFirst + uint16(i)
	}
	_, _ = stubScan(t, ports, 0)
	checked := 0
	port, err := scanAndImportPairingFile("100.99.166.81", []byte("invalid"),
		func(ip, candidate string, supplied []byte) error {
			checked++
			return errors.New("invalid pairing record")
		})
	if err == nil || port != 0 || checked != maxValidatedPorts {
		t.Fatalf("expected %d bounded attempts, got port=%d error=%v attempts=%d", maxValidatedPorts, port, err, checked)
	}
}

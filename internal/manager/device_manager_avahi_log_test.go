//go:build linux

package manager

import (
	"fmt"
	"strings"
	"testing"
)

func TestAvahiLogRedactsAuthTagWithoutChangingPairingData(t *testing.T) {
	const secret = "pairing-secret-123"
	txt := [][]byte{
		[]byte("identifier=device-id"),
		[]byte("authTag=" + secret),
		[]byte("flags=0"),
	}

	dm := newDeviceManager()
	logged := dm.parseTextRecordForLog(txt)
	if got := logged["authTag"]; got != "[REDACTED]" {
		t.Fatalf("logged authTag = %q, want redaction", got)
	}
	if got := logged["identifier"]; got != "device-id" {
		t.Fatalf("logged identifier = %q, want original metadata", got)
	}
	if got := logged["flags"]; got != "0" {
		t.Fatalf("logged flags = %q, want original metadata", got)
	}
	if strings.Contains(fmt.Sprint(logged), secret) {
		t.Fatal("pairing auth tag leaked into log metadata")
	}
	if got := dm.parseTextRecordAuthTag(txt); got != secret {
		t.Fatalf("pairing authTag changed: %q", got)
	}
	if got := string(txt[1]); got != "authTag="+secret {
		t.Fatalf("original TXT record mutated: %q", got)
	}
}

func TestAvahiLogPreservesRecordsWithoutAuthTag(t *testing.T) {
	dm := newDeviceManager()
	txt := [][]byte{
		[]byte("identifier=device-id"),
		[]byte("flags=0"),
		[]byte("invalid"),
	}
	got := dm.parseTextRecordForLog(txt)
	if _, ok := got["authTag"]; ok {
		t.Fatal("unexpected authTag field")
	}
	if got["identifier"] != "device-id" || got["flags"] != "0" {
		t.Fatalf("log metadata was changed: %v", got)
	}
	if _, ok := got["invalid"]; ok {
		t.Fatal("malformed TXT record unexpectedly parsed")
	}
}

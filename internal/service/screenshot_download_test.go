package service

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func jpegFixture(size int) []byte {
	data := bytes.Repeat([]byte{0x00}, size)
	copy(data, []byte{0xFF, 0xD8, 0xFF})
	return data
}

func TestDecodeScreenshotDownloadRoundTrip(t *testing.T) {
	data := jpegFixture(64)

	got, err := DecodeScreenshotDownload(base64.StdEncoding.EncodeToString(data))
	if err != nil {
		t.Fatalf("DecodeScreenshotDownload returned error: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("decoded bytes differ from the posted bytes")
	}
}

func TestDecodeScreenshotDownloadAcceptsSurroundingWhitespace(t *testing.T) {
	encoded := "  " + base64.StdEncoding.EncodeToString(jpegFixture(8)) + "\n"
	if _, err := DecodeScreenshotDownload(encoded); err != nil {
		t.Fatalf("expected whitespace to be tolerated, got %v", err)
	}
}

func TestDecodeScreenshotDownloadRejectsInvalidPayloads(t *testing.T) {
	cases := map[string]string{
		"empty":      "",
		"not base64": "!!!!",
		"not a jpeg": base64.StdEncoding.EncodeToString([]byte("hello")),
		"oversized":  base64.StdEncoding.EncodeToString(jpegFixture(screenshotDownloadMaxBytes + 1)),
		"truncated":  base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8}),
	}

	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeScreenshotDownload(payload); err == nil {
				t.Fatal("expected the payload to be rejected")
			}
		})
	}
}

func TestIsJPEG(t *testing.T) {
	if !isJPEG(jpegFixture(8)) {
		t.Fatal("expected the JPEG fixture to be recognized")
	}
	for _, data := range [][]byte{nil, {}, []byte("PNG"), {0xFF, 0xD8}} {
		if isJPEG(data) {
			t.Fatalf("expected %v to be rejected as JPEG", data)
		}
	}
}

func TestDecodeScreenshotDownloadErrorMessage(t *testing.T) {
	_, err := DecodeScreenshotDownload("!!!!")
	if err == nil || !strings.Contains(err.Error(), "base64") {
		t.Fatalf("error = %v, want it to mention base64", err)
	}
}

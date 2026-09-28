package service

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// screenshotDownloadMaxBytes caps a download request. The capture pipeline
// encodes a 1920x1080 JPEG at quality 80, far below this limit.
const screenshotDownloadMaxBytes = 8 << 20

// DecodeScreenshotDownload decodes the base64 JPEG a client posts when it
// cannot save a client-side download, so the same request can answer with the
// image as an attachment. In-app WebViews (notably on iOS) only download from
// a real http(s) response, so the client posts the preview it is already
// showing and the browser saves exactly that image.
func DecodeScreenshotDownload(encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, errors.New("screenshot data is not valid base64")
	}
	if len(data) == 0 {
		return nil, errors.New("screenshot data is empty")
	}
	if len(data) > screenshotDownloadMaxBytes {
		return nil, fmt.Errorf("screenshot is too large: %d bytes", len(data))
	}
	if !isJPEG(data) {
		return nil, errors.New("screenshot must be a JPEG image")
	}
	return data, nil
}

func isJPEG(data []byte) bool {
	return len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF
}

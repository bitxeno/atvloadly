package ipa

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDownloadIPAWithHeadersStripsAuthorizationOnCrossHostRedirect(t *testing.T) {
	var storageAuthorization string
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		storageAuthorization = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("fixture"))
	}))
	t.Cleanup(storage.Close)

	var apiAuthorization, apiAccept string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiAuthorization = r.Header.Get("Authorization")
		apiAccept = r.Header.Get("Accept")
		// Different hostname, same local test server: this exercises the
		// cross-host redirect rule used by GitHub's signed asset URL.
		target := strings.Replace(storage.URL, "127.0.0.1", "localhost", 1) + "/signed"
		http.Redirect(w, r, target, http.StatusFound)
	}))
	t.Cleanup(api.Close)

	header := make(http.Header)
	header.Set("Authorization", "Bearer secret")
	header.Set("Accept", "application/octet-stream")

	path, err := downloadIPAWithHeaders(api.URL+"/asset", t.TempDir(), header, nil)
	if err != nil {
		t.Fatalf("downloadIPAWithHeaders: %v", err)
	}
	defer func() { _ = os.Remove(path) }()

	if apiAuthorization != "Bearer secret" || apiAccept != "application/octet-stream" {
		t.Fatalf("initial headers: Authorization=%q Accept=%q", apiAuthorization, apiAccept)
	}
	if storageAuthorization != "" {
		t.Fatalf("Authorization leaked to redirect target: %q", storageAuthorization)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "fixture" {
		t.Fatalf("downloaded %q, want fixture", data)
	}
}

package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/source"
)

// brokenSource serves an AltStore source URL that always fails with 500.
func brokenSource(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/apps.json"
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func savedURLs(t *testing.T) []string {
	t.Helper()
	sources, err := GetSavedSources()
	if err != nil {
		t.Fatalf("GetSavedSources: %v", err)
	}
	var urls []string
	for _, s := range sources {
		urls = append(urls, s.URL)
	}
	return urls
}

func mustParseURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestAddAndDeleteSavedSource(t *testing.T) {
	setupTestDB(t)

	empty, err := GetSavedSources()
	if err != nil || mustMarshal(t, empty) != "[]" {
		t.Fatalf("GetSavedSources = %v, %v; want an empty array", empty, err)
	}

	u, requests := serveSource(t, testSource)
	saved, err := AddSavedSource(" " + u + "\n")
	if err != nil {
		t.Fatalf("AddSavedSource: %v", err)
	}
	if saved.ID == 0 || saved.URL != u || saved.Name != "Test source" || saved.CreatedAt.IsZero() {
		t.Fatalf("saved source = %+v", saved)
	}

	// Saving it again returns the existing entry without fetching the source.
	again, err := AddSavedSource(u)
	if err != nil || again.ID != saved.ID || again.Name != saved.Name {
		t.Fatalf("second AddSavedSource = %+v, %v; want %+v", again, err, saved)
	}
	if n := atomic.LoadInt32(requests); n != 1 {
		t.Fatalf("%d requests, want 1", n)
	}

	for _, bad := range []string{"", "apps.json", "ftp://example.com/apps.json"} {
		if _, err := AddSavedSource(bad); err == nil {
			t.Errorf("AddSavedSource(%q) should fail", bad)
		}
	}
	if _, err := AddSavedSource(brokenSource(t)); err == nil || err.Error() != "source returned 500" {
		t.Fatalf("unreadable source: %v", err)
	}
	noApp, _ := serveSource(t, `{"name":"Empty","apps":[{"name":"Nothing","bundleIdentifier":"com.example.none","version":"1.0"}]}`)
	if _, err := AddSavedSource(noApp); !errors.Is(err, source.ErrNoMatch) || err.Error() != "no matching build: the source has no downloadable app" {
		t.Fatalf("source without downloadable app: %v", err)
	}

	// A source without a name is saved under its host.
	nameless, _ := serveSource(t, `{"apps":[{"name":"A","bundleIdentifier":"com.example.a","versions":[{"version":"1","downloadURL":"https://example.com/A.ipa"}]}]}`)
	unnamed, err := AddSavedSource(nameless)
	if err != nil {
		t.Fatalf("AddSavedSource(nameless): %v", err)
	}
	if host := mustParseURL(t, nameless).Host; unnamed.Name != host {
		t.Fatalf("Name = %q, want the host %q", unnamed.Name, host)
	}

	if got := savedURLs(t); len(got) != 2 || got[0] != u || got[1] != nameless {
		t.Fatalf("saved sources = %v, want [%s %s]", got, u, nameless)
	}

	// An unknown id (the router parses a bad one as 0) deletes nothing.
	if err := DeleteSavedSource(0); err != nil || len(savedURLs(t)) != 2 {
		t.Fatalf("DeleteSavedSource(0) = %v, saved sources = %v", err, savedURLs(t))
	}

	// A deleted source can be saved again.
	if err := DeleteSavedSource(saved.ID); err != nil {
		t.Fatalf("DeleteSavedSource: %v", err)
	}
	if got := savedURLs(t); len(got) != 1 || got[0] != nameless {
		t.Fatalf("saved sources after delete = %v", got)
	}
	readded, err := AddSavedSource(u)
	if err != nil || readded.ID == saved.ID {
		t.Fatalf("AddSavedSource after delete = %+v, %v", readded, err)
	}
	if got := savedURLs(t); len(got) != 2 || got[1] != u {
		t.Fatalf("saved sources after adding again = %v", got)
	}
}

func TestAddSavedSourceConcurrently(t *testing.T) {
	setupTestDB(t)
	u, _ := serveSource(t, testSource)

	// Another request saves the same URL while this one fetches the source.
	first := model.SavedSource{URL: u, Name: "First"}
	oldFetch := fetchSource
	fetchSource = func(kind, location string) (source.Feed, error) {
		if err := db.Store().Create(&first).Error; err != nil {
			t.Error(err)
		}
		return source.Fetch(kind, location)
	}
	t.Cleanup(func() { fetchSource = oldFetch })

	saved, err := AddSavedSource(u)
	if err != nil || saved.ID != first.ID || saved.Name != "First" {
		t.Fatalf("AddSavedSource = %+v, %v; want the first entry %+v", saved, err, first)
	}
	if got := savedURLs(t); len(got) != 1 {
		t.Fatalf("saved sources = %v, want one", got)
	}
}

func TestGetSourceCatalog(t *testing.T) {
	setupTestDB(t)

	catalog, err := GetSourceCatalog("AppleTV")
	if err != nil || mustMarshal(t, catalog) != "[]" {
		t.Fatalf("empty catalog = %v, %v", catalog, err)
	}

	// The broken source is saved first: the catalog keeps the saved order.
	broken := model.SavedSource{URL: brokenSource(t), Name: "Broken"}
	u, requests := serveSource(t, testSource)
	alpha := latestBuild(t, u, "com.example.alpha")
	atomic.StoreInt32(requests, 0)
	working := model.SavedSource{URL: u, Name: "Saved name"}
	for _, s := range []*model.SavedSource{&broken, &working} {
		if err := db.Store().Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}

	catalog, err = GetSourceCatalog("AppleTV")
	if err != nil {
		t.Fatalf("GetSourceCatalog: %v", err)
	}
	if len(catalog) != 2 {
		t.Fatalf("got %d sources, want 2", len(catalog))
	}

	failed := catalog[0]
	if failed.ID != broken.ID || failed.URL != broken.URL || failed.Name != "Broken" || failed.Error != "source returned 500" || len(failed.Builds) != 0 {
		t.Fatalf("broken source = %+v", failed)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(mustMarshal(t, failed)), &decoded); err != nil {
		t.Fatal(err)
	}
	if builds, ok := decoded["builds"].([]any); !ok || len(builds) != 0 {
		t.Fatalf("broken source builds = %#v, want []", decoded["builds"])
	}

	// The name read from the source replaces the saved one.
	listed := catalog[1]
	if listed.ID != working.ID || listed.URL != u || listed.Name != "Test source" || listed.Error != "" || len(listed.Builds) != 2 ||
		listed.Builds[0].ID != alpha.ID || listed.Builds[1].Name != "Beta" || listed.Builds[1].Version != "5.0" {
		t.Fatalf("working source = %+v", listed)
	}
	if n := atomic.LoadInt32(requests); n != 1 {
		t.Fatalf("%d requests, want 1", n)
	}
}

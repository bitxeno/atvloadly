package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/model"
)

func TestDeleteAppOnlyRemovesOwnedDirectory(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "external path", path: "external"},
		{name: "staged upload", path: "upload"},
		{name: "empty path"},
		{name: "another app path", path: "other-app"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupTestDB(t)
			dataDir := app.Config.Server.DataDir

			installed := &model.InstalledApp{IpaName: "To remove"}
			if err := db.Store().Create(installed).Error; err != nil {
				t.Fatal(err)
			}

			ownedDir := filepath.Join(dataDir, "ipa", fmt.Sprintf("%d", installed.ID))
			ownedFile := writeTestFile(t, filepath.Join(ownedDir, "app.ipa"))

			var unrelatedFile, storedPath string
			switch test.path {
			case "external":
				unrelatedFile = writeTestFile(t, filepath.Join(t.TempDir(), "unrelated", "keep.ipa"))
				storedPath = unrelatedFile
			case "upload":
				storedPath = writeTestFile(t, filepath.Join(dataDir, "tmp", "staged.ipa"))
				unrelatedFile = writeTestFile(t, filepath.Join(dataDir, "tmp", "keep.ipa"))
			case "other-app":
				other := &model.InstalledApp{IpaName: "Keep"}
				if err := db.Store().Create(other).Error; err != nil {
					t.Fatal(err)
				}
				unrelatedFile = writeTestFile(t, filepath.Join(dataDir, "ipa", fmt.Sprintf("%d", other.ID), "app.ipa"))
				storedPath = unrelatedFile
			}
			if err := db.Store().Model(installed).Update("ipa_path", storedPath).Error; err != nil {
				t.Fatal(err)
			}

			removed, err := DeleteApp(installed.ID)
			if err != nil || !removed {
				t.Fatalf("DeleteApp = %t, %v", removed, err)
			}
			if _, err := GetApp(installed.ID); err == nil {
				t.Fatal("app record still exists")
			}
			if _, err := os.Stat(ownedFile); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("own app directory was not removed: %v", err)
			}
			if unrelatedFile != "" {
				if _, err := os.Stat(unrelatedFile); err != nil {
					t.Fatalf("another file was removed: %v", err)
				}
			}
		})
	}
}

func TestDeleteAppWithoutStorageRootKeepsRecord(t *testing.T) {
	for _, tc := range []struct {
		name      string
		nilConfig bool
	}{
		{name: "nil config", nilConfig: true},
		{name: "empty data directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupTestDB(t)
			dataDir := app.Config.Server.DataDir

			record := &model.InstalledApp{IpaName: "Keep"}
			if err := db.Store().Create(record).Error; err != nil {
				t.Fatal(err)
			}
			ownedFile := writeTestFile(t, filepath.Join(dataDir, "ipa", fmt.Sprintf("%d", record.ID), "app.ipa"))

			if tc.nilConfig {
				app.Config = nil
			} else {
				app.Config.Server.DataDir = ""
			}
			deleted, err := DeleteApp(record.ID)
			if err == nil || deleted {
				t.Fatalf("DeleteApp with missing storage root = %t, %v; want error without deletion", deleted, err)
			}
			if _, err := GetApp(record.ID); err != nil {
				t.Fatalf("app record was deleted: %v", err)
			}
			if _, err := os.Stat(ownedFile); err != nil {
				t.Fatalf("app files were altered: %v", err)
			}
		})
	}
}

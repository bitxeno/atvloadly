package app

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// legacyInstalledApp is the installed_apps schema released before signing modes existed.
type legacyInstalledApp struct {
	gorm.Model

	IpaName          string
	IpaPath          string
	Description      string
	Device           string
	DeviceClass      string
	UDID             string `gorm:"column:udid"`
	Account          string
	Password         string
	InstalledDate    *time.Time
	RefreshedDate    *time.Time
	ExpirationDate   *time.Time
	RefreshedResult  bool
	RefreshedError   int
	Icon             string
	BundleIdentifier string
	Version          string
	RemoveExtensions bool
	CustomName       string
	Enabled          bool
}

func (legacyInstalledApp) TableName() string { return "installed_apps" }

func createLegacyDatabase(t *testing.T, path string, row *legacyInstalledApp) {
	t.Helper()
	legacy, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if err := legacy.AutoMigrate(&legacyInstalledApp{}); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := legacy.Create(row).Error; err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	sqlDB, err := legacy.DB()
	if err != nil {
		t.Fatalf("legacy sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}
}

func initTestDb(t *testing.T, dir string) {
	t.Helper()
	conf := &Configuration{}
	conf.Db = db.Config{Path: dir, FileName: "app.db"}
	if err := InitDb(conf); err != nil {
		t.Fatalf("InitDb: %v", err)
	}
	sqlDB, err := db.Store().DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
}

func TestInitDbMigratesLegacyInstalledApps(t *testing.T) {
	dir := t.TempDir()
	expiration := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	legacyRow := &legacyInstalledApp{
		IpaName:          "Legacy",
		IpaPath:          "/data/ipa/legacy.ipa",
		Device:           "Living Room",
		DeviceClass:      "AppleTV",
		UDID:             "00008110-0000000000000000",
		Account:          "user@example.com",
		Password:         "legacy-password",
		ExpirationDate:   &expiration,
		RefreshedResult:  true,
		BundleIdentifier: "com.example.legacy",
		Version:          "1.0",
		Enabled:          true,
	}
	createLegacyDatabase(t, filepath.Join(dir, "app.db"), legacyRow)

	// Migrating twice covers the first start of the new version and a restart.
	for run := 1; run <= 2; run++ {
		initTestDb(t, dir)

		var apps []model.InstalledApp
		if err := db.Store().Find(&apps).Error; err != nil {
			t.Fatalf("run %d: read apps: %v", run, err)
		}
		if len(apps) != 1 {
			t.Fatalf("run %d: apps = %d, want 1", run, len(apps))
		}
		got := apps[0]
		if got.ID != legacyRow.ID || got.Account != legacyRow.Account || got.Password != legacyRow.Password ||
			got.BundleIdentifier != legacyRow.BundleIdentifier || got.UDID != legacyRow.UDID ||
			got.ExpirationDate == nil || !got.ExpirationDate.Equal(expiration) || !got.Enabled {
			t.Fatalf("run %d: legacy row not preserved: %+v", run, got)
		}
		if got.EffectiveSigningMode() != model.SigningModeAppleID || got.IsExternalSigning() {
			t.Fatalf("run %d: effective signing mode = %q, want %q", run, got.EffectiveSigningMode(), model.SigningModeAppleID)
		}

		data, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("run %d: marshal: %v", run, err)
		}
		var decoded struct {
			SigningMode string `json:"signing_mode"`
			Password    string `json:"password"`
		}
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("run %d: unmarshal: %v", run, err)
		}
		if decoded.SigningMode != string(model.SigningModeAppleID) || decoded.Password != "" {
			t.Fatalf("run %d: JSON signing_mode %q password %q", run, decoded.SigningMode, decoded.Password)
		}
	}
}

func TestInitDbCreatesSigningIdentities(t *testing.T) {
	initTestDb(t, t.TempDir())

	identity := &model.SigningIdentity{
		Name:              "Identity",
		CertificateSHA256: "cert",
		ProfileUUID:       "profile",
		ProfilePlatforms:  []string{"tvOS"},
		Revision:          1,
		SealedPrivateKey:  []byte("sealed"),
		CertificateDER:    []byte("der"),
		ProfileData:       []byte("profile"),
	}
	if err := db.Store().Create(identity).Error; err != nil {
		t.Fatalf("insert identity: %v", err)
	}
	var stored model.SigningIdentity
	if err := db.Store().First(&stored, identity.ID).Error; err != nil {
		t.Fatalf("read identity: %v", err)
	}
	if string(stored.SealedPrivateKey) != "sealed" || len(stored.ProfilePlatforms) != 1 || stored.ProfilePlatforms[0] != "tvOS" {
		t.Fatalf("identity not round-tripped: %+v", stored)
	}

	duplicate := *identity
	duplicate.ID = 0
	if err := db.Store().Create(&duplicate).Error; err == nil {
		t.Fatal("a second identity with the same certificate and profile was stored")
	}
}

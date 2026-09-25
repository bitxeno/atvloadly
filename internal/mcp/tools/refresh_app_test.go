package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/service"
	"github.com/bitxeno/atvloadly/internal/task"
)

// Refreshing one app signed with an external certificate queues its reinstall
// and reports the current expiry of its signing identity, which the reinstall
// gives the app, or why the reinstall will fail. The expiry stored for the app
// is never reported as the date it stays valid until.
func TestRefreshAppReportsExternalIdentityExpiry(t *testing.T) {
	day := 24 * time.Hour
	// appExpiresWithIdentity stores the current identity expiry as the app expiry.
	tests := []struct {
		name                   string
		certificateLeft        time.Duration
		profileLeft            time.Duration
		appLeft                time.Duration
		appExpiresWithIdentity bool
		missingIdentity        bool
		reportsExpiry          bool
		want                   []string
		notWant                []string
	}{
		{
			name:            "replaced profile",
			certificateLeft: 300 * day,
			profileLeft:     200 * day,
			appLeft:         10 * day,
			reportsExpiry:   true,
			notWant:         []string{"will fail", "does not extend"},
		},
		{
			name:                   "unchanged identity",
			certificateLeft:        300 * day,
			profileLeft:            200 * day,
			appExpiresWithIdentity: true,
			reportsExpiry:          true,
			want:                   []string{"does not extend"},
		},
		{
			name:            "expired profile",
			certificateLeft: 300 * day,
			profileLeft:     -day,
			appLeft:         -2 * day,
			want:            []string{"will fail", "Replace the provisioning profile"},
		},
		{
			name:            "expired certificate",
			certificateLeft: -day,
			profileLeft:     200 * day,
			appLeft:         -2 * day,
			want:            []string{"will fail", "new signing identity"},
			notWant:         []string{"Replace the provisioning profile"},
		},
		{
			name:            "missing identity",
			missingIdentity: true,
			appLeft:         10 * day,
			want:            []string{"will fail", "no longer exists"},
		},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := db.Open(db.Config{Path: t.TempDir(), FileName: "test.db"})
			if err := store.AutoMigrate(&model.InstalledApp{}, &model.SigningIdentity{}); err != nil {
				t.Fatalf("migrate: %v", err)
			}
			sqlDB, err := db.Store().DB()
			if err != nil {
				t.Fatalf("sql db: %v", err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })

			now := time.Now()
			identity := model.SigningIdentity{
				CertificateSHA256:     "cert",
				CertificateNotBefore:  now.Add(-400 * day),
				CertificateNotAfter:   now.Add(tt.certificateLeft),
				ProfileUUID:           "profile",
				ProfileExpirationDate: now.Add(tt.profileLeft),
				Revision:              1,
			}
			identityID := uint(99)
			if !tt.missingIdentity {
				if err := db.Store().Create(&identity).Error; err != nil {
					t.Fatalf("create identity: %v", err)
				}
				identityID = identity.ID
			}
			appExpiry := now.Add(tt.appLeft)
			if tt.appExpiresWithIdentity {
				appExpiry = identity.ExpiresAt()
			}
			installed := model.InstalledApp{
				// The task queue is global to the test process: one app id per case.
				Model:             gorm.Model{ID: uint(100 + i)},
				IpaName:           "App",
				Enabled:           true,
				ExpirationDate:    &appExpiry,
				SigningMode:       model.SigningModeExternalCertificate,
				SigningIdentityID: identityID,
			}
			if err := db.Store().Create(&installed).Error; err != nil {
				t.Fatalf("create app: %v", err)
			}
			stored, err := service.GetApp(installed.ID)
			if err != nil {
				t.Fatalf("load app: %v", err)
			}

			_, output, err := handleRefreshApp(context.Background(), nil, refreshAppInput{AppID: installed.ID})
			if err != nil {
				t.Fatalf("refresh_app: %v", err)
			}
			message := output.Message
			for _, want := range tt.want {
				if !strings.Contains(message, want) {
					t.Errorf("message %q does not contain %q", message, want)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(message, notWant) {
					t.Errorf("message %q contains %q", message, notWant)
				}
			}
			if tt.reportsExpiry {
				loaded, err := service.GetSigningIdentity(identityID)
				if err != nil {
					t.Fatalf("load identity: %v", err)
				}
				if want := "until " + loaded.ExpiresAt().Format(expiryLayout); !strings.Contains(message, want) {
					t.Errorf("message %q does not contain %q", message, want)
				}
			} else if strings.Contains(message, "until") {
				t.Errorf("message %q reports a validity date for a reinstall that will fail", message)
			}
			if !tt.appExpiresWithIdentity && strings.Contains(message, stored.ExpirationDate.Format(expiryLayout)) {
				t.Errorf("message %q reports the stored expiry of the app", message)
			}

			queued := false
			for _, v := range task.GetCurrentInstallingApps() {
				queued = queued || v.ID == installed.ID
			}
			if !queued {
				t.Errorf("app %d was not queued for reinstall", installed.ID)
			}
		})
	}
}

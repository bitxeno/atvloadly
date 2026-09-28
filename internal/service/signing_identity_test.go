package service

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/signing"
)

// openTestStore opens a fresh migrated database as the global store.
func openTestStore(t *testing.T) {
	t.Helper()
	store := db.Open(db.Config{Path: t.TempDir(), FileName: "test.db"})
	if err := store.AutoMigrate(&model.InstalledApp{}, &model.SigningIdentity{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB, err := db.Store().DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
}

func testIdentity(certificateSHA256, profileUUID string) *model.SigningIdentity {
	now := time.Now()
	return &model.SigningIdentity{
		Name:                  "Test identity",
		CertificateSHA256:     certificateSHA256,
		CertificateCommonName: "iPhone Distribution: Test (ABCDE12345)",
		CertificateNotBefore:  now.Add(-24 * time.Hour),
		CertificateNotAfter:   now.Add(300 * 24 * time.Hour),
		TeamID:                "ABCDE12345",
		ProfileUUID:           profileUUID,
		ProfileKind:           model.ProfileKind("ad_hoc"),
		ProfilePlatforms:      []string{"tvOS"},
		ProfileCreationDate:   now.Add(-24 * time.Hour),
		ProfileExpirationDate: now.Add(200 * 24 * time.Hour),
		Revision:              1,
		CertificateDER:        []byte("certificate-der-marker"),
		SealedPrivateKey:      []byte("sealed-private-key-marker"),
		ProfileData:           []byte("profile-data-marker"),
	}
}

func mustCreateIdentity(t *testing.T, identity *model.SigningIdentity) *model.SigningIdentity {
	t.Helper()
	if err := createSigningIdentity(identity); err != nil {
		t.Fatalf("create identity: %v", err)
	}
	return identity
}

func mustCreateExternalApp(t *testing.T, identityID uint) *model.InstalledApp {
	t.Helper()
	app := &model.InstalledApp{
		IpaName:           "Test",
		BundleIdentifier:  "com.example.test",
		SigningMode:       model.SigningModeExternalCertificate,
		SigningIdentityID: identityID,
	}
	if err := db.Store().Create(app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	return app
}

func identityExists(t *testing.T, id uint) bool {
	t.Helper()
	_, err := GetSigningIdentity(id)
	if err != nil && signing.CodeOf(err) != signing.CodeIdentityNotFound {
		t.Fatalf("get identity: %v", err)
	}
	return err == nil
}

func TestCreateSigningIdentityRefusesDuplicate(t *testing.T) {
	cases := []struct {
		name        string
		certificate string
		profile     string
		wantCode    string
	}{
		{name: "same certificate and profile", certificate: "cert-a", profile: "profile-a", wantCode: signing.CodeIdentityDuplicate},
		{name: "same certificate, other profile", certificate: "cert-a", profile: "profile-b"},
		{name: "other certificate, same profile", certificate: "cert-b", profile: "profile-a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			openTestStore(t)
			mustCreateIdentity(t, testIdentity("cert-a", "profile-a"))

			err := createSigningIdentity(testIdentity(tc.certificate, tc.profile))
			if got := signing.CodeOf(err); got != tc.wantCode {
				t.Fatalf("code = %q (err %v), want %q", got, err, tc.wantCode)
			}
			if tc.wantCode != "" && signing.ClassOf(err) != signing.ClassIdentity {
				t.Fatalf("class = %q, want %q", signing.ClassOf(err), signing.ClassIdentity)
			}
		})
	}
}

func TestSigningIdentityUniqueIndexViolationIsDuplicate(t *testing.T) {
	openTestStore(t)
	mustCreateIdentity(t, testIdentity("cert-a", "profile-a"))

	// A concurrent import passes the pre-insert check; the unique index still refuses it.
	err := db.Store().Create(testIdentity("cert-a", "profile-a")).Error
	if err == nil {
		t.Fatal("unique index did not refuse the duplicate row")
	}
	if got := signing.CodeOf(mapSigningIdentityWriteError(err)); got != signing.CodeIdentityDuplicate {
		t.Fatalf("code = %q, want %q", got, signing.CodeIdentityDuplicate)
	}
}

func TestDeleteSigningIdentity(t *testing.T) {
	cases := []struct {
		name        string
		apps        int
		deletedApps int
		force       bool
		leased      bool
		missing     bool
		wantCode    string
	}{
		{name: "unused identity", force: false},
		{name: "only deleted apps reference it", deletedApps: 1, force: false},
		{name: "referenced without force", apps: 2, force: false, wantCode: signing.CodeIdentityReferenced},
		{name: "referenced with force", apps: 2, force: true},
		{name: "leased", force: true, leased: true, wantCode: signing.CodeIdentityInUse},
		{name: "missing identity", force: true, missing: true, wantCode: signing.CodeIdentityNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			openTestStore(t)
			identity := mustCreateIdentity(t, testIdentity("cert-a", "profile-a"))
			var apps []*model.InstalledApp
			for range tc.apps {
				apps = append(apps, mustCreateExternalApp(t, identity.ID))
			}
			for range tc.deletedApps {
				app := mustCreateExternalApp(t, identity.ID)
				if _, err := DeleteApp(app.ID); err != nil {
					t.Fatalf("delete app: %v", err)
				}
			}
			if tc.leased {
				release := signing.AcquireLease(identity.ID)
				defer release()
			}
			id := identity.ID
			if tc.missing {
				id = identity.ID + 100
			}

			err := DeleteSigningIdentity(id, tc.force)
			if got := signing.CodeOf(err); got != tc.wantCode {
				t.Fatalf("code = %q (err %v), want %q", got, err, tc.wantCode)
			}
			if tc.wantCode != "" && err == nil {
				t.Fatal("expected an error")
			}
			if exists := identityExists(t, identity.ID); exists != (tc.wantCode != "") {
				t.Fatalf("identity exists = %v after delete with code %q", exists, tc.wantCode)
			}
			// Installed apps keep their rows and their reference to the identity.
			for _, app := range apps {
				stored, err := GetApp(app.ID)
				if err != nil {
					t.Fatalf("app %d lost: %v", app.ID, err)
				}
				if stored.SigningIdentityID != identity.ID {
					t.Fatalf("app %d identity = %d, want %d", app.ID, stored.SigningIdentityID, identity.ID)
				}
			}
		})
	}
}

func TestReplaceSigningIdentityProfileRefusedWhileLeased(t *testing.T) {
	openTestStore(t)
	identity := mustCreateIdentity(t, testIdentity("cert-a", "profile-a"))

	release := signing.AcquireLease(identity.ID)
	_, _, err := ReplaceSigningIdentityProfile(identity.ID, []byte("not a provisioning profile"))
	if got := signing.CodeOf(err); got != signing.CodeIdentityInUse {
		release()
		t.Fatalf("code = %q (err %v), want %q", got, err, signing.CodeIdentityInUse)
	}
	release()

	// Once released, the same request reaches profile validation.
	_, _, err = ReplaceSigningIdentityProfile(identity.ID, []byte("not a provisioning profile"))
	if err == nil || signing.CodeOf(err) == signing.CodeIdentityInUse {
		t.Fatalf("after release: err = %v, want a profile validation error", err)
	}

	stored, err := GetSigningIdentity(identity.ID)
	if err != nil {
		t.Fatalf("get identity: %v", err)
	}
	if stored.Revision != 1 || stored.ProfileUUID != "profile-a" {
		t.Fatalf("identity changed: revision %d, profile %q", stored.Revision, stored.ProfileUUID)
	}
}

func TestSaveReplacedProfile(t *testing.T) {
	openTestStore(t)
	identity := mustCreateIdentity(t, testIdentity("cert-a", "profile-a"))
	mustCreateIdentity(t, testIdentity("cert-a", "profile-taken"))
	createdAt := identity.CreatedAt

	replacement := func(profileUUID string, revision int) *model.SigningIdentity {
		updated := *identity
		updated.ProfileUUID = profileUUID
		updated.ProfileData = []byte("profile-" + profileUUID)
		updated.Revision = revision
		return &updated
	}

	stored, err := saveReplacedProfile(1, replacement("profile-b", 2))
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if stored.Revision != 2 || stored.ProfileUUID != "profile-b" || string(stored.ProfileData) != "profile-profile-b" {
		t.Fatalf("stored revision %d profile %q data %q", stored.Revision, stored.ProfileUUID, stored.ProfileData)
	}
	if !stored.CreatedAt.Equal(createdAt) {
		t.Fatalf("created_at changed: %v -> %v", createdAt, stored.CreatedAt)
	}

	cases := []struct {
		name             string
		previousRevision int
		profile          string
		wantCode         string
	}{
		{name: "stale revision", previousRevision: 1, profile: "profile-c", wantCode: signing.CodeIdentityInUse},
		{name: "profile of another identity", previousRevision: 2, profile: "profile-taken", wantCode: signing.CodeIdentityDuplicate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := saveReplacedProfile(tc.previousRevision, replacement(tc.profile, tc.previousRevision+1))
			if got := signing.CodeOf(err); got != tc.wantCode {
				t.Fatalf("code = %q (err %v), want %q", got, err, tc.wantCode)
			}
			current, err := GetSigningIdentity(identity.ID)
			if err != nil {
				t.Fatalf("get identity: %v", err)
			}
			if current.Revision != 2 || current.ProfileUUID != "profile-b" {
				t.Fatalf("identity changed: revision %d, profile %q", current.Revision, current.ProfileUUID)
			}
		})
	}
}

func TestSigningIdentityViewJSONExcludesSecrets(t *testing.T) {
	openTestStore(t)
	identity := mustCreateIdentity(t, testIdentity("cert-a", "profile-a"))
	mustCreateExternalApp(t, identity.ID)

	views, err := ListSigningIdentityViews()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("views = %d, want 1", len(views))
	}
	data, err := json.Marshal(views[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(data)

	for _, secret := range [][]byte{identity.CertificateDER, identity.SealedPrivateKey, identity.ProfileData} {
		if strings.Contains(body, string(secret)) || strings.Contains(body, base64.StdEncoding.EncodeToString(secret)) {
			t.Fatalf("view JSON leaks %q: %s", secret, body)
		}
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"certificate_der", "sealed_private_key", "profile_data", "password", "CertificateDER", "SealedPrivateKey", "ProfileData", "SigningIdentity"} {
		if _, ok := fields[key]; ok {
			t.Fatalf("view JSON has field %q: %s", key, body)
		}
	}

	var decoded struct {
		ID                uint      `json:"id"`
		CertificateSHA256 string    `json:"certificate_sha256"`
		ExpiresAt         time.Time `json:"expires_at"`
		Status            []any     `json:"status"`
		AppCount          int64     `json:"app_count"`
		InUse             bool      `json:"in_use"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ID != identity.ID || decoded.CertificateSHA256 != "cert-a" {
		t.Fatalf("identity fields not flattened: %s", body)
	}
	if !decoded.ExpiresAt.Equal(identity.ProfileExpirationDate) {
		t.Fatalf("expires_at = %v, want the profile expiration %v", decoded.ExpiresAt, identity.ProfileExpirationDate)
	}
	if decoded.Status == nil || decoded.AppCount != 1 || decoded.InUse {
		t.Fatalf("status %v, app_count %d, in_use %v: %s", decoded.Status, decoded.AppCount, decoded.InUse, body)
	}
}

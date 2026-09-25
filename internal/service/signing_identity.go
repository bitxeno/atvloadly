package service

import (
	"errors"
	"time"

	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/signing"
	"gorm.io/gorm"
)

// SigningIdentityView is the API representation of a signing identity: its
// metadata (secret material stays excluded by the json:"-" tags of the model)
// plus the effective expiry, the current findings and its usage.
type SigningIdentityView struct {
	model.SigningIdentity
	ExpiresAt time.Time       `json:"expires_at"`
	Status    []signing.Issue `json:"status"`
	AppCount  int64           `json:"app_count"`
	InUse     bool            `json:"in_use"`
}

func newSigningIdentityView(identity model.SigningIdentity, appCount int64, now time.Time) *SigningIdentityView {
	status := signing.IdentityStatus(identity, now)
	if status == nil {
		status = []signing.Issue{}
	}
	return &SigningIdentityView{
		SigningIdentity: identity,
		ExpiresAt:       identity.ExpiresAt(),
		Status:          status,
		AppCount:        appCount,
		InUse:           signing.LeaseHeld(identity.ID),
	}
}

// GetSigningIdentity loads one signing identity including its sealed material.
// A missing identity is a *signing.Error with CodeIdentityNotFound.
func GetSigningIdentity(id uint) (*model.SigningIdentity, error) {
	var identity model.SigningIdentity
	result := db.Store().Where("id = ?", id).First(&identity)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, signing.Errorf(signing.ClassIdentity, signing.CodeIdentityNotFound, "signing identity %d not found", id)
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &identity, nil
}

// ListSigningIdentities returns every signing identity, newest first.
func ListSigningIdentities() ([]model.SigningIdentity, error) {
	var identities []model.SigningIdentity
	if result := db.Store().Order("created_at desc").Find(&identities); result.Error != nil {
		return nil, result.Error
	}
	return identities, nil
}

// CountAppsBySigningIdentity counts the installed apps signed with the identity.
func CountAppsBySigningIdentity(id uint) (int64, error) {
	var count int64
	result := db.Store().Model(&model.InstalledApp{}).
		Where("signing_mode = ? AND signing_identity_id = ?", model.SigningModeExternalCertificate, id).
		Count(&count)
	return count, result.Error
}

// ListSigningIdentityViews returns the view of every signing identity, newest first.
func ListSigningIdentityViews() ([]*SigningIdentityView, error) {
	identities, err := ListSigningIdentities()
	if err != nil {
		return nil, err
	}

	var counts []struct {
		SigningIdentityID uint
		Count             int64
	}
	result := db.Store().Model(&model.InstalledApp{}).
		Select("signing_identity_id, count(*) AS count").
		Where("signing_mode = ?", model.SigningModeExternalCertificate).
		Group("signing_identity_id").
		Scan(&counts)
	if result.Error != nil {
		return nil, result.Error
	}
	appCounts := make(map[uint]int64, len(counts))
	for _, c := range counts {
		appCounts[c.SigningIdentityID] = c.Count
	}

	now := time.Now()
	views := make([]*SigningIdentityView, 0, len(identities))
	for _, identity := range identities {
		views = append(views, newSigningIdentityView(identity, appCounts[identity.ID], now))
	}
	return views, nil
}

// ImportSigningIdentity decodes, validates and stores a new signing identity.
// It returns the view of the stored identity and the non-blocking findings of
// the import. Blocking findings, a duplicate (same certificate and profile) and
// sealing failures are returned as *signing.Error.
func ImportSigningIdentity(req signing.ImportRequest) (*SigningIdentityView, []signing.Issue, error) {
	sealer, err := signing.DefaultSealer()
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	identity, issues, err := signing.NewIdentity(req, sealer, now)
	if err != nil {
		return nil, issues, err
	}
	if err := createSigningIdentity(identity); err != nil {
		return nil, issues, err
	}
	return newSigningIdentityView(*identity, 0, now), issues, nil
}

// createSigningIdentity inserts a new identity, refusing a certificate and
// profile pair that is already stored.
func createSigningIdentity(identity *model.SigningIdentity) error {
	if err := ensureUniqueSigningIdentity(db.Store(), identity.CertificateSHA256, identity.ProfileUUID, 0); err != nil {
		return err
	}
	if result := db.Store().Create(identity); result.Error != nil {
		return mapSigningIdentityWriteError(result.Error)
	}
	return nil
}

// ReplaceSigningIdentityProfile validates a new provisioning profile against
// the identity certificate and stores it atomically, incrementing Revision.
// It fails with CodeIdentityInUse while a running task holds a lease on the
// identity or when the identity changed concurrently.
func ReplaceSigningIdentityProfile(id uint, profileData []byte) (*SigningIdentityView, []signing.Issue, error) {
	var stored *model.SigningIdentity
	var issues []signing.Issue
	now := time.Now()
	err := signing.Exclusive(id, func() error {
		current, err := GetSigningIdentity(id)
		if err != nil {
			return err
		}
		updated, found, err := signing.ReplaceProfile(*current, profileData, now)
		issues = found
		if err != nil {
			return err
		}
		stored, err = saveReplacedProfile(current.Revision, updated)
		return err
	})
	if err != nil {
		return nil, issues, err
	}

	appCount, err := CountAppsBySigningIdentity(id)
	if err != nil {
		return nil, issues, err
	}
	return newSigningIdentityView(*stored, appCount, now), issues, nil
}

// saveReplacedProfile stores updated in one transaction only when the stored
// row still has previousRevision, and returns the stored row. A concurrent
// modification fails with CodeIdentityInUse, a certificate and profile pair
// already stored by another identity with CodeIdentityDuplicate.
func saveReplacedProfile(previousRevision int, updated *model.SigningIdentity) (*model.SigningIdentity, error) {
	var stored model.SigningIdentity
	err := db.Store().Transaction(func(tx *gorm.DB) error {
		if err := ensureUniqueSigningIdentity(tx, updated.CertificateSHA256, updated.ProfileUUID, updated.ID); err != nil {
			return err
		}
		result := tx.Model(&model.SigningIdentity{}).
			Where("id = ? AND revision = ?", updated.ID, previousRevision).
			Select("*").Omit("id", "created_at").
			Updates(updated)
		if result.Error != nil {
			return mapSigningIdentityWriteError(result.Error)
		}
		if result.RowsAffected == 0 {
			return signing.Errorf(signing.ClassIdentity, signing.CodeIdentityInUse,
				"signing identity %d was modified concurrently", updated.ID)
		}
		return tx.Where("id = ?", updated.ID).First(&stored).Error
	})
	if err != nil {
		return nil, err
	}
	return &stored, nil
}

// DeleteSigningIdentity removes a signing identity and its sealed key. It fails
// with CodeIdentityInUse while a running task holds a lease on the identity and
// with CodeIdentityReferenced when installed apps use it and force is false.
// Apps signed with a deleted identity keep their records; reinstalling them
// fails with CodeIdentityNotFound.
func DeleteSigningIdentity(id uint, force bool) error {
	return signing.Exclusive(id, func() error {
		if _, err := GetSigningIdentity(id); err != nil {
			return err
		}
		appCount, err := CountAppsBySigningIdentity(id)
		if err != nil {
			return err
		}
		if appCount > 0 && !force {
			return signing.Errorf(signing.ClassIdentity, signing.CodeIdentityReferenced,
				"signing identity %d is used by %d installed app(s)", id, appCount)
		}
		return db.Store().Delete(&model.SigningIdentity{}, id).Error
	})
}

// ensureUniqueSigningIdentity refuses a certificate and profile pair already
// stored by another identity than exceptID.
func ensureUniqueSigningIdentity(tx *gorm.DB, certificateSHA256, profileUUID string, exceptID uint) error {
	var count int64
	result := tx.Model(&model.SigningIdentity{}).
		Where("certificate_sha256 = ? AND profile_uuid = ? AND id <> ?", certificateSHA256, profileUUID, exceptID).
		Count(&count)
	if result.Error != nil {
		return result.Error
	}
	if count > 0 {
		return duplicateSigningIdentityError()
	}
	return nil
}

// mapSigningIdentityWriteError maps a unique index violation on the
// certificate and profile pair to CodeIdentityDuplicate.
func mapSigningIdentityWriteError(err error) error {
	if translator, ok := db.Store().Dialector.(gorm.ErrorTranslator); ok {
		if errors.Is(translator.Translate(err), gorm.ErrDuplicatedKey) {
			return duplicateSigningIdentityError()
		}
	}
	return err
}

func duplicateSigningIdentityError() error {
	return signing.Errorf(signing.ClassIdentity, signing.CodeIdentityDuplicate,
		"a signing identity with the same certificate and provisioning profile already exists")
}

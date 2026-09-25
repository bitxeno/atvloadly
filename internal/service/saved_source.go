package service

import (
	"errors"
	"net/url"
	"sync"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/bitxeno/atvloadly/internal/db"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/source"
)

// CatalogSource is the apps of one saved source for the catalog.
type CatalogSource struct {
	ID     uint           `json:"id"`
	URL    string         `json:"url"`
	Name   string         `json:"name"`   // name read from the source, or the saved name when it could not be read
	Error  string         `json:"error"`  // why the source could not be read ("" on success)
	Builds []source.Build `json:"builds"` // newest version of each app (Feed.Preview builds), [] on error
}

// GetSavedSources returns the saved AltStore sources, oldest first.
func GetSavedSources() ([]model.SavedSource, error) {
	sources := []model.SavedSource{}
	if result := db.Store().Order("id").Find(&sources); result.Error != nil {
		return nil, result.Error
	}

	return sources, nil
}

// findSavedSource returns the saved source whose URL is u.
func findSavedSource(u string) (*model.SavedSource, error) {
	var saved model.SavedSource
	if result := db.Store().Where("url = ?", u).First(&saved); result.Error != nil {
		return nil, result.Error
	}

	return &saved, nil
}

// AddSavedSource saves the AltStore source at location. It fetches the source to check it and to read its name;
// saving a URL that is already saved returns the existing entry.
func AddSavedSource(location string) (*model.SavedSource, error) {
	u, err := source.Normalize(source.KindAltStore, location)
	if err != nil {
		return nil, err
	}
	saved, err := findSavedSource(u)
	if err == nil {
		return saved, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	feed, err := fetchSource(source.KindAltStore, u)
	if err != nil {
		return nil, err
	}
	preview, err := feed.Preview("", false)
	if err != nil {
		return nil, err
	}
	name := preview.Title
	if name == "" {
		// Normalize accepted u, so it parses.
		parsed, _ := url.Parse(u)
		name = parsed.Host
	}

	// A concurrent add of the same URL keeps the first entry.
	row := model.SavedSource{URL: u, Name: name}
	if result := db.Store().Clauses(clause.OnConflict{DoNothing: true}).Create(&row); result.Error != nil {
		return nil, result.Error
	}
	return findSavedSource(u)
}

// DeleteSavedSource removes a saved source (hard delete). Apps tracking it are not affected.
func DeleteSavedSource(id uint) error {
	return db.Store().Delete(&model.SavedSource{}, id).Error
}

// GetSourceCatalog fetches every saved source concurrently (one request each) and returns their apps for a device of
// deviceClass, in saved order. A failing source is reported in Error and does not fail the others.
func GetSourceCatalog(deviceClass string) ([]CatalogSource, error) {
	saved, err := GetSavedSources()
	if err != nil {
		return nil, err
	}

	catalog := make([]CatalogSource, len(saved))
	var wg sync.WaitGroup
	for i, s := range saved {
		wg.Go(func() { catalog[i] = catalogSource(s, deviceClass) })
	}
	wg.Wait()
	return catalog, nil
}

// catalogSource fetches the saved source s and lists its apps.
func catalogSource(s model.SavedSource, deviceClass string) CatalogSource {
	c := CatalogSource{ID: s.ID, URL: s.URL, Name: s.Name, Builds: []source.Build{}}
	feed, err := fetchSource(source.KindAltStore, s.URL)
	var preview *source.Preview
	if err == nil {
		preview, err = feed.Preview(deviceClass, false)
	}
	if err != nil {
		c.Error = err.Error()
		return c
	}

	if preview.Title != "" {
		c.Name = preview.Title
	}
	c.Builds = preview.Builds
	return c
}

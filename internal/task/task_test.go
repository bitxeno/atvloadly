package task

import (
	"testing"

	"github.com/bitxeno/atvloadly/internal/model"
)

func TestShouldUseRefreshMode(t *testing.T) {
	newApp := model.InstalledApp{}
	if shouldUseRefreshMode(newApp) {
		t.Fatal("new app install should not use refresh mode")
	}

	existingApp := model.InstalledApp{}
	existingApp.ID = 1
	if !shouldUseRefreshMode(existingApp) {
		t.Fatal("existing app refresh should use refresh mode")
	}
}

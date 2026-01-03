package manager

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
)

var accountManager = newAccountManager()

type AccountManager struct{}

func newAccountManager() *AccountManager {
	return &AccountManager{}
}

// GetAccounts reads account file and returns simplified account info.
// The accounts.json may be an array or an object; normalize both to a slice.
func (am *AccountManager) GetAccounts() (*model.Accounts, error) {
	path := filepath.Join(app.SideloadDataDir(), "accounts.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &model.Accounts{}, nil
		}
		return nil, err
	}

	// 1) try as array of accounts
	var a model.Accounts
	if err := json.Unmarshal(data, &a); err != nil {
		return &model.Accounts{}, nil
	}

	return &a, nil
}

func (am *AccountManager) DeleteAccount(email string) error {
	_, err := ExecuteCommand("plumesign", "account", "logout", "-u", email)
	if err != nil {
		log.Err(err).Msgf("Error delete account: %s", email)
		return err
	}
	return nil
}

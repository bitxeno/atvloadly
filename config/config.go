package config

func Load() error {
	if err := loadApp(); err != nil {
		return err
	}

	return loadSettings()
}

package secret

import "sync"

// Configure sets the dedicated key file. Call once at startup.
func Configure(keyFile string) {
	configMu.Lock()
	defer configMu.Unlock()
	if keyFile != configKeyFile {
		defaultStore = nil
	}
	configKeyFile = keyFile
}

var (
	configMu      sync.Mutex
	configKeyFile string
	defaultStore  *Store
)

// Default returns the store of the configured key file, loading or creating
// it on first use. A failed load is retried on the next call.
func Default() (*Store, error) {
	configMu.Lock()
	defer configMu.Unlock()
	if defaultStore != nil {
		return defaultStore, nil
	}
	if configKeyFile == "" {
		return nil, ErrUnavailable
	}
	s, err := LoadOrCreate(configKeyFile)
	if err != nil {
		return nil, err
	}
	defaultStore = s
	return s, nil
}

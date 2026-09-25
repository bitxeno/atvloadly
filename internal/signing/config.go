package signing

import "sync"

// Upload limits. HTTP handlers reject larger files before reading them and
// the decoders reject larger inputs.
const (
	MaxP12Size     = 1 << 20
	MaxProfileSize = 2 << 20
)

var (
	configMu      sync.Mutex
	configKeyFile string
	configRoot    string
	defaultSealer *Sealer
)

// Configure sets the deployment key file and the workspace root. Call once at startup.
func Configure(keyFile, workRoot string) {
	configMu.Lock()
	defer configMu.Unlock()
	if keyFile != configKeyFile {
		defaultSealer = nil
	}
	configKeyFile = keyFile
	configRoot = workRoot
}

// DefaultSealer returns the sealer of the configured key file, loading or
// creating it on first use. A failed load is retried on the next call.
func DefaultSealer() (*Sealer, error) {
	configMu.Lock()
	defer configMu.Unlock()
	if defaultSealer != nil {
		return defaultSealer, nil
	}
	if configKeyFile == "" {
		return nil, Errorf(ClassIdentity, CodeKeyUnavailable, "the signing key file is not configured")
	}
	sealer, err := LoadOrCreateSealer(configKeyFile)
	if err != nil {
		return nil, err
	}
	defaultSealer = sealer
	return sealer, nil
}

func workspaceRoot() string {
	configMu.Lock()
	defer configMu.Unlock()
	return configRoot
}

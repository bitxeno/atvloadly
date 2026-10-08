package manager

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
	"github.com/bitxeno/atvloadly/internal/signing"
)

var certificateManager = newCertificateManager()

type CertificateManager struct{}

func newCertificateManager() *CertificateManager {
	return &CertificateManager{}
}

func (m *CertificateManager) GetCertificates(email string) ([]model.Certificate, error) {
	output, err := exec.NewCommand("plumesign", "certificate", "list", "-u", email).
		WithDir(app.Config.Server.DataDir).
		WithEnv(GetRunEnvs()).
		CombinedOutput()
	if err != nil {
		log.Err(err).Msgf("Error getting certificates for %s", email)
		return nil, err
	}

	var certs []model.Certificate
	// Regex to parse the output by extracting contents between backticks
	re := regexp.MustCompile("-\\s+`([^`]+)`.*`([^`]+)`.*`([^`]+)`.*`([^`]+)`.*`([^`]+)`.*`([^`]+)`")

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) > 6 {
			cert := model.Certificate{
				Name:           matches[1],
				MachineName:    matches[6],
				Status:         matches[3],
				InUse:          matches[4] == "1",
				SerialNumber:   matches[2],
				ExpirationDate: matches[5],
			}
			certs = append(certs, cert)
		}
	}

	return certs, nil
}

func (m *CertificateManager) RevokeCertificate(email string, serialNumber string) error {
	_, err := exec.NewCommand("plumesign", "certificate", "revoke", "-u", email, "-s", serialNumber).
		WithDir(app.Config.Server.DataDir).
		WithEnv(GetRunEnvs()).
		CombinedOutput()
	if err != nil {
		log.Err(err).Msgf("Error revoking certificate %s", serialNumber)
		return err
	}
	return nil
}

func (m *CertificateManager) ExportCertificate(email, password, path string) (string, error) {
	output, err := exec.NewCommand("plumesign", "certificate", "export", "-u", email, "-p", password, "-o", path).
		WithDir(app.Config.Server.DataDir).
		WithEnv(GetRunEnvs()).
		WithSecret(password).
		CombinedOutput()
	if err != nil {
		log.Err(err).Msgf("Error exporting certificate for %s", email)
		return string(output), err
	}
	return string(output), nil
}

// ImportCertificate imports the certificate of the PKCS#12 file p12 for email
// through the signing engine.
//
// The file is decoded in process and written again as a legacy PKCS#12 file
// before the engine runs. SideStore's "Export Full (.p12)" stores the private
// key as a plain keyBag without a MAC, which the engine's reader reports as
// holding no private key.
//
// password only decodes the input here: the engine is handed the throwaway
// password of the re-wrapped file, so the password of the user never reaches
// the command line or the debug log. The input is never written to disk; only
// the re-wrapped copy is, for the duration of the engine run.
func (m *CertificateManager) ImportCertificate(email, password string, p12 []byte) error {
	rewrapped, rewrapPassword, err := signing.RewrapP12(p12, password)
	if err != nil {
		return err
	}

	path, cleanup, err := writeTempP12(rewrapped)
	if err != nil {
		return err
	}
	defer cleanup()

	output, err := exec.NewCommand("plumesign", "certificate", "import", "-u", email, "-p", rewrapPassword, "-i", path).
		WithDir(app.Config.Server.DataDir).
		WithEnv(GetRunEnvs()).
		WithSecret(rewrapPassword).
		CombinedOutput()
	if err != nil {
		log.Err(err).Msgf("Error importing certificate for %s: %s", email, string(output))
		return err
	}
	return nil
}

// writeTempP12 stores data in a private temporary file and returns its path
// and a cleanup function. os.CreateTemp creates the file with mode 0600.
func writeTempP12(data []byte) (string, func(), error) {
	file, err := os.CreateTemp("", "atvloadly-cert-*.p12")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create a temporary certificate file: %w", err)
	}
	cleanup := func() { _ = os.Remove(file.Name()) }
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, fmt.Errorf("failed to write the temporary certificate file: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to write the temporary certificate file: %w", err)
	}
	return file.Name(), cleanup, nil
}

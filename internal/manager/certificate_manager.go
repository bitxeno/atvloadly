package manager

import (
	"regexp"
	"strings"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/exec"
	"github.com/bitxeno/atvloadly/internal/log"
	"github.com/bitxeno/atvloadly/internal/model"
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
		CombinedOutput()
	if err != nil {
		log.Err(err).Msgf("Error exporting certificate for %s", email)
		return string(output), err
	}
	return string(output), nil
}

func (m *CertificateManager) ImportCertificate(email, password, path string) error {
	output, err := exec.NewCommand("plumesign", "certificate", "import", "-u", email, "-p", password, "-i", path).
		WithDir(app.Config.Server.DataDir).
		WithEnv(GetRunEnvs()).
		CombinedOutput()
	if err != nil {
		log.Err(err).Msgf("Error importing certificate for %s: %s", email, string(output))
		return err
	}
	return nil
}

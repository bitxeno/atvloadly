package lib

import (
	"crypto/x509"
	"log"
	"os"
)

// ReadCertPool get CertPool by crt file
func ReadCertPool(crt string) *x509.CertPool {
	_crt, err := os.ReadFile(crt)
	if err != nil {
		log.Fatalln("Read crt file failed:", err.Error())
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(_crt) {
		log.Fatalln("Load crt file failed.")
	}
	return pool
}

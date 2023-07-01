package lib

import (
	"crypto/x509"
	"io/ioutil"
	"log"
)

// ReadCertPool get CertPool by crt file
func ReadCertPool(crt string) *x509.CertPool {
	_crt, err := ioutil.ReadFile(crt)
	if err != nil {
		log.Fatalln("Read crt file failed:", err.Error())
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(_crt) {
		log.Fatalln("Load crt file failed.")
	}
	return pool
}

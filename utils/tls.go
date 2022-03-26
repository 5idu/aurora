package utils

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
)

func GetTlsCfg(ce, key, ca string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(ce, key)
	if err != nil {
		return nil, err
	}

	caData, err := ioutil.ReadFile(ca)
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caData)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}, nil
}

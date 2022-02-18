package tls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
)

// LoadConfig loads a pre-defined configuration using the files provided.
// Accepts `ca` the Public Key of the certificate authority, `cert`
// the certificate and `key` the private key.
func LoadConfig(ca, cert, key string) (*tls.Config, error) {
	// Load the system certificate pool
	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	// If a CA file is provided, load it and add it
	// to the system certificate pool
	if ca != "" {
		// Load the CA certificate
		var caCert []byte
		caCert, err = os.ReadFile(ca)
		if err != nil {
			return nil, err
		}

		ok := caCertPool.AppendCertsFromPEM(caCert)
		if !ok {
			return nil, errors.New("failed to parse root certificate")
		}
	}

	c, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		MinVersion:               tls.VersionTLS13,
		RootCAs:                  caCertPool,
		ClientCAs:                caCertPool,
		ClientAuth:               tls.RequireAndVerifyClientCert,
		Certificates:             []tls.Certificate{c},
		PreferServerCipherSuites: true,
	}, nil
}

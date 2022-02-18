package tls

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	CERT = "CERTIFICATE"
	PK   = "RSA PRIVATE KEY"
)

func NewCA(
	basePath string,
	certFile string,
	keyFile string,
	subject *pkix.Name,
	keysize int,
) (*x509.Certificate, *rsa.PrivateKey, error) {
	// NOTE: This is not ideal because there is no knowledge of previous
	// certificates in the chain. Using a random serial number is a
	// temporary solution for the example. In reality, a real CA should
	// create a random serial number that does not collide with previous
	// certificates.
	serial, err := rand.Int(rand.Reader, big.NewInt(100000000000))
	if err != nil {
		return nil, nil, err
	}

	// NOTE: I'm using a 10 year expiry date for this example. This is
	// not a realistic expiration date.
	ca := &x509.Certificate{
		SerialNumber: serial,
		Subject:      *subject,
		NotBefore:    time.Now(),                   // No backdating
		NotAfter:     time.Now().AddDate(10, 0, 0), // 10 years
		IsCA:         true,                         // flag as CA
		KeyUsage: x509.KeyUsageDigitalSignature |
			x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// Generate the key for the CA
	caPrivKey, err := rsa.GenerateKey(rand.Reader, keysize)
	if err != nil {
		return nil, nil, err
	}

	caBytes, err := x509.CreateCertificate(
		rand.Reader,
		ca,
		ca,
		&caPrivKey.PublicKey,
		caPrivKey,
	)
	if err != nil {
		return nil, nil, err
	}

	caPEM := new(bytes.Buffer)
	_ = pem.Encode(caPEM, &pem.Block{
		Type:  CERT,
		Bytes: caBytes,
	})

	caPrivKeyPEM := new(bytes.Buffer)
	_ = pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  PK,
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	err = os.WriteFile(
		filepath.Join(basePath, certFile),
		caPEM.Bytes(),
		0600,
	)
	if err != nil {
		return nil, nil, err
	}

	err = os.WriteFile(
		filepath.Join(basePath, keyFile),
		caPrivKeyPEM.Bytes(),
		0600,
	)
	if err != nil {
		return nil, nil, err
	}

	return ca, caPrivKey, nil
}

func NewCert(
	basePath string,
	ca *x509.Certificate,
	caPrivKey *rsa.PrivateKey,
	keysize int,
	server bool,
	subjects ...*pkix.Name,
) error {
	for _, subject := range subjects {
		serial, err := rand.Int(rand.Reader, big.NewInt(1000000))
		if err != nil {
			return err
		}

		var filename = strconv.Itoa(int(serial.Int64()))
		if len(subject.Organization) > 0 {
			filename = subject.Organization[0]
		}

		if len(subject.OrganizationalUnit) > 0 {
			filename = fmt.Sprintf(
				"%s_%s",
				filename, strings.Join(subject.OrganizationalUnit, "-"),
			)
		}

		var usages []x509.ExtKeyUsage
		if server {
			usages = append(usages, x509.ExtKeyUsageServerAuth)
		} else {
			usages = append(usages, x509.ExtKeyUsageClientAuth)
		}

		cert := &x509.Certificate{
			SerialNumber: serial,
			Subject:      *subject,
			NotBefore:    time.Now(),                   // No backdating
			NotAfter:     time.Now().AddDate(10, 0, 0), // 10 years
			ExtKeyUsage:  usages,
			KeyUsage:     x509.KeyUsageDigitalSignature,

			// NOTE: These are only hard-coded for the example. These
			// should be setup for a specific domain or host.
			DNSNames:    []string{"localhost"},
			IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		}

		certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			panic(err)
		}

		certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
		if err != nil {
			panic(err)
		}

		certPEM := new(bytes.Buffer)
		_ = pem.Encode(certPEM, &pem.Block{
			Type:  CERT,
			Bytes: certBytes,
		})

		certPrivKeyPEM := new(bytes.Buffer)
		_ = pem.Encode(certPrivKeyPEM, &pem.Block{
			Type:  PK,
			Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
		})

		err = os.WriteFile(
			filepath.Join(basePath, fmt.Sprintf("%s.cert", filename)),
			certPEM.Bytes(),
			0600,
		)
		if err != nil {
			return err
		}

		err = os.WriteFile(
			filepath.Join(basePath, fmt.Sprintf("%s.key", filename)),
			certPrivKeyPEM.Bytes(),
			0600,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

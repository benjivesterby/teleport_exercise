package main

import (
	"crypto/x509/pkix"
	"flag"
	"fmt"
	"os"

	"go.benjiv.com/sandbox/internal/tls"
)

var basepath *string

func main() {
	basepath = flag.String("basepath", "", "base path for certs")
	flag.Parse()

	err := Generate()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func Generate() error {
	ca, caPrivKey, err := tls.NewCA(
		*basepath,
		"ca.cert",
		"ca.key",
		&pkix.Name{
			Organization:  []string{"Company Name"},
			Country:       []string{"US"},
			Province:      []string{"Raleigh"},
			Locality:      []string{"North Carolina"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		}, 4096)

	if err != nil {
		return err
	}

	// generate server cert
	err = tls.NewCert(*basepath, ca, caPrivKey, 4096, true, &pkix.Name{
		Organization:  []string{"server"},
		Country:       []string{"US"},
		Province:      []string{"Raleigh"},
		Locality:      []string{"North Carolina"},
		StreetAddress: []string{""},
		PostalCode:    []string{""},
	})

	if err != nil {
		return err
	}

	// generate client certs
	return tls.NewCert(*basepath, ca, caPrivKey, 4096, false, &pkix.Name{
		Organization:       []string{"it"},
		OrganizationalUnit: []string{"admin"},
		Country:            []string{"US"},
		Province:           []string{"Raleigh"},
		Locality:           []string{"North Carolina"},
		StreetAddress:      []string{""},
		PostalCode:         []string{""},
	}, &pkix.Name{
		Organization:       []string{"it"},
		OrganizationalUnit: []string{"user"},
		Country:            []string{"US"},
		Province:           []string{"Raleigh"},
		Locality:           []string{"North Carolina"},
		StreetAddress:      []string{""},
		PostalCode:         []string{""},
	}, &pkix.Name{
		Organization:       []string{"hr"},
		OrganizationalUnit: []string{"user"},
		Country:            []string{"US"},
		Province:           []string{"Raleigh"},
		Locality:           []string{"North Carolina"},
		StreetAddress:      []string{""},
		PostalCode:         []string{""},
	}, &pkix.Name{
		Organization:       []string{"invalid"},
		OrganizationalUnit: []string{"admin"},
		Country:            []string{"US"},
		Province:           []string{"Raleigh"},
		Locality:           []string{"North Carolina"},
		StreetAddress:      []string{""},
		PostalCode:         []string{""},
	})
}

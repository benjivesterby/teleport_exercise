package proto

import (
	"crypto/tls"
	"net"
	"testing"

	mytls "go.benjiv.com/sandbox/internal/tls"
)

func Test_mTLS(t *testing.T) {
	testdata := map[string]struct {
		serverca   string
		servercert string
		serverkey  string
		clientca   string
		clientcert string
		clientkey  string
		success    bool
	}{
		"valid-shared-ca": {
			serverca:   "./testdata/ca.cert",
			servercert: "./testdata/server.cert",
			serverkey:  "./testdata/server.key",
			clientca:   "./testdata/ca.cert",
			clientcert: "./testdata/invalid_admin.cert",
			clientkey:  "./testdata/invalid_admin.key",
			success:    true,
		},
	}

	for name, test := range testdata {
		t.Run(name, func(t *testing.T) {
			// Load the TLS certificates and create a config.
			serverConfig, err := mytls.LoadConfig(test.serverca, test.servercert, test.serverkey)
			if err != nil {
				t.Fatal(err)
			}

			t.Log("starting server")
			ln, err := tls.Listen("tcp", ":0", serverConfig)
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()

			t.Log("loading client config")
			// Create a client config.
			// clientConfig, err := mytls.LoadConfig(test.clientca, test.clientcert, test.clientkey)
			// if err != nil {
			// 	t.Fatal(err)
			// }

			t.Log("dialing server")
			// Create a client.
			// conn, err := tls.Dial("tcp", ln.Addr().String(), clientConfig)
			conn, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
		})
	}
}

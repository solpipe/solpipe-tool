package proxy

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"time"

	sgo "github.com/SolmateDev/solana-go"
)

/*
we need to fix this page to get direct tls connections authenticated by Solana TLS keys.
Otherwise, we can only use TOR.
*/

//const ED25519_IDENTIFIER: [u32; 4] = [1, 3, 101, 112];

func ED25519_IDENTIFIER() [4]uint32 {
	return [4]uint32{1, 3, 101, 112}
}
func pemBlockForKey(priv interface{}) (*pem.Block, error) {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}, nil
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return nil, err
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}, nil
	case *ed25519.PrivateKey:
		b, err := x509.MarshalPKCS8PrivateKey(priv)
		if err != nil {
			return nil, err
		}
		return &pem.Block{Type: "PRIVATE KEY", Bytes: b}, nil
	default:
		return nil, errors.New("unknown format")
	}
}

type TlsForServer struct {
	Private     []byte
	Certificate []byte
}

func NewSelfSignedTlsCertificateChainServer(key sgo.PrivateKey, dnsAddr []string, expire time.Time) (*TlsForServer, error) {
	priv := ed25519.PrivateKey(key)
	pub := priv.Public()
	if time.Now().Add(24 * time.Hour).After(expire) {
		return nil, errors.New("bad expire time")
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: dnsAddr,
		},
		NotBefore:   time.Now(),
		NotAfter:    expire,
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, pub, priv)
	if err != nil {
		return nil, err
	}
	out := &bytes.Buffer{}
	err = pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return nil, err
	}
	cert := out.Bytes()
	out.Reset()
	o1, err := pemBlockForKey(priv)
	if err != nil {
		return nil, err
	}
	err = pem.Encode(out, o1)
	if err != nil {
		return nil, err
	}
	sslprivkey := out.Bytes()

	return &TlsForServer{
		Private:     sslprivkey,
		Certificate: cert,
	}, nil

}

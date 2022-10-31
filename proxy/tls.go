package proxy

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
)

/*
we need to fix this page to get direct tls connections authenticated by Solana TLS keys.
Otherwise, we can only use TOR.
*/

//const ED25519_IDENTIFIER: [u32; 4] = [1, 3, 101, 112];

//	func ED25519_IDENTIFIER() [4]uint32 {
//		return [4]uint32{1, 3, 101, 112}
//	}

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
	case ed25519.PrivateKey:
		b, err := x509.MarshalPKCS8PrivateKey(k)
		if err != nil {
			return nil, err
		}
		return &pem.Block{Type: "PRIVATE KEY", Bytes: b, Headers: nil}, nil
	default:
		return nil, errors.New("unknown format")
	}
}

type TlsForServer struct {
	Private     []byte
	Certificate []byte
	Ca          []byte
}

func (tfs TlsForServer) CertAndKey() (cert tls.Certificate, err error) {
	//return cert, nil
	//os.Stderr.WriteString(string(tfs.Certificate) + "\n")
	//os.Stderr.WriteString(string(tfs.Private) + "\n")
	return tls.X509KeyPair(tfs.Certificate, tfs.Private)
}

func generateCa(key sgo.PrivateKey) ([]byte, *ed25519.PrivateKey, error) {
	caPrivKey := ed25519.PrivateKey(key)
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"Golden Gate Bridge"},
			PostalCode:    []string{"94016"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, caPrivKey.Public(), caPrivKey)
	if err != nil {
		return nil, nil, err
	}

	return caBytes, &caPrivKey, nil
}

func generateEphemeralCert(
	caBytes []byte,
	caPrivKey ed25519.PrivateKey,
) ([]byte, *ed25519.PrivateKey, error) {
	ca, err := x509.ParseCertificate(caBytes)
	if err != nil {
		return nil, nil, err
	}

	pub, certPrivKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization:  []string{"Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"Golden Gate Bridge"},
			PostalCode:    []string{"94016"},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, pub, caPrivKey)
	if err != nil {
		return nil, nil, err
	}
	checkCert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, err
	}
	certPool := x509.NewCertPool()
	certPool.AddCert(ca)
	chain, err := checkCert.Verify(x509.VerifyOptions{
		Roots: certPool,
	})
	if err != nil {
		return nil, nil, err
	}
	for i := 0; i < len(chain); i++ {
		for j := 0; j < len(chain[i]); j++ {
			log.Debugf("(%d,%d)=%+v", i, j, chain[i][j].PublicKey)
		}
	}
	return certBytes, &certPrivKey, nil
}

func serializePrivateKeyToPEM(priv *ed25519.PrivateKey) ([]byte, error) {
	if priv == nil {
		return nil, errors.New("no key")
	}
	keyOut := &bytes.Buffer{}
	o1, err := pemBlockForKey(priv)
	if err != nil {
		return nil, err
	}
	err = pem.Encode(keyOut, o1)
	if err != nil {
		return nil, err
	}
	return keyOut.Bytes(), nil
}

func serializeCertDerToPem(certDer []byte) []byte {
	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDer,
	})
	return certPEM.Bytes()
}

func NewSelfSignedTlsCertificateChainServer(
	key sgo.PrivateKey,
	dnsAddr []string,
	expire time.Time,
) (*tls.Certificate, error) {

	ca, capriv, err := generateCa(key)
	if err != nil {
		return nil, err
	}
	cert, priv, err := generateEphemeralCert(ca, *capriv)
	if err != nil {
		return nil, err
	}

	ans := new(tls.Certificate)

	// leaf first
	ans.Certificate = [][]byte{cert, ca}
	ans.PrivateKey = *priv

	return ans, nil
}

const NAME_CERTIFICATE = "CERTIFICATE"

func VerifyCertificate(cert []byte, pubkey sgo.PublicKey) bool {
	block, _ := pem.Decode(cert)
	if block == nil {
		return false
	}
	if block.Type != NAME_CERTIFICATE {
		return false
	}
	parsedCertificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	if parsedCertificate.PublicKeyAlgorithm != x509.Ed25519 {
		return false
	}
	derivedPubkey := parsedCertificate.PublicKey
	x, ok := derivedPubkey.(ed25519.PublicKey)
	if !ok {
		return false
	}
	return sgo.PublicKeyFromBytes(x).Equals(pubkey)

}

type ServerConfiguration struct {
	Port uint16   `json:"port"`
	Host []string `json:"host"`
}

func (sc ServerConfiguration) dns_string() []string {
	ans := make([]string, len(sc.Host))
	for i := 0; i < len(sc.Host); i++ {
		ans[i] = fmt.Sprintf("%s:%d", sc.Host[i], sc.Port)
	}
	return ans
}

// use inside the TlsConfig Verify Peer Certificate argument
func verify(destination sgo.PublicKey, rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	ans := false
out:
	for i := 0; i < len(rawCerts); i++ {
		ans = VerifyCertificate(rawCerts[i], destination)
		if ans {
			break out
		}
	}
	return nil
}

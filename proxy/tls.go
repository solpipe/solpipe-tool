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
	"os"
	"strings"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	"golang.org/x/crypto/ssh"
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
}

func (tfs TlsForServer) CertAndKey() (cert tls.Certificate, err error) {

	cert.PrivateKey, err = ssh.ParseRawPrivateKey(tfs.Private)
	if err != nil {
		return cert, err
	}
	var skippedBlockTypes []string
	certPEMBlock := tfs.Certificate
	for {
		var certDERBlock *pem.Block
		certDERBlock, certPEMBlock = pem.Decode(certPEMBlock)
		if certDERBlock == nil {
			break
		}
		if certDERBlock.Type == "CERTIFICATE" {
			cert.Certificate = append(cert.Certificate, certDERBlock.Bytes)
		} else {
			skippedBlockTypes = append(skippedBlockTypes, certDERBlock.Type)
		}
	}
	if len(cert.Certificate) == 0 {
		if len(skippedBlockTypes) == 0 {
			return cert, errors.New("tls: failed to find any PEM data in certificate input")
		}
		if len(skippedBlockTypes) == 1 && strings.HasSuffix(skippedBlockTypes[0], "PRIVATE KEY") {
			return cert, errors.New("tls: failed to find certificate PEM data in certificate input, but did find a private key; PEM inputs may have been switched")
		}
		return cert, fmt.Errorf("tls: failed to find \"CERTIFICATE\" PEM block in certificate input after skipping PEM blocks of the following types: %v", skippedBlockTypes)
	}

	//return cert, nil
	os.Stderr.WriteString(string(tfs.Certificate) + "\n")
	os.Stderr.WriteString(string(tfs.Private) + "\n")
	return tls.X509KeyPair(tfs.Certificate, tfs.Private)
}

func NewSelfSignedTlsCertificateChainServer(
	key sgo.PrivateKey,
	dnsAddr []string,
	expire time.Time,
) (*TlsForServer, error) {

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
	err = pem.Encode(out, &pem.Block{Type: NAME_CERTIFICATE, Bytes: derBytes})
	if err != nil {
		return nil, err
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

	return &TlsForServer{
		Private:     keyOut.Bytes(),
		Certificate: out.Bytes(),
	}, nil

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

func (sc ServerConfiguration) Tls(admin sgo.PrivateKey) (*TlsForServer, error) {
	return NewSelfSignedTlsCertificateChainServer(
		admin,
		sc.dns_string(),
		time.Now().Add(36*time.Hour),
	)
}

func createClientCert(
	admin sgo.PrivateKey,
) (ans *tls.Certificate, err error) {

	var x *TlsForServer
	x, err = NewSelfSignedTlsCertificateChainServer(
		admin,
		[]string{fmt.Sprintf("client.%s", admin.PublicKey())},
		time.Now().Add(7*24*time.Hour),
	)
	if err != nil {
		return
	}
	ans = new(tls.Certificate)
	*ans, err = x.CertAndKey()
	if err != nil {
		return
	}
	return
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

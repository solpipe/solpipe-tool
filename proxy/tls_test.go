package proxy_test

import (
	"testing"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/solpipe/solpipe-tool/proxy"
)

func TestTls(t *testing.T) {

	key1, err := sgo.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	tlsCertPrivKey, err := proxy.NewSelfSignedTlsCertificateChainServer(key1, []string{"localhost:2003"}, time.Now().Add(32*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !proxy.VerifyCertificate(tlsCertPrivKey.Certificate, key1.PublicKey()) {
		t.Fatal("public key does not match certificate")
	}

	key2, err := sgo.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	if proxy.VerifyCertificate(tlsCertPrivKey.Certificate, key2.PublicKey()) {
		t.Fatal("public key should not match, but does")
	}

}

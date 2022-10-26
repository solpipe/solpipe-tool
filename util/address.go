package util

import (
	"bytes"
	"crypto/ed25519"
	crypto_rand "crypto/rand"

	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	sgo "github.com/SolmateDev/solana-go"
	bin "github.com/gagliardetto/binary"
	"golang.org/x/crypto/sha3"
)

func readSeed() (seed []byte, err error) {
	rand := crypto_rand.Reader
	seed = make([]byte, ed25519.SeedSize)
	if _, err = io.ReadFull(rand, seed); err != nil {
		return
	}
	return
}

func iterateSeed(seed []byte, i uint64) (reader io.Reader, err error) {
	bigArray := make([]byte, len(seed)+8)
	writer := bin.NewBinEncoder(bytes.NewBuffer(bigArray))

	err = writer.WriteUint64(i, binary.BigEndian)
	if err != nil {
		return
	}
	for i := 0; i < len(seed); i++ {
		err = writer.WriteByte(seed[i])
		if err != nil {
			return
		}
	}
	x := sha512.Sum512(bigArray)
	reader = bytes.NewBuffer(x[:])

	return
}

const DEFAULT_PROXY_PORT = 50051
const ONION_VERSION = 3

func GenerateOnionAddressFromSolanaPublicKey(pubKey sgo.PublicKey) (string, error) {
	return GetOnionID(ed25519.PublicKey(pubKey.Bytes()))
}

func GenerateVanityOnionAddressSolanaPrivatekey(prefix string, maxTries uint64) (sgo.PrivateKey, error) {
	k, err := GenerateVanityAddress(prefix, maxTries)
	if err != nil {
		return nil, err
	}
	return sgo.PrivateKey([]byte(k)), nil
}

func reverseBytes(a []byte) []byte {
	n := len(a)
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		b[i] = a[n-i-1]
	}
	return b
}

// from https://github.com/torproject/tor/blob/22552ad88e1e95ef9d2c6655c7602b7b25836075/src/test/hs_build_address.py
func GetOnionID(pubKey ed25519.PublicKey) (address string, err error) {
	pubKeyData := []byte(pubKey)
	sha256Size := sha3.New256().Size()
	if len(pubKeyData) != sha256Size {
		err = fmt.Errorf("pubkey is not %d", sha256Size)
		return
	}

	// CHECKSUM = H(".onion checksum" | PUBKEY | VERSION)[:2]
	hasher := sha3.New256()
	//hasher := sha512.New512_256()
	textPart := []byte(".onion checksum")
	var n int
	sum := 0
	n, err = hasher.Write(textPart)
	//_, err = hasher.Write(reverseBytes([]byte(".onion checksum")))
	if err != nil {
		return
	}
	sum += n
	n, err = hasher.Write(pubKeyData)
	if err != nil {
		return
	}
	sum += n
	n, err = hasher.Write([]byte{byte(ONION_VERSION)})
	if err != nil {
		return
	}
	sum += n
	checkSum := hasher.Sum(nil)
	// onion_address = base32(PUBKEY | CHECKSUM | VERSION) + ".onion"
	data := make([]byte, len(pubKeyData)+2+1)
	copy(data[:sha256Size], pubKeyData[:])
	data[sha256Size] = checkSum[0]
	data[sha256Size+1] = checkSum[1]
	data[sha256Size+2] = byte(ONION_VERSION)
	s := base32.StdEncoding.WithPadding(-1)           //.EncodeToString(data)
	address = strings.ToLower(s.EncodeToString(data)) // + ".onion"
	return
}

// the same as GenerateAddress, but customize the onion address prefix
func GenerateVanityAddress(prefix string, maxTries uint64) (ed25519.PrivateKey, error) {

	seed, err := readSeed()
	if err != nil {
		return nil, err
	}
	var pub ed25519.PublicKey
	var priv ed25519.PrivateKey
	var address string
	for i := uint64(0); i < maxTries; i++ {
		reader, err := iterateSeed(seed, i)
		if err != nil {
			return nil, err
		}
		pub, priv, err = ed25519.GenerateKey(reader)
		if err != nil {
			return nil, err
		}
		address, err = GetOnionID(pub)
		if err != nil {
			return nil, err
		}
		if prefix == "" {
			return priv, nil
		}
		if strings.HasPrefix(address, prefix) {
			return priv, nil
		}
	}

	return nil, errors.New("failed to generate vanity address")
}

// generate a ed25519 key pair that can be used as both a Solana address and Tor hidden service
func GenerateAddress() (ed25519.PrivateKey, error) {
	return GenerateVanityAddress("", 1)
}

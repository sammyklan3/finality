package mcrypto

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

func EncodeECDSAPrivateKey(privateKey *ecdsa.PrivateKey) ([]byte, error) {
	derBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: derBytes,
	}), nil
}

func DecodeECDSAPrivateKey(privBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(privBytes)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("Invalid PEM block")
	}

	return x509.ParseECPrivateKey(block.Bytes)
}

// Creates an ECDSA public key from provided bytes
// Returns an error is bytes is NOT an ECDSA public key
func DecodeECDSAPublicKey(pubBytes []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(pubBytes)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("Invalid PEM block")
	}

	// Create a public key from public key bytes
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	publicKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("Unsupported public key type. Platform only supports ECDSA public keys")
	}
	return publicKey, nil
}

func EncodeECDSAPublicKey(publicKey *ecdsa.PublicKey) ([]byte, error) {
	derBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}

	pemEncoded := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: derBytes,
	})
	return pemEncoded, nil
}

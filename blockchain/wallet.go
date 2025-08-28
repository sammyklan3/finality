package blockchain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sammyklan3/finality/blockchain/utils"
	"github.com/sammyklan3/finality/blockchain/utils/mcrypto"
)

type Wallet struct {
	Owner           string
	PrivateKeyBytes []byte // We export the key as PEM encoded bytes

	privateKey *ecdsa.PrivateKey
}

func NewWallet(owner string) (*Wallet, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return nil, fmt.Errorf("Wallet must have an owner")
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("Error generating ECDSA key pair; %v", err)
	}

	privateKeyBytes, err := mcrypto.EncodeECDSAPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("Error encoding ECDSA private key; %v", err)
	}

	return &Wallet{
		Owner:           strings.ToUpper(owner),
		PrivateKeyBytes: privateKeyBytes,
		privateKey:      privateKey,
	}, nil
}

func (w *Wallet) PrivateKey() (*ecdsa.PrivateKey, error) {
	if w.privateKey != nil {
		return w.privateKey, nil
	}

	// Parse private key from private key bytes
	privateKey, err := mcrypto.DecodeECDSAPrivateKey(w.PrivateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("Error decoding ECDSA private key; %v", err)
	}

	// Set private key onto the wallet, so we don't have to parse its
	// bytes again
	w.privateKey = privateKey

	return privateKey, nil
}

// Tries reading saved wallet from file.
// Creates a new wallet and writes it to the file if wallet does not exist.
func GetOrCreateWallet(owner string) (*Wallet, error) {
	filename := filepath.Join(utils.SecretsDir, fmt.Sprintf("%v.wallet", owner))

	// Open file for read/write, create if not exists
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Try reading wallet from file
	var wallet Wallet
	err = gob.NewDecoder(file).Decode(&wallet)
	if err == nil {
		return &wallet, nil // successfully decoded
	}

	// If file is empty (new), create a new wallet
	if errors.Is(err, io.EOF) {
		newWallet, err := NewWallet(owner)
		if err != nil {
			return nil, fmt.Errorf("Error creating new wallet; %v", err)
		}

		// Reset file before writing
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		if err := file.Truncate(0); err != nil {
			return nil, err
		}

		enc := gob.NewEncoder(file)
		if err := enc.Encode(newWallet); err != nil {
			return nil, err
		}

		return newWallet, nil
	}

	// Any other decode error means corruption
	return nil, fmt.Errorf("failed to decode wallet: %w", err)
}

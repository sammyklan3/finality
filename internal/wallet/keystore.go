package wallet

import (
    "crypto/ecdsa"
	"finality/internal/core"
)

type Keystore struct {
    Dir string // Directory where encrypted keyfiles are stored
}

func NewKeystore(dir string) *Keystore {
    return &Keystore{Dir: dir}
}

// Generate and store new account
func (ks *Keystore) CreateAccount(passphrase string) (string, error) {
    // 1. Generate ECDSA keypair
    // 2. Derive address
    // 3. Encrypt private key
    // 4. Save as JSON in ks.Dir
    return address, nil
}

// Unlock an account
func (ks *Keystore) Unlock(address, passphrase string) (*ecdsa.PrivateKey, error) {
    // 1. Load JSON file
    // 2. Decrypt using passphrase
    return privateKey, nil
}

// Sign a transaction
func (ks *Keystore) SignTx(address string, tx *core.Transaction, passphrase string) error {
    // Unlock -> Sign -> Zero memory
    return nil
}

// List all accounts
func (ks *Keystore) Accounts() ([]string, error) {
    return []string{}, nil
}

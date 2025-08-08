package core

import (
	"testing"
)

func TestNewWallet(t *testing.T) {
	wallet := NewWallet()
	if wallet.PrivateKey == nil {
		t.Error("Expected PrivateKey to be non-nil")
	}
	if wallet.PublicKey == nil {
		t.Error("Expected PublicKey to be non-nil")
	}
	if wallet.PublicKey != &wallet.PrivateKey.PublicKey {
		t.Error("PublicKey does not match PrivateKey.PublicKey")
	}
}

func TestGetAddress(t *testing.T) {
	wallet := NewWallet()
	address := wallet.GetAddress()
	if len(address) != 40 {
		t.Errorf("Expected address length of 40, got %d", len(address))
	}
}

func TestSignAndVerify(t *testing.T) {
	wallet := NewWallet()
	message := []byte("Hello, blockchain!")
	sig, err := wallet.Sign(message)
	if err != nil {
		t.Errorf("Sign failed: %v", err)
	}
	if len(sig) != 64 {
		t.Errorf("Expected signature length of 64, got %d", len(sig))
	}

	if !wallet.Verify(message, sig) {
		t.Error("Expected signature to be valid")
	}

	if wallet.Verify([]byte("Tampered message"), sig) {
		t.Error("Expected tampered signature to be invalid")
	}
}

func TestSignEmptyData(t *testing.T) {
	wallet := NewWallet()
	sig, err := wallet.Sign([]byte{})
	if err == nil {
		t.Error("Expected error when signing empty data")
	}
	if sig != nil {
		t.Error("Expected nil signature on error")
	}
}

func TestKeyExportImport(t *testing.T) {
	originalWallet := NewWallet()

	privPEM, err := originalWallet.ExportPrivateKey()
	if err != nil || privPEM == nil {
		t.Fatalf("Failed to export private key: %v", err)
	}

	pubPEM, err := originalWallet.ExportPublicKey()
	if err != nil || pubPEM == nil {
		t.Fatalf("Failed to export public key: %v", err)
	}

	importedWallet, err := ImportPrivateKey(privPEM)
	if err != nil || importedWallet == nil {
		t.Fatalf("Failed to import wallet: %v", err)
	}

	message := []byte("Reconstructed wallet test")
	sig, err := importedWallet.Sign(message)
	if err != nil {
		t.Errorf("Sign failed: %v", err)
	}

	if !importedWallet.Verify(message, sig) {
		t.Error("Signature verification failed on imported wallet")
	}
}

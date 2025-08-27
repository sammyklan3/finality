package dtos

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sammyklan3/finality/blockchain/utils/mcrypto"
)

const (
	MIN_NAME_LEN uint = 4
)

type keyType uint

const (
	RSA keyType = iota
	DSA
	ECDSA
	ED25519
	ECDH
)

var (
	ErrUnsupportedKeyType error = errors.New("unsupported key type")
)

// Representation of our organizations table in database
type Organization struct {
	Id          int       `json:"id"`
	AccountName string    `json:"account_name"`
	PublicKey   []byte    `json:"public_key"`
	CreatedAt   time.Time `json:"created_at"`
}

func NewOrganization(name string, publicKey []byte) (*Organization, error) {
	o := Organization{
		AccountName: name,
		PublicKey:   publicKey,
	}
	if err := o.Validate(); err != nil {
		return nil, err
	}
	return &o, nil
}

func (o Organization) Validate() error {
	if err := ValidateName(o.AccountName); err != nil {
		return err
	}
	if err := validatePublicKey(o.PublicKey); err != nil {
		log.Printf("Error validating public key data; %v\n", err)
		return fmt.Errorf("Error validating public key data")
	}
	return nil
}

func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("Name cannot be empty")
	}
	if len(name) < int(MIN_NAME_LEN) {
		return fmt.Errorf("Name cannot be less than %v characters long", MIN_NAME_LEN)
	}
	return nil
}

func validatePublicKey(data []byte) error {
	log.Printf("Validating potential public key data; \n\n%s\n", data)
	_, err := mcrypto.DecodeECDSAPublicKey(data)
	return err
}

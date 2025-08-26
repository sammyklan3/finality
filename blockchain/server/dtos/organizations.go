package dtos

import (
	"crypto/dsa"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
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
	Id        int       `json:"id"`
	OrgName   string    `json:"org_name"`
	PublicKey []byte    `json:"public_key"`
	CreatedAt time.Time `json:"created_at"`
}

func NewOrganization(name string, publicKey []byte) (*Organization, error) {
	o := Organization{
		OrgName:   name,
		PublicKey: publicKey,
	}
	if err := o.Validate(); err != nil {
		return nil, err
	}
	return &o, nil
}

func (o Organization) Validate() error {
	if err := validateOrgName(o.OrgName); err != nil {
		return err
	}
	if err := validatePublicKey(o.PublicKey); err != nil {
		log.Printf("Error validating public key data; %v\n", err)
		return fmt.Errorf("Error validating public key data")
	}
	return nil
}

func validateOrgName(orgName string) error {
	orgName = strings.TrimSpace(orgName)
	if orgName == "" {
		return fmt.Errorf("Organization name cannot be empty")
	}
	if len(orgName) < int(MIN_NAME_LEN) {
		return fmt.Errorf("Organization name cannot be less than %v characters long", MIN_NAME_LEN)
	}
	return nil
}

func validatePublicKey(data []byte) error {
	log.Printf("Validating potential public key data; \n\n%s\n", data)

	block, _ := pem.Decode(data)
	if block == nil {
		return fmt.Errorf("Invalid PEM block")
	}

	if block.Type == "RSA PUBLIC KEY" {
		// We somehow cannot parse RSA public keys with x509.ParsePKIXPublicKey;
		// it fails even for valid public keys
		_, err := x509.ParsePKCS1PublicKey(block.Bytes)
		return err
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}

	_, err = parsePublicKey(publicKey)
	if err != nil {
		return err
	}
	return nil
}

// Checks if a key falls into any of the following types;
//
//	*[rsa.PublicKey], *[dsa.PublicKey], *[ecdsa.PublicKey], [ed25519.PublicKey] (not a pointer), or *[ecdh.PublicKey]
func parsePublicKey(publicKey any) (keyType, error) {
	_, ok := publicKey.(*rsa.PublicKey)
	if ok {
		return RSA, nil
	}

	_, ok = publicKey.(*dsa.PublicKey)
	if ok {
		return DSA, nil
	}

	_, ok = publicKey.(*ecdsa.PublicKey)
	if ok {
		return ECDSA, nil
	}

	_, ok = publicKey.(ed25519.PublicKey)
	if ok {
		return ED25519, nil
	}

	_, ok = publicKey.(*ecdh.PublicKey)
	if ok {
		return ECDH, nil
	}
	return 0, ErrUnsupportedKeyType
}

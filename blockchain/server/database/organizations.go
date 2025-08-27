package database

import (
	"database/sql"
	"log"

	"github.com/sammyklan3/finality/blockchain/server/dtos"
)

type OrganizationsTable struct {
	db *sql.DB
}

func NewOrganizationTable() *OrganizationsTable {
	return &OrganizationsTable{
		db: db,
	}
}

func (model *OrganizationsTable) Create(org dtos.Organization) error {
	query := "INSERT INTO organizations(account_name, public_key) VALUES(?, ?)"
	_, err := model.db.Exec(query, org.AccountName, string(org.PublicKey))
	return err
}

func (model *OrganizationsTable) Exists(orgName string) bool {
	query := "SELECT EXISTS(SELECT 1 FROM organizations WHERE account_name = ?)"
	row := model.db.QueryRow(query, orgName)

	var exists bool
	err := row.Scan(&exists)
	if err != nil {
		log.Printf("Error checking if organization EXISTS; %v\n", err)
		return false
	}
	return exists
}

func (model *OrganizationsTable) Get(orgName string) (*dtos.Organization, error) {
	var org dtos.Organization

	query := "SELECT * FROM organizations WHERE account_name=?"
	row := model.db.QueryRow(query, orgName)
	err := row.Scan(
		&org.Id,
		&org.AccountName,
		&org.PublicKey,
		&org.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &org, nil
}

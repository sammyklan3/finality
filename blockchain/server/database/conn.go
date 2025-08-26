package database

import (
	"database/sql"
	"log"
	"os"

	"github.com/go-sql-driver/mysql"
	"github.com/sammyklan3/finality/blockchain/server/utils"
)

var (
	db *sql.DB
)

func init() {
	err := utils.LoadEnv()
	if err != nil {
		log.Fatalf("Error loading environment variables; %v\n", err)
	}

	config := mysql.Config{
		User:                 os.Getenv("MYSQL_USER"),
		Passwd:               os.Getenv("MYSQL_PASSWORD"),
		DBName:               os.Getenv("MYSQL_DBNAME"),
		AllowNativePasswords: true,
		MultiStatements:      true,
		ParseTime:            true,
	}

	db, err = openDbConn(config)
	if err != nil {
		log.Printf("Error opening database connection; %v\n", err)
		log.Println("Make sure you have created a .env file within project directory with the following variables")
		log.Println("MYSQL_USER=", config.User)
		log.Println("MYSQL_PASSWORD=", config.Passwd)
		log.Println("MYSQL_DBNAME=", config.DBName)
		os.Exit(1)
	}

	// During first launch, our database is not going to have any tables
	// we need to create them immediately
	// TODO: mirate tables
}

func openDbConn(config mysql.Config) (*sql.DB, error) {
	log.Println("Opening database connection")

	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		return nil, err
	}

	// Verify that database is actually open
	err = db.Ping()
	return db, err
}

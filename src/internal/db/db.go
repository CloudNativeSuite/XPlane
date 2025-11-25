package db

import (
	"database/sql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"os"
)

func Open() (*sql.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "file:gtm.db?_foreign_keys=on"
		return sql.Open("sqlite3", dsn)
	}
	return sql.Open("postgres", dsn)
}

func Migrate(db *sql.DB) error {
	schemaBytes, err := os.ReadFile("internal/db/schema.sql")
	if err != nil {
		return err
	}
	_, err = db.Exec(string(schemaBytes))
	return err
}

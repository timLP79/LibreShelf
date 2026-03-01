package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DatabaseManager struct {
	db *sql.DB
}

func NewDatabaseManager(dbPath string) *DatabaseManager {
	// Create the data directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Enable foreign key enforcement
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		log.Fatalf("Failed to enable foreign keys: %v", err)
	}

	dm := &DatabaseManager{db: db}
	dm.createSchema()
	return dm
}

func (dm *DatabaseManager) createSchema() {
	schema := `CREATE TABLE IF NOT EXISTS books (
          id             INTEGER PRIMARY KEY AUTOINCREMENT,
          title          TEXT NOT NULL,
          isbn           TEXT,
          cover_url      TEXT,
          published_year INTEGER,
          available      INTEGER DEFAULT 1
      );

      CREATE TABLE IF NOT EXISTS authors (
          id   INTEGER PRIMARY KEY AUTOINCREMENT,
          name TEXT NOT NULL
      );

      CREATE TABLE IF NOT EXISTS book_authors (
          book_id   INTEGER REFERENCES books(id),
          author_id INTEGER REFERENCES authors(id),
          PRIMARY KEY (book_id, author_id)
      );

      CREATE TABLE IF NOT EXISTS patrons (
          id    INTEGER PRIMARY KEY AUTOINCREMENT,
          name  TEXT NOT NULL,
          email TEXT
      );

      CREATE TABLE IF NOT EXISTS loans (
          id              INTEGER PRIMARY KEY AUTOINCREMENT,
          book_id         INTEGER REFERENCES books(id),
          patron_id       INTEGER REFERENCES patrons(id),
          checked_out_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
          due_date        DATETIME,
          returned_at     DATETIME
      );`

	if _, err := dm.db.Exec(schema); err != nil {
		log.Fatalf("Failed to create schema: %v", err)
	}

	log.Println("Database schema ready")
}

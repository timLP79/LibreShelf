package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"

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
          id     			INTEGER PRIMARY KEY AUTOINCREMENT,
          title          	TEXT NOT NULL,
          isbn           	TEXT,
          cover_url      	TEXT,
          published_year 	INTEGER,
          available      	INTEGER DEFAULT 1
      );

      CREATE TABLE IF NOT EXISTS authors (
          id	INTEGER PRIMARY KEY AUTOINCREMENT,
          name	TEXT NOT NULL
      );

      CREATE TABLE IF NOT EXISTS book_authors (
          book_id	INTEGER REFERENCES books(id),
          author_id INTEGER REFERENCES authors(id),
          PRIMARY KEY (book_id, author_id)
      );

      CREATE TABLE IF NOT EXISTS patrons (
          id    INTEGER PRIMARY KEY AUTOINCREMENT,
          name  TEXT NOT NULL,
          email TEXT
      );

      CREATE TABLE IF NOT EXISTS loans (
          id       			INTEGER PRIMARY KEY AUTOINCREMENT,
          book_id       	INTEGER REFERENCES books(id),
          patron_id       	INTEGER REFERENCES patrons(id),
          checked_out_at	DATETIME DEFAULT CURRENT_TIMESTAMP,
          due_date        	DATETIME,
          returned_at     	DATETIME
      );

	  CREATE TABLE IF NOT EXISTS users (
	    id				INTEGER PRIMARY KEY AUTOINCREMENT,
	    username		TEXT NOT NULL UNIQUE,
	    password_hash	TEXT NOT NULL,
	    role 			TEXT NOT NULL CHECK(role IN('admin', 'patron')),
	    patron_id		INTEGER REFERENCES patrons(id),
	    created_at		DATETIME DEFAULT	CURRENT_TIMESTAMP
	  );
	
	  CREATE TABLE IF NOT EXISTS sessions (
	    token TEXT 	PRIMARY KEY,
	    user_id 	INTEGER NOT NULL REFERENCES users(id),
	    expires_at 	DATETIME NOT NULL      
	  );`

	if _, err := dm.db.Exec(schema); err != nil {
		log.Fatalf("Failed to create schema: %v", err)
	}

	log.Println("Database schema ready")
}

type User struct {
	ID           int
	Username     string
	PasswordHash string
	Role         string
	PatronID     *int
}

func (dm *DatabaseManager) GetUserByUsername(username string) (*User, error) {
	user := &User{}
	err := dm.db.QueryRow(
		"SELECT id, username, password_hash, role, patron_id FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.PatronID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (dm *DatabaseManager) CreateUser(username, passwordHash, role string, patronID *int) error {
	_, err := dm.db.Exec(
		"INSERT INTO users (username, password_hash, role, patron_id) VALUES (?, ?, ?, ?)",
		username, passwordHash, role, patronID,
	)
	return err
}

func (dm *DatabaseManager) CreateSession(token string, userID int, expiresAt time.Time) error {
	_, err := dm.db.Exec(
		"INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		token, userID, expiresAt,
	)
	return err
}

func (dm *DatabaseManager) GetSession(token string) (*User, error) {
	user := &User{}
	err := dm.db.QueryRow(`
		SELECT u.id, u.username, u.password_hash, u.role, u.patron_id
		FROM sessions s
		JOIN users u on s.user_id = u.id
		WHERE s.token = ? AND s.expires_at > CURRENT_TIMESTAMP`,
		token,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.PatronID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (dm *DatabaseManager) DeleteSession(token string) error {
	_, err := dm.db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

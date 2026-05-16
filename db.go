// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	_ "modernc.org/sqlite"
)

const (
	DefaultLoanTermDays     = 14
	MaxActiveLoansPerPatron = 5
)

type Book struct {
	ID                int
	Title             string
	ISBN              *string
	CoverFilename     *string
	Year              *int
	Publisher         *string
	Description       *string
	Genre             *string
	QuantityTotal     int
	QuantityAvailable int
	Authors           string
}

type Author struct {
	ID   int
	Name string
}

type LoanRecord struct {
	ID           int
	PatronName   string
	CheckedOutAt string
	DueDate      string
	ReturnedAt   *string
	Status       string
}

type LoanListRow struct {
	LoanID      int
	BookID      int
	BookTitle   string
	PatronID    int
	PatronName  string
	DueDate     string
	DaysOverdue int
}

type StaffMember struct {
	ID        int
	Username  string
	Role      string
	CreatedAt string
}

type Patron struct {
	ID              int
	Name            string
	Email           *string
	Phone           *string
	JoinedDate      string
	Metadata        *string
	Username        string
	HasTempPassword bool
}

func (dm *DatabaseManager) GetAllStaff() ([]StaffMember, error) {
	rows, err := dm.db.Query(`
	SELECT id, username, role, created_at
	FROM users
	WHERE role != 'patron'
	ORDER BY CASE role WHEN 'admin' THEN 0 ELSE 1 END, username`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var staff []StaffMember
	for rows.Next() {
		var s StaffMember
		if err := rows.Scan(&s.ID, &s.Username, &s.Role, &s.CreatedAt); err != nil {
			return nil, err
		}
		staff = append(staff, s)
	}
	return staff, rows.Err()
}

func (dm *DatabaseManager) GetAllBooks() ([]Book, error) {
	rows, err := dm.db.Query(`
			SELECT b.id, b.title, b.isbn, b.cover_filename, b.year, b.publisher,
                   b.description, b.genre, b.quantity_total, b.quantity_available,                                                                                                        
                   GROUP_CONCAT(a.name, ', ') AS authors
                FROM books b                                                                                                                                                                  
                LEFT JOIN book_authors ba ON b.id = ba.book_id                                                                                                                                
                LEFT JOIN authors a ON ba.author_id = a.id                                                                                                                                    
                GROUP BY b.id                                                                                                                                                                 
                ORDER BY b.title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []Book
	for rows.Next() {
		var book Book
		var authors *string
		if err := rows.Scan(&book.ID, &book.Title, &book.ISBN, &book.CoverFilename,
			&book.Year, &book.Publisher, &book.Description, &book.Genre,
			&book.QuantityTotal, &book.QuantityAvailable, &authors); err != nil {
			return nil, err
		}
		if authors != nil {
			book.Authors = *authors
		}
		books = append(books, book)
	}
	return books, rows.Err()
}

// GetOutOfStockBooks returns books whose quantity_available is 0
// (every copy is checked out). Used by HandleCatalog when invoked
// with ?filter=out so the dashboard's Out-of-Stock card can deep-link
// into a filtered catalog view. The shape matches GetAllBooks so the
// same template renders the result.
func (dm *DatabaseManager) GetOutOfStockBooks() ([]Book, error) {
	rows, err := dm.db.Query(`
		SELECT b.id, b.title, b.isbn, b.cover_filename, b.year, b.publisher,
		       b.description, b.genre, b.quantity_total, b.quantity_available,
		       GROUP_CONCAT(a.name, ', ') AS authors
		FROM books b
		LEFT JOIN book_authors ba ON b.id = ba.book_id
		LEFT JOIN authors a ON ba.author_id = a.id
		WHERE b.quantity_available = 0
		GROUP BY b.id
		ORDER BY b.title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []Book
	for rows.Next() {
		var book Book
		var authors *string
		if err := rows.Scan(&book.ID, &book.Title, &book.ISBN, &book.CoverFilename,
			&book.Year, &book.Publisher, &book.Description, &book.Genre,
			&book.QuantityTotal, &book.QuantityAvailable, &authors); err != nil {
			return nil, err
		}
		if authors != nil {
			book.Authors = *authors
		}
		books = append(books, book)
	}
	return books, rows.Err()
}

func (dm *DatabaseManager) GetBookByID(id int) (*Book, error) {
	book := &Book{}
	err := dm.db.QueryRow(`         
                SELECT id, title, isbn, cover_filename, year, publisher,
                       description, genre, quantity_total, quantity_available
                FROM books WHERE id = ?`, id).Scan(
		&book.ID, &book.Title, &book.ISBN, &book.CoverFilename,
		&book.Year, &book.Publisher, &book.Description, &book.Genre,
		&book.QuantityTotal, &book.QuantityAvailable)
	if err != nil {
		return nil, err
	}

	rows, err := dm.db.Query(`
                SELECT a.name FROM authors a
                JOIN book_authors ba ON a.id = ba.author_id
                WHERE ba.book_id = ?
                ORDER BY ba.position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	book.Authors = strings.Join(names, ", ")

	return book, nil

}

func (dm *DatabaseManager) GetBookByISBN(isbn string) (*Book, error) {
	book := &Book{}
	err := dm.db.QueryRow(
		"SELECT id, title FROM books WHERE isbn = ?", isbn,
	).Scan(&book.ID, &book.Title)
	if err != nil {
		return nil, err
	}
	return book, nil
}

func (dm *DatabaseManager) CheckoutBook(bookID, patronID int, dueDate time.Time) error {
	tx, err := dm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var overdueCount int
	if err := tx.QueryRow(`
			SELECT COUNT(*) FROM loans
			WHERE patron_id = ?
				AND returned_at IS NULL
				AND due_date < DATE('now')`,
		patronID).Scan(&overdueCount); err != nil {
		return err
	}
	if overdueCount > 0 {
		return ErrPatronHasOverdue
	}

	var activeCount int
	if err := tx.QueryRow(`
			SELECT COUNT(*) FROM loans
			WHERE patron_id = ?
				AND returned_at IS NULL`,
		patronID).Scan(&activeCount); err != nil {
		return err
	}
	if activeCount >= MaxActiveLoansPerPatron {
		return ErrPatronAtLoanLimit
	}

	var available int
	if err := tx.QueryRow(
		`SELECT quantity_available FROM books WHERE id = ?`,
		bookID).Scan(&available); err != nil {
		return err
	}
	if available <= 0 {
		return ErrNoCopiesAvailable
	}

	if _, err := tx.Exec(
		`INSERT INTO loans (book_id, patron_id, due_date) VALUES (?, ?, ?)`,
		bookID, patronID, dueDate.UTC().Format("2006-01-02")); err != nil {
		return err
	}

	if _, err := tx.Exec(
		`UPDATE books SET quantity_available = quantity_available - 1 WHERE id = ?`,
		bookID); err != nil {
		return err
	}

	return tx.Commit()
}

func (dm *DatabaseManager) ReturnBook(loanID int) error {
	tx, err := dm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var bookID int
	var returnedAt sql.NullString
	if err := tx.QueryRow(
		`SELECT book_id, returned_at FROM loans WHERE id = ?`,
		loanID).Scan(&bookID, &returnedAt); err != nil {
		return err
	}
	if returnedAt.Valid {
		return ErrLoanAlreadyReturned
	}

	if _, err := tx.Exec(
		`UPDATE loans SET returned_at = CURRENT_TIMESTAMP WHERE id = ?`,
		loanID); err != nil {
		return err
	}

	if _, err := tx.Exec(
		`UPDATE books SET quantity_available = quantity_available + 1 WHERE id = ?`,
		bookID); err != nil {
		return err
	}

	return tx.Commit()
}

func (dm *DatabaseManager) GetLoanHistory(bookID int) ([]LoanRecord, error) {
	rows, err := dm.db.Query(`                         
                SELECT l.id, p.name, l.checked_out_at, l.due_date, l.returned_at                                                                                                              
                FROM loans l             
                JOIN patrons p ON l.patron_id = p.id
                WHERE l.book_id = ?                  
                ORDER BY l.checked_out_at DESC`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []LoanRecord
	for rows.Next() {
		var r LoanRecord
		if err := rows.Scan(&r.ID, &r.PatronName, &r.CheckedOutAt, &r.DueDate, &r.ReturnedAt); err != nil {
			return nil, err
		}
		if r.ReturnedAt != nil {
			r.Status = "returned"
		} else if r.DueDate < time.Now().Format("2006-01-02 15:04:05") {
			r.Status = "overdue"
		} else {
			r.Status = "active"
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (dm *DatabaseManager) GetActiveLoans() ([]LoanListRow, error) {
	rows, err := dm.db.Query(`
			SELECT l.id, b.id, b.title, p.id, p.name, l.due_date,
				CAST(julianday('now') - julianday(l.due_date) AS INTEGER) AS days_overdue
			FROM loans l
			JOIN books b ON l.book_id = b.id
			JOIN patrons p ON l.patron_id = p.id
			WHERE l.returned_at IS NULL
				AND l.due_date >= DATE('now')
			ORDER BY l.due_date ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var loans []LoanListRow
	for rows.Next() {
		var r LoanListRow
		if err := rows.Scan(&r.LoanID, &r.BookID, &r.BookTitle,
			&r.PatronID, &r.PatronName, &r.DueDate, &r.DaysOverdue); err != nil {
			return nil, err
		}
		loans = append(loans, r)
	}
	return loans, rows.Err()
}

func (dm *DatabaseManager) GetOverdueLoans() ([]LoanListRow, error) {
	rows, err := dm.db.Query(`
			SELECT l.id, b.id, b.title, p.id, p.name, l.due_date,
				CAST(julianday('now') - julianday(l.due_date) AS INTEGER) AS days_overdue
			FROM loans l
			JOIN books b ON l.book_id = b.id
			JOIN patrons p ON l.patron_id = p.id
			WHERE l.returned_at IS NULL
				AND l.due_date < DATE('now')
			ORDER BY l.due_date ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var loans []LoanListRow
	for rows.Next() {
		var r LoanListRow
		if err := rows.Scan(&r.LoanID, &r.BookID, &r.BookTitle,
			&r.PatronID, &r.PatronName, &r.DueDate, &r.DaysOverdue); err != nil {
			return nil, err
		}
		loans = append(loans, r)
	}
	return loans, rows.Err()
}

func (dm *DatabaseManager) GetPatronActiveLoans(patronID int) ([]LoanListRow, error) {
	rows, err := dm.db.Query(`
			SELECT l.id, b.id, b.title, p.id, p.name, l.due_date,
				CAST(julianday('now') - julianday(l.due_date) AS INTEGER) AS days_overdue
			FROM loans l
			JOIN books b ON l.book_id = b.id
			JOIN patrons p ON l.patron_id = p.id
			WHERE l.returned_at IS NULL
				AND l.patron_id = ?
			ORDER BY l.due_date ASC`, patronID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var loans []LoanListRow
	for rows.Next() {
		var r LoanListRow
		if err := rows.Scan(&r.LoanID, &r.BookID, &r.BookTitle,
			&r.PatronID, &r.PatronName, &r.DueDate, &r.DaysOverdue); err != nil {
			return nil, err
		}
		loans = append(loans, r)
	}
	return loans, rows.Err()
}

func (dm *DatabaseManager) CountActiveLoans() (int, error) {
	var count int
	err := dm.db.QueryRow(`
			SELECT COUNT(*) FROM loans
			WHERE returned_at IS NULL
				AND due_date >= DATE('now')`).Scan(&count)
	return count, err
}

func (dm *DatabaseManager) CountOverdueLoans() (int, error) {
	var count int
	err := dm.db.QueryRow(`
			SELECT COUNT(*) FROM loans
			WHERE returned_at IS NULL
				AND due_date < DATE('now')`).Scan(&count)
	return count, err
}

func (dm *DatabaseManager) CountOutOfStock() (int, error) {
	var count int
	err := dm.db.QueryRow(`
			SELECT COUNT(*) FROM books
			WHERE quantity_available = 0`).Scan(&count)
	return count, err
}

func (dm *DatabaseManager) CountBooks() (int, error) {
	var count int
	err := dm.db.QueryRow(`SELECT COUNT(*) FROM books`).Scan(&count)
	return count, err
}

func (dm *DatabaseManager) CountPatrons() (int, error) {
	var count int
	err := dm.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'patron'`).Scan(&count)
	return count, err
}

func (dm *DatabaseManager) CountTotalLoans() (int, error) {
	var count int
	err := dm.db.QueryRow(`SELECT COUNT(*) FROM loans`).Scan(&count)
	return count, err
}

// SnapshotTo writes a consistent point-in-time copy of the database to
// destPath using SQLite's VACUUM INTO. destPath must NOT already exist.
// VACUUM INTO does not accept parameter bindings for the destination,
// so the path is escaped and inlined. Callers should construct destPath
// from a process-controlled source (e.g. os.MkdirTemp).
func (dm *DatabaseManager) SnapshotTo(destPath string) error {
	escaped := strings.ReplaceAll(destPath, "'", "''")
	_, err := dm.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", escaped))
	return err
}

type DatabaseManager struct {
	mu     sync.RWMutex
	db     *sql.DB
	dbPath string
}

func NewDatabaseManager(dbPath string) *DatabaseManager {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	db, err := openDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	dm := &DatabaseManager{db: db, dbPath: dbPath}
	dm.createSchema()
	return dm
}

// openDB opens a SQLite database with foreign keys + WAL mode, returning
// any error rather than calling log.Fatal. Used by HandleBackupImport
// when reopening after a swap, where a non-fatal failure must be
// recoverable rather than killing the server.
func openDB(dbPath string) (*sql.DB, error) {
	// PRAGMAs and txlock are passed in the DSN so the modernc.org/sqlite
	// driver applies them to *every* connection it opens, not just the
	// first one a `db.Exec("PRAGMA ...")` call happens to grab from the
	// pool.
	//
	//   foreign_keys   per-connection. Default off; must be set on each
	//                  new connection.
	//   journal_mode   database-level (persisted in the file once set),
	//                  but harmless to assert per-connection.
	//   busy_timeout   per-connection. Default 0 -- a losing concurrent
	//                  writer returns SQLITE_BUSY immediately. With 5s,
	//                  the loser queues on the journal/WAL lock and then
	//                  re-evaluates its guards inside its own transaction.
	//   _txlock        every non-readonly Begin() issues "BEGIN IMMEDIATE"
	//                  rather than "BEGIN DEFERRED". DEFERRED starts as a
	//                  reader, reads a snapshot, then tries to upgrade to
	//                  writer on the first write -- if another tx
	//                  committed in between, the upgrade fails with
	//                  SQLITE_BUSY_SNAPSHOT (code 517). IMMEDIATE takes
	//                  the write lock at BEGIN time, so other writers
	//                  queue on the lock (with busy_timeout) instead of
	//                  racing for the snapshot. Every dm.db.Begin() call
	//                  in this file is a write transaction.
	//
	// Verified load-bearing by TestCheckoutBookConcurrentRace in
	// db_loans_test.go (cs408-go-stack-7an, DEC-031). CI initially
	// surfaced SQLITE_BUSY (5) on db.Exec-style PRAGMAs (only one
	// connection got them); then SQLITE_BUSY_SNAPSHOT (517) on
	// BEGIN DEFERRED. _txlock=immediate is what closes both classes.
	dsn := dbPath + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)&_pragma=journal_mode(WAL)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	// Touch the DB so the driver opens a connection and surfaces any
	// PRAGMA failure (bad path, permissions) here rather than at first
	// query time.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return db, nil
}

type sessionRow struct {
	Token     string
	UserID    int
	CSRFToken string
	ExpiresAt string
}

// DumpSessions returns every row of the sessions table. Used by the
// import handler to preserve live sessions across a database swap.
func (dm *DatabaseManager) DumpSessions() ([]sessionRow, error) {
	rows, err := dm.db.Query(`SELECT token, user_id, csrf_token, expires_at FROM sessions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sessionRow
	for rows.Next() {
		var s sessionRow
		if err := rows.Scan(&s.Token, &s.UserID, &s.CSRFToken, &s.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// RestoreSessions truncates the sessions table and re-inserts the given
// rows. Sessions whose user_id no longer exists in the (possibly
// imported) users table are skipped to avoid foreign key violations.
func (dm *DatabaseManager) RestoreSessions(sessions []sessionRow) error {
	if _, err := dm.db.Exec(`DELETE FROM sessions`); err != nil {
		return err
	}
	for _, s := range sessions {
		var exists int
		err := dm.db.QueryRow(`SELECT 1 FROM users WHERE id = ?`, s.UserID).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if _, err := dm.db.Exec(
			`INSERT INTO sessions (token, user_id, csrf_token, expires_at) VALUES (?, ?, ?, ?)`,
			s.Token, s.UserID, s.CSRFToken, s.ExpiresAt,
		); err != nil {
			return err
		}
	}
	return nil
}

func (dm *DatabaseManager) createSchema() {
	schema := `CREATE TABLE IF NOT EXISTS books (
		id                 INTEGER PRIMARY KEY AUTOINCREMENT,
		title              TEXT NOT NULL,
		isbn               TEXT UNIQUE,
		cover_filename     TEXT,
		year               INTEGER,
		publisher          TEXT,
		description        TEXT,
		genre              TEXT,
		quantity_total     INTEGER DEFAULT 1,
		quantity_available INTEGER DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS authors (
		id   INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE COLLATE NOCASE
	);

	CREATE TABLE IF NOT EXISTS book_authors (
		book_id   INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
		author_id INTEGER NOT NULL REFERENCES authors(id) ON DELETE CASCADE,
		position  INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (book_id, author_id)
	);

	CREATE TABLE IF NOT EXISTS patrons (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL,
		email       TEXT,
		phone       TEXT,
		joined_date DATETIME DEFAULT CURRENT_TIMESTAMP,
		metadata    TEXT
	);

	CREATE TABLE IF NOT EXISTS loans (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		book_id        INTEGER NOT NULL REFERENCES books(id),
		patron_id      INTEGER NOT NULL REFERENCES patrons(id),
		checked_out_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		due_date       TEXT NOT NULL,
		returned_at    DATETIME
	);

	CREATE TABLE IF NOT EXISTS users (
		id                   INTEGER PRIMARY KEY AUTOINCREMENT,
		username             TEXT NOT NULL UNIQUE,
		password_hash        TEXT NOT NULL,
		role                 TEXT NOT NULL CHECK(role IN('admin', 'staff', 'patron')),
		patron_id            INTEGER REFERENCES patrons(id),
		created_at           DATETIME DEFAULT CURRENT_TIMESTAMP,
		must_change_password INTEGER NOT NULL DEFAULT 0,
		temp_password        TEXT
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token      TEXT PRIMARY KEY,
		user_id    INTEGER NOT NULL REFERENCES users(id),
		csrf_token TEXT NOT NULL,
		expires_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS settings (
		key        TEXT PRIMARY KEY,
		value      TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_by INTEGER REFERENCES users(id)
	);`

	if _, err := dm.db.Exec(schema); err != nil {
		log.Fatalf("Failed to create schema: %v", err)
	}

	log.Println("Database schema ready")
}

type Session struct {
	User      *User
	CSRFToken string
}

type User struct {
	ID                 int
	Username           string
	PasswordHash       string
	Role               string
	PatronID           *int
	MustChangePassword bool
	TempPassword       *string
}

func (dm *DatabaseManager) GetUserByUsername(username string) (*User, error) {
	user := &User{}
	err := dm.db.QueryRow(
		"SELECT id, username, password_hash, role, patron_id, must_change_password, temp_password FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.PatronID, &user.MustChangePassword, &user.TempPassword)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (dm *DatabaseManager) GetUserByID(id int) (*User, error) {
	user := &User{}
	err := dm.db.QueryRow(
		"SELECT id, username, password_hash, role, patron_id, must_change_password, temp_password FROM users where id = ?", id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.PatronID, &user.MustChangePassword, &user.TempPassword)
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

func (dm *DatabaseManager) UpdateStaffUser(id int, username, role string) error {
	_, err := dm.db.Exec(
		"UPDATE users SET username = ?, role = ? WHERE id = ?",
		username, role, id,
	)
	return err
}

func (dm *DatabaseManager) UpdateUserPassword(id int, passwordHash string) error {
	tx, err := dm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		"UPDATE users SET password_hash = ?, must_change_password = 0, temp_password = NULL WHERE id = ?",
		passwordHash, id,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		"DELETE FROM sessions WHERE user_id = ?", id,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (dm *DatabaseManager) SetMustChangePassword(userID int) error {
	_, err := dm.db.Exec(
		"UPDATE users SET must_change_password = 1 WHERE id = ?", userID,
	)
	return err
}

func (dm *DatabaseManager) ClearTempPassword(userID int) error {
	_, err := dm.db.Exec(
		"UPDATE users SET temp_password = NULL WHERE id = ?", userID,
	)
	return err
}

// RegenerateTempPassword generates a new temp, hashes it, swaps both the
// hash and stored plaintext, sets must_change_password=1, and wipes the
// user's existing sessions so any in-flight session under the old hash
// is invalidated.
func (dm *DatabaseManager) RegenerateTempPassword(userID int) (string, error) {
	tempPassword, err := generateTempPassword()
	if err != nil {
		return "", fmt.Errorf("db: generate temp password: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(tempPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("db: hash temp password: %w", err)
	}

	tx, err := dm.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE users SET password_hash = ?, temp_password = ?, must_change_password = 1 WHERE id = ?`,
		string(hash), tempPassword, userID,
	); err != nil {
		return "", err
	}
	if _, err := tx.Exec("DELETE FROM sessions WHERE user_id = ?", userID); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return tempPassword, nil
}

func (dm *DatabaseManager) DeleteUser(id int) error {
	tx, err := dm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM sessions WHERE user_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM users WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}

func (dm *DatabaseManager) CountAdmins() (int, error) {
	var count int
	err := dm.db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	return count, err
}

func (dm *DatabaseManager) CreateSession(token string, userID int, csrfToken string, expiresAt time.Time) error {
	_, err := dm.db.Exec(
		"INSERT INTO sessions (token, user_id, csrf_token, expires_at) VALUES (?, ?, ?, ?)",
		token, userID, csrfToken, expiresAt.UTC().Format("2006-01-02 15:04:05"),
	)
	return err
}

func (dm *DatabaseManager) GetSession(token string) (*Session, error) {
	session := &Session{User: &User{}}
	err := dm.db.QueryRow(`
		SELECT u.id, u.username, u.password_hash, u.role, u.patron_id, u.must_change_password, u.temp_password, s.csrf_token
		FROM sessions s
		JOIN users u on s.user_id = u.id
		WHERE s.token = ? AND datetime(s.expires_at) > datetime('now')`,
		token,
	).Scan(&session.User.ID, &session.User.Username, &session.User.PasswordHash, &session.User.Role, &session.User.PatronID, &session.User.MustChangePassword, &session.User.TempPassword, &session.CSRFToken)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (dm *DatabaseManager) DeleteSession(token string) error {
	_, err := dm.db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

func (dm *DatabaseManager) GetSetting(key string) (string, error) {
	var value string
	err := dm.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

func (dm *DatabaseManager) SetSetting(key, value string, byUserID int) error {
	_, err := dm.db.Exec(
		`INSERT INTO settings (key, value, updated_at, updated_by)
		 VALUES (?, ?, CURRENT_TIMESTAMP, ?)
		 ON CONFLICT(key) DO UPDATE SET
		     value = excluded.value,
		     updated_at = excluded.updated_at,
		     updated_by = excluded.updated_by`,
		key, value, byUserID,
	)
	return err
}

func (dm *DatabaseManager) GetSettingBool(key string, defaultValue bool) bool {
	v, err := dm.GetSetting(key)
	if err != nil || v == "" {
		return defaultValue
	}
	return strings.EqualFold(v, "true")
}

func (dm *DatabaseManager) SeedDefaultUsers() {
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "Admin123!"
	}
	if err := ValidatePassword(adminPassword); err != nil {
		log.Fatalf("ADMIN_PASSWORD does not meet requirements: %v", err)
	}

	var count int

	if err := dm.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'admin'").Scan(&count); err != nil {
		log.Fatalf("Failed to check for admin user: %v", err)
	}
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("Failed to hash admin password: %v", err)
		}
		if err := dm.CreateUser("admin", string(hash), "admin", nil); err != nil {
			log.Fatalf("Failed to seed admin user: %v", err)
		}
		log.Println("Seeded admin user")
	}

	if err := dm.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'patron1'").Scan(&count); err != nil {
		log.Fatalf("Failed to check for patron1 user: %v", err)
	}
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte("Patron123!"), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("Failed to hash patron 1 password: %v", err)
		}
		if err := dm.seedLinkedPatron("patron1", string(hash), "Seed Patron"); err != nil {
			log.Fatalf("Failed to seed patron1: %v", err)
		}
		log.Println("Seeded patron1 user and linked patrons row")
	}

	if err := dm.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'staff1'").Scan(&count); err != nil {
		log.Fatalf("Failed to check for staff1 user: %v", err)
	}
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte("Staff123!"), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("Failed to hash staff1 password: %v", err)
		}
		if err := dm.CreateUser("staff1", string(hash), "staff", nil); err != nil {
			log.Fatalf("Failed to seed staff1 user: %v", err)
		}
		log.Println("Seeded staff1 user")
	}
}

// seedLinkedPatron inserts a patrons row and a linked users row in a
// single transaction, mirroring CreatePatron's two-row write (DEC-022)
// but with an explicit username instead of auto-generation. Used by
// SeedDefaultUsers for patron1 so the seed account appears in the
// admin /patrons list and gives CP6 checkout / return something to
// target. Separate from CreatePatron because that function derives
// the username from a name via generateBaseUsername, and the seed
// needs to keep the canonical "patron1" credential for pre-existing
// auth tests.
func (dm *DatabaseManager) seedLinkedPatron(username, passwordHash, patronName string) error {
	tx, err := dm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec("INSERT INTO patrons (name) VALUES (?)", patronName)
	if err != nil {
		return err
	}
	patronID64, err := res.LastInsertId()
	if err != nil {
		return err
	}
	patronID := int(patronID64)

	if _, err := tx.Exec(
		"INSERT INTO users (username, password_hash, role, patron_id) VALUES (?, ?, 'patron', ?)",
		username, passwordHash, patronID,
	); err != nil {
		return err
	}

	return tx.Commit()
}

type seedBook struct {
	title       string
	isbn        string
	year        int
	publisher   string
	description string
	genre       string
	quantity    int
	authors     []string
}

func (dm *DatabaseManager) SeedBooks() {
	var count int
	if err := dm.db.QueryRow("SELECT COUNT(*) FROM books").Scan(&count); err != nil {
		log.Fatalf("Failed to check book count: %v", err)
	}
	if count > 0 {
		return
	}

	books := []seedBook{
		{
			title:       "Pride and Prejudice",
			isbn:        "9780141439518",
			year:        1813,
			publisher:   "Penguin Classics",
			description: "A romantic novel of manners that chronicles the emotional development of Elizabeth Bennet.",
			genre:       "Classic Literature",
			quantity:    3,
			authors:     []string{"Jane Austen"},
		},
		{
			title:       "To Kill a Mockingbird",
			isbn:        "9780061120084",
			year:        1960,
			publisher:   "Harper Perennial",
			description: "A novel about racial injustice in the American South, seen through the eyes of a young girl.",
			genre:       "Classic Literature",
			quantity:    4,
			authors:     []string{"Harper Lee"},
		},
		{
			title:       "1984",
			isbn:        "9780451524935",
			year:        1949,
			publisher:   "Signet Classics",
			description: "A dystopian novel set in a totalitarian society ruled by Big Brother.",
			genre:       "Science Fiction",
			quantity:    2,
			authors:     []string{"George Orwell"},
		},
		{
			title:       "The Great Gatsby",
			isbn:        "9780743273565",
			year:        1925,
			publisher:   "Scribner",
			description: "A story of wealth, class, and the American Dream in the Jazz Age.",
			genre:       "Classic Literature",
			quantity:    2,
			authors:     []string{"F. Scott Fitzgerald"},
		},
		{
			title:       "Good Omens",
			isbn:        "9780060853983",
			year:        1990,
			publisher:   "William Morrow",
			description: "An angel and a demon team up to prevent the apocalypse.",
			genre:       "Fantasy",
			quantity:    3,
			authors:     []string{"Neil Gaiman", "Terry Pratchett"},
		},
		{
			title:       "Dune",
			isbn:        "9780441013593",
			year:        1965,
			publisher:   "Ace Books",
			description: "An epic science fiction novel set on the desert planet Arrakis.",
			genre:       "Science Fiction",
			quantity:    2,
			authors:     []string{"Frank Herbert"},
		},
		{
			title:       "The Catcher in the Rye",
			isbn:        "9780316769488",
			year:        1951,
			publisher:   "Little, Brown and Company",
			description: "A disillusioned teenager wanders New York after being expelled from prep school.",
			genre:       "Classic Literature",
			quantity:    3,
			authors:     []string{"J.D. Salinger"},
		},
		{
			title:       "Brave New World",
			isbn:        "9780060850524",
			year:        1932,
			publisher:   "Harper Perennial",
			description: "A dystopian society engineered for stability through conditioning and pleasure.",
			genre:       "Science Fiction",
			quantity:    3,
			authors:     []string{"Aldous Huxley"},
		},
		{
			title:       "Jane Eyre",
			isbn:        "9780141441146",
			year:        1847,
			publisher:   "Penguin Classics",
			description: "An orphaned governess falls for her brooding employer and uncovers his secret.",
			genre:       "Classic Literature",
			quantity:    2,
			authors:     []string{"Charlotte Bronte"},
		},
		{
			title:       "Wuthering Heights",
			isbn:        "9780141439556",
			year:        1847,
			publisher:   "Penguin Classics",
			description: "A passionate, destructive love story set on the windswept Yorkshire moors.",
			genre:       "Classic Literature",
			quantity:    2,
			authors:     []string{"Emily Bronte"},
		},
		{
			title:       "Frankenstein",
			isbn:        "9780141439471",
			year:        1818,
			publisher:   "Penguin Classics",
			description: "A scientist assembles a creature from corpses and pays the price for playing god.",
			genre:       "Gothic Fiction",
			quantity:    3,
			authors:     []string{"Mary Shelley"},
		},
		{
			title:       "Dracula",
			isbn:        "9780141439846",
			year:        1897,
			publisher:   "Penguin Classics",
			description: "A Transylvanian count spreads his undead curse through Victorian England.",
			genre:       "Gothic Fiction",
			quantity:    2,
			authors:     []string{"Bram Stoker"},
		},
		{
			title:       "Moby-Dick",
			isbn:        "9780142437247",
			year:        1851,
			publisher:   "Penguin Classics",
			description: "Captain Ahab's obsessive hunt for the white whale that took his leg.",
			genre:       "Classic Literature",
			quantity:    2,
			authors:     []string{"Herman Melville"},
		},
		{
			title:       "Crime and Punishment",
			isbn:        "9780143058144",
			year:        1866,
			publisher:   "Penguin Classics",
			description: "A destitute student murders a pawnbroker and unravels under his own guilt.",
			genre:       "Classic Literature",
			quantity:    2,
			authors:     []string{"Fyodor Dostoevsky"},
		},
		{
			title:       "The Count of Monte Cristo",
			isbn:        "9780140449266",
			year:        1844,
			publisher:   "Penguin Classics",
			description: "A wrongfully imprisoned sailor escapes and engineers an elaborate revenge.",
			genre:       "Classic Literature",
			quantity:    2,
			authors:     []string{"Alexandre Dumas"},
		},
		{
			title:       "Anna Karenina",
			isbn:        "9780143035008",
			year:        1877,
			publisher:   "Penguin Classics",
			description: "A married aristocrat risks everything for a passionate affair in Imperial Russia.",
			genre:       "Classic Literature",
			quantity:    2,
			authors:     []string{"Leo Tolstoy"},
		},
		{
			title:       "The Hobbit",
			isbn:        "9780547928227",
			year:        1937,
			publisher:   "Mariner Books",
			description: "A reluctant hobbit is swept into a quest to reclaim a dwarven kingdom from a dragon.",
			genre:       "Fantasy",
			quantity:    5,
			authors:     []string{"J.R.R. Tolkien"},
		},
		{
			title:       "The Fellowship of the Ring",
			isbn:        "9780547928210",
			year:        1954,
			publisher:   "Mariner Books",
			description: "A young hobbit sets out with an unlikely fellowship to destroy an evil ring.",
			genre:       "Fantasy",
			quantity:    4,
			authors:     []string{"J.R.R. Tolkien"},
		},
		{
			title:       "The Two Towers",
			isbn:        "9780547928203",
			year:        1954,
			publisher:   "Mariner Books",
			description: "The broken fellowship faces the gathering armies of Mordor on two fronts.",
			genre:       "Fantasy",
			quantity:    3,
			authors:     []string{"J.R.R. Tolkien"},
		},
		{
			title:       "Harry Potter and the Sorcerer's Stone",
			isbn:        "9780590353427",
			year:        1997,
			publisher:   "Scholastic",
			description: "An orphaned boy discovers he is a wizard and begins his first year at Hogwarts.",
			genre:       "Fantasy",
			quantity:    5,
			authors:     []string{"J.K. Rowling"},
		},
		{
			title:       "A Game of Thrones",
			isbn:        "9780553593716",
			year:        1996,
			publisher:   "Bantam",
			description: "Noble houses scheme, betray, and bleed for the Iron Throne of Westeros.",
			genre:       "Fantasy",
			quantity:    4,
			authors:     []string{"George R.R. Martin"},
		},
		{
			title:       "The Name of the Wind",
			isbn:        "9780756404741",
			year:        2007,
			publisher:   "DAW Books",
			description: "An innkeeper recounts his rise from orphaned street urchin to legendary arcanist.",
			genre:       "Fantasy",
			quantity:    3,
			authors:     []string{"Patrick Rothfuss"},
		},
		{
			title:       "Mistborn: The Final Empire",
			isbn:        "9780765350381",
			year:        2006,
			publisher:   "Tor Books",
			description: "A crew of thieves plots to topple an immortal god-emperor using metal-based magic.",
			genre:       "Fantasy",
			quantity:    3,
			authors:     []string{"Brandon Sanderson"},
		},
		{
			title:       "The Way of Kings",
			isbn:        "9780765365279",
			year:        2010,
			publisher:   "Tor Books",
			description: "Storm-ridden warriors, scholars, and slaves converge on the eve of a returning apocalypse.",
			genre:       "Fantasy",
			quantity:    2,
			authors:     []string{"Brandon Sanderson"},
		},
		{
			title:       "American Gods",
			isbn:        "9780062572233",
			year:        2001,
			publisher:   "William Morrow",
			description: "Ancient gods brought to America by immigrants clash with the new gods of media and tech.",
			genre:       "Fantasy",
			quantity:    3,
			authors:     []string{"Neil Gaiman"},
		},
		{
			title:       "Foundation",
			isbn:        "9780553293357",
			year:        1951,
			publisher:   "Bantam Spectra",
			description: "A mathematician predicts the fall of a galactic empire and founds a colony to preserve knowledge.",
			genre:       "Science Fiction",
			quantity:    2,
			authors:     []string{"Isaac Asimov"},
		},
		{
			title:       "Ender's Game",
			isbn:        "9780812550702",
			year:        1985,
			publisher:   "Tor Books",
			description: "A gifted child is trained in zero-gravity war games to lead humanity against an alien threat.",
			genre:       "Science Fiction",
			quantity:    4,
			authors:     []string{"Orson Scott Card"},
		},
		{
			title:       "Neuromancer",
			isbn:        "9780441569595",
			year:        1984,
			publisher:   "Ace Books",
			description: "A burned-out console cowboy takes one last job that pulls him through cyberspace and back.",
			genre:       "Science Fiction",
			quantity:    2,
			authors:     []string{"William Gibson"},
		},
		{
			title:       "Snow Crash",
			isbn:        "9780553380958",
			year:        1992,
			publisher:   "Bantam Spectra",
			description: "A pizza-delivering hacker chases a virus that infects programmers in both the real world and the metaverse.",
			genre:       "Science Fiction",
			quantity:    2,
			authors:     []string{"Neal Stephenson"},
		},
		{
			title:       "The Left Hand of Darkness",
			isbn:        "9780441478125",
			year:        1969,
			publisher:   "Ace Books",
			description: "An envoy to an ice-bound planet must navigate a society with no fixed gender.",
			genre:       "Science Fiction",
			quantity:    2,
			authors:     []string{"Ursula K. Le Guin"},
		},
		{
			title:       "The Martian",
			isbn:        "9780553418026",
			year:        2011,
			publisher:   "Broadway Books",
			description: "An astronaut stranded on Mars improvises his way to survival using botany, chemistry, and humor.",
			genre:       "Science Fiction",
			quantity:    4,
			authors:     []string{"Andy Weir"},
		},
		{
			title:       "Project Hail Mary",
			isbn:        "9780593135204",
			year:        2021,
			publisher:   "Ballantine Books",
			description: "A lone astronaut wakes with amnesia on a mission to save Earth from a stellar plague.",
			genre:       "Science Fiction",
			quantity:    3,
			authors:     []string{"Andy Weir"},
		},
		{
			title:       "Ready Player One",
			isbn:        "9780307887443",
			year:        2011,
			publisher:   "Broadway Books",
			description: "A teenage gunter races rival corporations to solve a billion-dollar easter egg in a virtual world.",
			genre:       "Science Fiction",
			quantity:    3,
			authors:     []string{"Ernest Cline"},
		},
		{
			title:       "The Three-Body Problem",
			isbn:        "9780765382030",
			year:        2008,
			publisher:   "Tor Books",
			description: "A Cultural Revolution-era signal reaches a dying alien civilization, with consequences for Earth.",
			genre:       "Science Fiction",
			quantity:    2,
			authors:     []string{"Liu Cixin"},
		},
		{
			title:       "Slaughterhouse-Five",
			isbn:        "9780385333849",
			year:        1969,
			publisher:   "Dial Press",
			description: "An American soldier becomes unstuck in time after surviving the firebombing of Dresden.",
			genre:       "Science Fiction",
			quantity:    2,
			authors:     []string{"Kurt Vonnegut"},
		},
		{
			title:       "Fahrenheit 451",
			isbn:        "9781451673319",
			year:        1953,
			publisher:   "Simon & Schuster",
			description: "A fireman whose job is to burn books begins to question the society that forbids them.",
			genre:       "Science Fiction",
			quantity:    3,
			authors:     []string{"Ray Bradbury"},
		},
		{
			title:       "The Handmaid's Tale",
			isbn:        "9780385490818",
			year:        1985,
			publisher:   "Anchor Books",
			description: "In a theocratic America, fertile women are enslaved as reproductive vessels for the ruling class.",
			genre:       "Science Fiction",
			quantity:    3,
			authors:     []string{"Margaret Atwood"},
		},
		{
			title:       "And Then There Were None",
			isbn:        "9780062073488",
			year:        1939,
			publisher:   "William Morrow",
			description: "Ten strangers invited to an island are murdered one by one according to a nursery rhyme.",
			genre:       "Mystery",
			quantity:    3,
			authors:     []string{"Agatha Christie"},
		},
		{
			title:       "Murder on the Orient Express",
			isbn:        "9780062073501",
			year:        1934,
			publisher:   "William Morrow",
			description: "Detective Hercule Poirot investigates a stabbing aboard a snowbound luxury train.",
			genre:       "Mystery",
			quantity:    2,
			authors:     []string{"Agatha Christie"},
		},
		{
			title:       "The Girl with the Dragon Tattoo",
			isbn:        "9780307454546",
			year:        2005,
			publisher:   "Vintage Crime",
			description: "A disgraced journalist and a brilliant hacker dig into a decades-old disappearance.",
			genre:       "Thriller",
			quantity:    3,
			authors:     []string{"Stieg Larsson"},
		},
		{
			title:       "Gone Girl",
			isbn:        "9780307588371",
			year:        2012,
			publisher:   "Crown Publishing",
			description: "A husband becomes the prime suspect when his wife vanishes on their fifth anniversary.",
			genre:       "Thriller",
			quantity:    4,
			authors:     []string{"Gillian Flynn"},
		},
		{
			title:       "The Da Vinci Code",
			isbn:        "9780307474278",
			year:        2003,
			publisher:   "Anchor Books",
			description: "A symbologist and a cryptographer chase a conspiracy through the art of the Louvre.",
			genre:       "Thriller",
			quantity:    3,
			authors:     []string{"Dan Brown"},
		},
		{
			title:       "The Silent Patient",
			isbn:        "9781250301697",
			year:        2019,
			publisher:   "Celadon Books",
			description: "A therapist is obsessed with unlocking the silence of a painter who shot her husband.",
			genre:       "Thriller",
			quantity:    3,
			authors:     []string{"Alex Michaelides"},
		},
		{
			title:       "In the Woods",
			isbn:        "9780143113492",
			year:        2007,
			publisher:   "Penguin Books",
			description: "A Dublin detective working a child murder is haunted by his own childhood in the same woods.",
			genre:       "Mystery",
			quantity:    2,
			authors:     []string{"Tana French"},
		},
		{
			title:       "The Big Sleep",
			isbn:        "9780394758282",
			year:        1939,
			publisher:   "Vintage Crime",
			description: "Private eye Philip Marlowe takes a blackmail case that opens into something much worse.",
			genre:       "Mystery",
			quantity:    2,
			authors:     []string{"Raymond Chandler"},
		},
		{
			title:       "It",
			isbn:        "9781501142970",
			year:        1986,
			publisher:   "Scribner",
			description: "Seven adults return to their hometown to face the shape-shifting entity that terrorized their childhood.",
			genre:       "Horror",
			quantity:    3,
			authors:     []string{"Stephen King"},
		},
		{
			title:       "The Shining",
			isbn:        "9780307743657",
			year:        1977,
			publisher:   "Anchor Books",
			description: "An isolated caretaker at an empty hotel loses his grip as the building works on his family.",
			genre:       "Horror",
			quantity:    3,
			authors:     []string{"Stephen King"},
		},
		{
			title:       "Carrie",
			isbn:        "9780307743664",
			year:        1974,
			publisher:   "Anchor Books",
			description: "A bullied teenage girl discovers telekinetic powers just in time for prom night.",
			genre:       "Horror",
			quantity:    2,
			authors:     []string{"Stephen King"},
		},
		{
			title:       "Pet Sematary",
			isbn:        "9781501156700",
			year:        1983,
			publisher:   "Pocket Books",
			description: "A family discovers that the burial ground behind their new home returns the dead, but wrong.",
			genre:       "Horror",
			quantity:    2,
			authors:     []string{"Stephen King"},
		},
		{
			title:       "The Haunting of Hill House",
			isbn:        "9780143039983",
			year:        1959,
			publisher:   "Penguin Classics",
			description: "Four strangers summer in a house with a dark history and it begins to single one of them out.",
			genre:       "Horror",
			quantity:    2,
			authors:     []string{"Shirley Jackson"},
		},
		{
			title:       "The Hunger Games",
			isbn:        "9780439023528",
			year:        2008,
			publisher:   "Scholastic Press",
			description: "A teenage girl volunteers for a televised fight to the death to save her sister.",
			genre:       "Young Adult",
			quantity:    5,
			authors:     []string{"Suzanne Collins"},
		},
		{
			title:       "The Fault in Our Stars",
			isbn:        "9780142424179",
			year:        2012,
			publisher:   "Penguin Books",
			description: "Two teenagers meet in a cancer support group and fall into a love shadowed by their diagnoses.",
			genre:       "Young Adult",
			quantity:    4,
			authors:     []string{"John Green"},
		},
		{
			title:       "Twilight",
			isbn:        "9780316015844",
			year:        2005,
			publisher:   "Little, Brown and Company",
			description: "A high school girl moves to a rainy town and falls for a boy who turns out to be a vampire.",
			genre:       "Young Adult",
			quantity:    3,
			authors:     []string{"Stephenie Meyer"},
		},
		{
			title:       "Divergent",
			isbn:        "9780062024039",
			year:        2011,
			publisher:   "Katherine Tegen Books",
			description: "In a faction-divided dystopia, a girl who fits no category uncovers a conspiracy against her kind.",
			genre:       "Young Adult",
			quantity:    3,
			authors:     []string{"Veronica Roth"},
		},
		{
			title:       "Percy Jackson & The Lightning Thief",
			isbn:        "9780786838653",
			year:        2005,
			publisher:   "Disney-Hyperion",
			description: "A troubled twelve-year-old learns he is the son of Poseidon and is accused of stealing Zeus's bolt.",
			genre:       "Young Adult",
			quantity:    4,
			authors:     []string{"Rick Riordan"},
		},
		{
			title:       "The Giver",
			isbn:        "9780544336261",
			year:        1993,
			publisher:   "HMH Books for Young Readers",
			description: "A boy in a colorless, conflict-free community is chosen to inherit its hidden memories.",
			genre:       "Young Adult",
			quantity:    3,
			authors:     []string{"Lois Lowry"},
		},
		{
			title:       "Outlander",
			isbn:        "9780440212560",
			year:        1991,
			publisher:   "Dell Publishing",
			description: "A post-war British nurse is hurled from 1945 Scotland into the Jacobite uprising of 1743.",
			genre:       "Romance",
			quantity:    3,
			authors:     []string{"Diana Gabaldon"},
		},
		{
			title:       "Me Before You",
			isbn:        "9780143124542",
			year:        2012,
			publisher:   "Penguin Books",
			description: "A young caregiver is hired to bring joy back to a paralyzed man planning to end his life.",
			genre:       "Romance",
			quantity:    3,
			authors:     []string{"Jojo Moyes"},
		},
		{
			title:       "The Notebook",
			isbn:        "9780446605236",
			year:        1996,
			publisher:   "Warner Books",
			description: "An elderly man reads to his wife from a notebook recounting the summer that forged their love.",
			genre:       "Romance",
			quantity:    3,
			authors:     []string{"Nicholas Sparks"},
		},
		{
			title:       "Red, White & Royal Blue",
			isbn:        "9781250316776",
			year:        2019,
			publisher:   "St. Martin's Griffin",
			description: "The American First Son and the Prince of Wales stage a fake friendship that becomes something else.",
			genre:       "Romance",
			quantity:    3,
			authors:     []string{"Casey McQuiston"},
		},
		{
			title:       "Sapiens: A Brief History of Humankind",
			isbn:        "9780062316097",
			year:        2011,
			publisher:   "Harper",
			description: "A sweeping account of how Homo sapiens came to dominate the planet through shared fiction.",
			genre:       "Non-fiction",
			quantity:    4,
			authors:     []string{"Yuval Noah Harari"},
		},
		{
			title:       "Homo Deus: A Brief History of Tomorrow",
			isbn:        "9780062464316",
			year:        2016,
			publisher:   "Harper",
			description: "The follow-up to Sapiens asks what humans will become once we conquer famine, plague, and war.",
			genre:       "Non-fiction",
			quantity:    2,
			authors:     []string{"Yuval Noah Harari"},
		},
		{
			title:       "Educated",
			isbn:        "9780399590504",
			year:        2018,
			publisher:   "Random House",
			description: "A woman raised off-grid by survivalist parents claws her way into a doctorate at Cambridge.",
			genre:       "Memoir",
			quantity:    3,
			authors:     []string{"Tara Westover"},
		},
		{
			title:       "Becoming",
			isbn:        "9781524763138",
			year:        2018,
			publisher:   "Crown",
			description: "Michelle Obama traces her path from Chicago's South Side to the White House.",
			genre:       "Memoir",
			quantity:    3,
			authors:     []string{"Michelle Obama"},
		},
		{
			title:       "Born a Crime",
			isbn:        "9780399588174",
			year:        2016,
			publisher:   "Spiegel & Grau",
			description: "Trevor Noah's childhood as a mixed-race boy whose very existence was illegal under apartheid.",
			genre:       "Memoir",
			quantity:    3,
			authors:     []string{"Trevor Noah"},
		},
		{
			title:       "The Immortal Life of Henrietta Lacks",
			isbn:        "9781400052189",
			year:        2010,
			publisher:   "Crown",
			description: "A poor Black woman's cancer cells, taken without consent, became one of medicine's great tools.",
			genre:       "Non-fiction",
			quantity:    2,
			authors:     []string{"Rebecca Skloot"},
		},
		{
			title:       "Into the Wild",
			isbn:        "9780385486804",
			year:        1996,
			publisher:   "Anchor Books",
			description: "Jon Krakauer reconstructs the final months of a young idealist who walked alone into Alaska.",
			genre:       "Non-fiction",
			quantity:    2,
			authors:     []string{"Jon Krakauer"},
		},
		{
			title:       "A Short History of Nearly Everything",
			isbn:        "9780767908184",
			year:        2003,
			publisher:   "Broadway Books",
			description: "Bill Bryson surveys the big questions of science with the curiosity of a brilliant amateur.",
			genre:       "Non-fiction",
			quantity:    2,
			authors:     []string{"Bill Bryson"},
		},
		{
			title:       "Guns, Germs, and Steel",
			isbn:        "9780393317558",
			year:        1997,
			publisher:   "W. W. Norton",
			description: "Jared Diamond argues that geography, not genetics, shaped the unequal fates of human societies.",
			genre:       "History",
			quantity:    3,
			authors:     []string{"Jared Diamond"},
		},
		{
			title:       "The Devil in the White City",
			isbn:        "9780375725609",
			year:        2003,
			publisher:   "Vintage Books",
			description: "The 1893 Chicago World's Fair and the serial killer who used it as his hunting ground.",
			genre:       "History",
			quantity:    2,
			authors:     []string{"Erik Larson"},
		},
		{
			title:       "Hiroshima",
			isbn:        "9780679721031",
			year:        1946,
			publisher:   "Vintage Books",
			description: "Six survivors of the atomic bombing, in their own voices, on the hour their world ended.",
			genre:       "History",
			quantity:    2,
			authors:     []string{"John Hersey"},
		},
		{
			title:       "The Emperor of All Maladies",
			isbn:        "9781439170915",
			year:        2010,
			publisher:   "Scribner",
			description: "A biography of cancer, from ancient descriptions to the frontier of modern oncology.",
			genre:       "History",
			quantity:    2,
			authors:     []string{"Siddhartha Mukherjee"},
		},
		{
			title:       "Atomic Habits",
			isbn:        "9780735211292",
			year:        2018,
			publisher:   "Avery",
			description: "A practical system for building good habits and breaking bad ones through tiny daily changes.",
			genre:       "Self-help",
			quantity:    5,
			authors:     []string{"James Clear"},
		},
		{
			title:       "The 7 Habits of Highly Effective People",
			isbn:        "9781982137274",
			year:        1989,
			publisher:   "Simon & Schuster",
			description: "A character-based framework for personal and professional effectiveness.",
			genre:       "Self-help",
			quantity:    3,
			authors:     []string{"Stephen R. Covey"},
		},
		{
			title:       "Thinking, Fast and Slow",
			isbn:        "9780374533557",
			year:        2011,
			publisher:   "Farrar, Straus and Giroux",
			description: "Daniel Kahneman maps the two systems that drive the way we think, judge, and decide.",
			genre:       "Psychology",
			quantity:    3,
			authors:     []string{"Daniel Kahneman"},
		},
		{
			title:       "Man's Search for Meaning",
			isbn:        "9780807014295",
			year:        1946,
			publisher:   "Beacon Press",
			description: "A psychiatrist's account of surviving Auschwitz and the theory of purpose it produced.",
			genre:       "Psychology",
			quantity:    4,
			authors:     []string{"Viktor E. Frankl"},
		},
		{
			title:       "Outliers: The Story of Success",
			isbn:        "9780316017930",
			year:        2008,
			publisher:   "Back Bay Books",
			description: "Malcolm Gladwell on the hidden advantages and cultural legacies behind extraordinary achievement.",
			genre:       "Non-fiction",
			quantity:    3,
			authors:     []string{"Malcolm Gladwell"},
		},
		{
			title:       "Meditations",
			isbn:        "9780812968255",
			year:        2002,
			publisher:   "Modern Library",
			description: "A Roman emperor's private notebook of Stoic reflection, never intended for publication.",
			genre:       "Philosophy",
			quantity:    3,
			authors:     []string{"Marcus Aurelius"},
		},
		{
			title:       "The Republic",
			isbn:        "9780872201361",
			year:        1992,
			publisher:   "Hackett Publishing",
			description: "Plato's foundational dialogue on justice, the ideal state, and the education of philosopher-kings.",
			genre:       "Philosophy",
			quantity:    2,
			authors:     []string{"Plato"},
		},
		{
			title:       "Charlotte's Web",
			isbn:        "9780064400558",
			year:        1952,
			publisher:   "HarperCollins",
			description: "A clever spider spins words in her web to save a pig named Wilbur from the slaughterhouse.",
			genre:       "Children's",
			quantity:    4,
			authors:     []string{"E.B. White"},
		},
		{
			title:       "Matilda",
			isbn:        "9780142410370",
			year:        1988,
			publisher:   "Puffin Books",
			description: "A gifted girl with terrible parents and a worse headmistress discovers she has a secret power.",
			genre:       "Children's",
			quantity:    4,
			authors:     []string{"Roald Dahl"},
		},
		{
			title:       "Charlie and the Chocolate Factory",
			isbn:        "9780142410318",
			year:        1964,
			publisher:   "Puffin Books",
			description: "Five children win a tour of a reclusive chocolatier's factory, but only one will leave intact.",
			genre:       "Children's",
			quantity:    3,
			authors:     []string{"Roald Dahl"},
		},
		{
			title:       "The Lion, the Witch and the Wardrobe",
			isbn:        "9780064404990",
			year:        1950,
			publisher:   "HarperCollins",
			description: "Four siblings step through a wardrobe into Narnia and join a talking lion's war against the Witch.",
			genre:       "Children's",
			quantity:    3,
			authors:     []string{"C.S. Lewis"},
		},
		{
			title:       "Watchmen",
			isbn:        "9780930289232",
			year:        1987,
			publisher:   "DC Comics",
			description: "A murdered vigilante's former allies uncover a plot that redefines the superhero genre.",
			genre:       "Graphic Novel",
			quantity:    2,
			authors:     []string{"Alan Moore", "Dave Gibbons"},
		},
		{
			title:       "Maus",
			isbn:        "9780679748403",
			year:        1986,
			publisher:   "Pantheon Books",
			description: "A cartoonist interviews his Holocaust-survivor father, depicting Jews as mice and Nazis as cats.",
			genre:       "Graphic Novel",
			quantity:    2,
			authors:     []string{"Art Spiegelman"},
		},
		{
			title:       "Persepolis",
			isbn:        "9780375714573",
			year:        2000,
			publisher:   "Pantheon Books",
			description: "Marjane Satrapi's illustrated memoir of growing up during the Iranian Revolution.",
			genre:       "Graphic Novel",
			quantity:    2,
			authors:     []string{"Marjane Satrapi"},
		},
		{
			title:       "The Pragmatic Programmer",
			isbn:        "9780135957059",
			year:        2019,
			publisher:   "Addison-Wesley Professional",
			description: "A working programmer's guide to writing flexible, dependable, and maintainable software.",
			genre:       "Technology",
			quantity:    3,
			authors:     []string{"David Thomas", "Andrew Hunt"},
		},
		{
			title:       "Clean Code",
			isbn:        "9780132350884",
			year:        2008,
			publisher:   "Prentice Hall",
			description: "Robert C. Martin's principles, patterns, and practices for writing readable, maintainable code.",
			genre:       "Technology",
			quantity:    3,
			authors:     []string{"Robert C. Martin"},
		},
		{
			title:       "Structure and Interpretation of Computer Programs",
			isbn:        "9780262510875",
			year:        1985,
			publisher:   "MIT Press",
			description: "The classic MIT text that teaches computation as a way of thinking, not just coding.",
			genre:       "Technology",
			quantity:    2,
			authors:     []string{"Harold Abelson", "Gerald Jay Sussman"},
		},
		{
			title:       "The Mythical Man-Month",
			isbn:        "9780201835953",
			year:        1995,
			publisher:   "Addison-Wesley Professional",
			description: "Fred Brooks on why adding people to a late software project makes it later, and other hard truths.",
			genre:       "Technology",
			quantity:    2,
			authors:     []string{"Frederick P. Brooks Jr."},
		},
		{
			title:       "The Kite Runner",
			isbn:        "9781594631931",
			year:        2003,
			publisher:   "Riverhead Books",
			description: "A privileged Afghan boy betrays his servant friend and spends a lifetime trying to atone.",
			genre:       "Literary Fiction",
			quantity:    3,
			authors:     []string{"Khaled Hosseini"},
		},
		{
			title:       "A Thousand Splendid Suns",
			isbn:        "9781594483851",
			year:        2007,
			publisher:   "Riverhead Books",
			description: "Two Afghan women bound together by war, marriage, and the search for hope.",
			genre:       "Literary Fiction",
			quantity:    2,
			authors:     []string{"Khaled Hosseini"},
		},
		{
			title:       "The Book Thief",
			isbn:        "9780375842207",
			year:        2005,
			publisher:   "Knopf Books for Young Readers",
			description: "Death narrates the story of a German girl who steals books in the shadow of the Third Reich.",
			genre:       "Historical Fiction",
			quantity:    3,
			authors:     []string{"Markus Zusak"},
		},
		{
			title:       "Life of Pi",
			isbn:        "9780156027328",
			year:        2001,
			publisher:   "Harcourt",
			description: "A shipwrecked Indian boy shares a lifeboat with a Bengal tiger for 227 days on the Pacific.",
			genre:       "Literary Fiction",
			quantity:    3,
			authors:     []string{"Yann Martel"},
		},
		{
			title:       "The Alchemist",
			isbn:        "9780062315007",
			year:        1988,
			publisher:   "HarperOne",
			description: "A young Andalusian shepherd follows a recurring dream across the desert toward a hidden treasure.",
			genre:       "Literary Fiction",
			quantity:    4,
			authors:     []string{"Paulo Coelho"},
		},
		{
			title:       "Beloved",
			isbn:        "9781400033416",
			year:        1987,
			publisher:   "Vintage Books",
			description: "A formerly enslaved woman in Ohio is haunted by the daughter she killed rather than see returned to slavery.",
			genre:       "Literary Fiction",
			quantity:    2,
			authors:     []string{"Toni Morrison"},
		},
		{
			title:       "The Color Purple",
			isbn:        "9780156031820",
			year:        1982,
			publisher:   "Harcourt",
			description: "Letters from a Black woman in the early 20th-century South chart her slow emergence into selfhood.",
			genre:       "Literary Fiction",
			quantity:    2,
			authors:     []string{"Alice Walker"},
		},
		{
			title:       "Where the Crawdads Sing",
			isbn:        "9780735219090",
			year:        2018,
			publisher:   "G.P. Putnam's Sons",
			description: "A girl raised alone in the North Carolina marsh becomes the prime suspect in a local murder.",
			genre:       "Literary Fiction",
			quantity:    4,
			authors:     []string{"Delia Owens"},
		},
		{
			title:       "The Midnight Library",
			isbn:        "9780525559474",
			year:        2020,
			publisher:   "Viking",
			description: "Between life and death, a woman visits a library of the lives she could have lived.",
			genre:       "Literary Fiction",
			quantity:    4,
			authors:     []string{"Matt Haig"},
		},
		{
			title:       "Tomorrow, and Tomorrow, and Tomorrow",
			isbn:        "9780593321201",
			year:        2022,
			publisher:   "Knopf",
			description: "Two friends build video games together across three decades of love, rivalry, and grief.",
			genre:       "Literary Fiction",
			quantity:    3,
			authors:     []string{"Gabrielle Zevin"},
		},
	}

	for _, b := range books {
		if err := dm.seedOneBook(b); err != nil {
			log.Fatalf("Failed to seed book %q: %v", b.title, err)
		}
	}

	log.Printf("Seeded %d books", len(books))
}

func (dm *DatabaseManager) seedOneBook(b seedBook) error {
	tx, err := dm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		INSERT INTO books (title, isbn, year, publisher, description, genre,
		                   quantity_total, quantity_available)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		b.title, b.isbn, b.year, b.publisher, b.description, b.genre,
		b.quantity, b.quantity)
	if err != nil {
		return err
	}
	bookID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	for _, authorName := range b.authors {
		var authorID int64
		err := tx.QueryRow("SELECT id FROM authors WHERE name = ?", authorName).Scan(&authorID)
		if err == sql.ErrNoRows {
			res, execErr := tx.Exec("INSERT INTO authors (name) VALUES (?)", authorName)
			if execErr != nil {
				return execErr
			}
			authorID, err = res.LastInsertId()
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		if _, err := tx.Exec(
			"INSERT INTO book_authors (book_id, author_id) VALUES (?, ?)",
			bookID, authorID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func findOrCreateAuthor(tx *sql.Tx, name string) (int, error) {
	var id int
	err := tx.QueryRow("SELECT id FROM authors WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	result, err := tx.Exec("INSERT INTO authors (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	id64, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id64), nil
}

func (dm *DatabaseManager) CreateBook(book *Book, authors []string) (int, error) {
	tx, err := dm.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
			INSERT INTO books (title, isbn, cover_filename, year, publisher, description, genre, quantity_total, quantity_available)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		book.Title, book.ISBN, book.CoverFilename, book.Year,
		book.Publisher, book.Description, book.Genre,
		book.QuantityTotal, book.QuantityAvailable)
	if err != nil {
		return 0, err
	}

	bookID64, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	bookID := int(bookID64)

	for i, name := range authors {
		authorID, err := findOrCreateAuthor(tx, name)
		if err != nil {
			return 0, err
		}
		if _, err := tx.Exec(
			"INSERT INTO book_authors (book_id, author_id, position) VALUES (?, ?, ?)",
			bookID, authorID, i+1,
		); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return bookID, nil
}

func (dm *DatabaseManager) UpdateBook(id int, book *Book, authors []string) error {
	tx, err := dm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		UPDATE books
		SET title = ?, isbn = ?, cover_filename = ?, year = ?, publisher = ?,
		    description = ?, genre = ?, quantity_total = ?, quantity_available = ?
		WHERE id = ?`,
		book.Title, book.ISBN, book.CoverFilename, book.Year,
		book.Publisher, book.Description, book.Genre,
		book.QuantityTotal, book.QuantityAvailable, id); err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM book_authors WHERE book_id = ?", id); err != nil {
		return err
	}

	for i, name := range authors {
		authorID, err := findOrCreateAuthor(tx, name)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(
			"INSERT INTO book_authors (book_id, author_id, position) VALUES (?, ?, ?)",
			id, authorID, i+1,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (dm *DatabaseManager) UpdateBookCover(id int, filename string) error {
	_, err := dm.db.Exec("UPDATE books SET cover_filename = ? WHERE id = ?", filename, id)
	return err
}

var ErrBookHasLoans = errors.New("db: book has loan history, cannot delete")

func (dm *DatabaseManager) DeleteBook(id int) error {
	tx, err := dm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var loanCount int
	if err := tx.QueryRow("SELECT COUNT(*) FROM loans WHERE book_id = ?", id).Scan(&loanCount); err != nil {
		return err
	}
	if loanCount > 0 {
		return ErrBookHasLoans
	}

	if _, err := tx.Exec("DELETE FROM books WHERE id = ?", id); err != nil {
		return err
	}

	return tx.Commit()
}

var ErrPatronHasLoans = errors.New("db: patron has loan history, cannot delete")

var (
	ErrNoCopiesAvailable   = errors.New("db: no copies available for check out")
	ErrLoanAlreadyReturned = errors.New("db: loan already returned")
	ErrPatronHasOverdue    = errors.New("db: patron has overdue loans, cannot check out")
	ErrPatronAtLoanLimit   = errors.New("db: patron at max active loans")
)

func (dm *DatabaseManager) GetAllPatrons() ([]Patron, error) {
	rows, err := dm.db.Query(`
		SELECT p.id, p.name, p.email, p.phone, p.joined_date, p.metadata,
		       COALESCE(u.username, ''), (u.temp_password IS NOT NULL)
		FROM patrons p
		LEFT JOIN users u ON u.patron_id = p.id
		ORDER BY p.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patrons []Patron
	for rows.Next() {
		var p Patron
		if err := rows.Scan(&p.ID, &p.Name, &p.Email, &p.Phone, &p.JoinedDate, &p.Metadata, &p.Username, &p.HasTempPassword); err != nil {
			return nil, err
		}
		patrons = append(patrons, p)
	}
	return patrons, rows.Err()
}

func (dm *DatabaseManager) GetPatronByID(id int) (*Patron, error) {
	p := &Patron{}
	err := dm.db.QueryRow(`
		SELECT p.id, p.name, p.email, p.phone, p.joined_date, p.metadata,
		       COALESCE(u.username, ''), (u.temp_password IS NOT NULL)
		FROM patrons p
		LEFT JOIN users u ON u.patron_id = p.id
		WHERE p.id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Email, &p.Phone, &p.JoinedDate, &p.Metadata, &p.Username, &p.HasTempPassword)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// CreatePatron inserts a patrons row and a linked users row in a single
// transaction per DEC-022. The username is auto-generated inside the
// transaction: generateBaseUsername produces a starting form, and we
// retry with "base", "base2", "base3", ... until SELECT COUNT returns
// zero. The COUNT uses COLLATE NOCASE so "jsmith" does not collide
// with an existing "JSmith"; the users.username column itself is not
// NOCASE today (pre-dates #21), so this check is the belt-and-
// suspenders until that schema fix lands. Returns the final username
// so the handler can flash it to the admin.
//
// Email and phone are passed as plain strings; empty string converts
// to a nil pointer before INSERT so the column stores NULL rather
// than a zero-length string. This keeps the DB shape honest about
// "not provided" vs "provided as empty".
func (dm *DatabaseManager) CreatePatron(name, email, phone, passwordHash string) (int, string, error) {
	tx, err := dm.db.Begin()
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback()

	var emailPtr, phonePtr *string
	if email != "" {
		emailPtr = &email
	}
	if phone != "" {
		phonePtr = &phone
	}

	res, err := tx.Exec(
		"INSERT INTO patrons (name, email, phone) VALUES (?, ?, ?)",
		name, emailPtr, phonePtr,
	)
	if err != nil {
		return 0, "", err
	}
	patronID64, err := res.LastInsertId()
	if err != nil {
		return 0, "", err
	}
	patronID := int(patronID64)

	base := generateBaseUsername(name)
	if base == "" {
		return 0, "", errors.New("db: cannot derive a username from the provided name")
	}

	username := base
	for suffix := 2; ; suffix++ {
		var count int
		if err := tx.QueryRow(
			"SELECT COUNT(*) FROM users WHERE username = ? COLLATE NOCASE",
			username,
		).Scan(&count); err != nil {
			return 0, "", err
		}
		if count == 0 {
			break
		}
		username = fmt.Sprintf("%s%d", base, suffix)
	}

	if _, err := tx.Exec(
		"INSERT INTO users (username, password_hash, role, patron_id) VALUES (?, ?, 'patron', ?)",
		username, passwordHash, patronID,
	); err != nil {
		return 0, "", err
	}

	if err := tx.Commit(); err != nil {
		return 0, "", err
	}
	return patronID, username, nil
}

func (dm *DatabaseManager) CreatePatronNoLogin(name, email, phone, metadataJSON string) (int, error) {
	var emailPtr, phonePtr, metaPtr *string
	if email != "" {
		emailPtr = &email
	}
	if phone != "" {
		phonePtr = &phone
	}
	if metadataJSON != "" {
		metaPtr = &metadataJSON
	}
	res, err := dm.db.Exec(
		"INSERT INTO patrons (name, email, phone, metadata) VALUES (?, ?, ?, ?)",
		name, emailPtr, phonePtr, metaPtr,
	)
	if err != nil {
		return 0, err
	}
	id64, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id64), nil
}

// CreatePatronWithLogin returns the plaintext temp password as the
// fourth return value. Never log it; it leaves this function only via
// the credentials CSV download once.
func (dm *DatabaseManager) CreatePatronWithLogin(name, email, phone, metadataJSON string) (int, int, string, string, error) {
	tempPassword, err := generateTempPassword()
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("db: generate temp password: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(tempPassword), bcrypt.DefaultCost)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("db: hash temp password: %w", err)
	}

	tx, err := dm.db.Begin()
	if err != nil {
		return 0, 0, "", "", err
	}
	defer tx.Rollback()

	var emailPtr, phonePtr, metaPtr *string
	if email != "" {
		emailPtr = &email
	}
	if phone != "" {
		phonePtr = &phone
	}
	if metadataJSON != "" {
		metaPtr = &metadataJSON
	}

	res, err := tx.Exec(
		"INSERT INTO patrons (name, email, phone, metadata) VALUES (?, ?, ?, ?)",
		name, emailPtr, phonePtr, metaPtr,
	)
	if err != nil {
		return 0, 0, "", "", err
	}
	patronID64, err := res.LastInsertId()
	if err != nil {
		return 0, 0, "", "", err
	}
	patronID := int(patronID64)

	base := generateBaseUsername(name)
	if base == "" {
		return 0, 0, "", "", errors.New("db: cannot derive a username from the provided name")
	}

	username := base
	for suffix := 2; ; suffix++ {
		var count int
		if err := tx.QueryRow(
			"SELECT COUNT(*) FROM users WHERE username = ? COLLATE NOCASE",
			username,
		).Scan(&count); err != nil {
			return 0, 0, "", "", err
		}
		if count == 0 {
			break
		}
		username = fmt.Sprintf("%s%d", base, suffix)
	}

	userRes, err := tx.Exec(
		`INSERT INTO users (username, password_hash, role, patron_id, must_change_password, temp_password)
		 VALUES (?, ?, 'patron', ?, 1, ?)`,
		username, string(hash), patronID, tempPassword,
	)
	if err != nil {
		return 0, 0, "", "", err
	}
	userID64, err := userRes.LastInsertId()
	if err != nil {
		return 0, 0, "", "", err
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, "", "", err
	}
	return patronID, int(userID64), username, tempPassword, nil
}

func (dm *DatabaseManager) FindPatronByExternalID(externalID string) (*Patron, error) {
	if externalID == "" {
		return nil, nil
	}
	p := &Patron{}
	var username sql.NullString
	err := dm.db.QueryRow(`
		SELECT p.id, p.name, p.email, p.phone, p.joined_date, p.metadata, COALESCE(u.username, '')
		FROM patrons p
		LEFT JOIN users u ON u.patron_id = p.id
		WHERE json_extract(p.metadata, '$.external_id') = ?
		LIMIT 1`, externalID,
	).Scan(&p.ID, &p.Name, &p.Email, &p.Phone, &p.JoinedDate, &p.Metadata, &username)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Username = username.String
	return p, nil
}

func (dm *DatabaseManager) FindPatronByEmail(email string) (*Patron, error) {
	if email == "" {
		return nil, nil
	}
	p := &Patron{}
	var username sql.NullString
	err := dm.db.QueryRow(`
		SELECT p.id, p.name, p.email, p.phone, p.joined_date, p.metadata, COALESCE(u.username, '')
		FROM patrons p
		LEFT JOIN users u ON u.patron_id = p.id
		WHERE p.email = ?
		LIMIT 1`, email,
	).Scan(&p.ID, &p.Name, &p.Email, &p.Phone, &p.JoinedDate, &p.Metadata, &username)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Username = username.String
	return p, nil
}

func (dm *DatabaseManager) UpdatePatron(id int, name, email, phone string) error {
	var emailPtr, phonePtr *string
	if email != "" {
		emailPtr = &email
	}
	if phone != "" {
		phonePtr = &phone
	}
	_, err := dm.db.Exec(
		"UPDATE patrons SET name = ?, email = ?, phone = ? WHERE id = ?",
		name, emailPtr, phonePtr, id,
	)
	return err
}

// DeletePatron removes the patron + their linked users + sessions rows
// in a single transaction per DEC-022. Guard fires if any loans row
// references this patron (active or returned) so history survives;
// admin's recovery path for a truly departed patron is to wait until
// the loan rows are archived or -- post-submission -- use a soft-
// delete flag. Order of deletes matters: sessions first (while users
// still exists for the subquery), then users, then patrons.
func (dm *DatabaseManager) DeletePatron(id int) error {
	tx, err := dm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var loanCount int
	if err := tx.QueryRow("SELECT COUNT(*) FROM loans WHERE patron_id = ?", id).Scan(&loanCount); err != nil {
		return err
	}
	if loanCount > 0 {
		return ErrPatronHasLoans
	}

	if _, err := tx.Exec(
		"DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE patron_id = ?)", id,
	); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM users WHERE patron_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM patrons WHERE id = ?", id); err != nil {
		return err
	}

	return tx.Commit()
}

// FetchAndStoreSeedCovers scans the books table for rows that have an
// ISBN but no cover_filename and opportunistically backfills covers
// from Open Library. Safe to call every startup: after a successful
// first run the SELECT returns zero rows and the function exits fast.
// Per-book failures (OL not-found, network, bad content, DB update)
// log a warning and continue -- never panic, never block the server
// from starting. Called from main.go after SeedBooks so a fresh DB
// gets real cover art for the seed books instead of placeholder slots.
//
// The network budget comes from the ctx the caller passes, plus a
// 10s per-request timeout inside FetchOpenLibraryBook and
// SaveCoverFromURL. If ctx fires, remaining books get skipped and
// their covers can be backfilled on the next startup.
func (dm *DatabaseManager) FetchAndStoreSeedCovers(ctx context.Context) {
	if !IsExternalAllowed(dm) {
		log.Printf("FetchAndStoreSeedCovers: offline mode -- skipping seed cover backfill")
		return
	}

	rows, err := dm.db.Query(`
		SELECT id, isbn FROM books
		WHERE cover_filename IS NULL AND isbn IS NOT NULL AND isbn != ''`)
	if err != nil {
		log.Printf("FetchAndStoreSeedCovers: SELECT: %v", err)
		return
	}

	type missing struct {
		id   int
		isbn string
	}
	var pending []missing
	for rows.Next() {
		var m missing
		if err := rows.Scan(&m.id, &m.isbn); err != nil {
			log.Printf("FetchAndStoreSeedCovers: scan: %v", err)
			rows.Close()
			return
		}
		pending = append(pending, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Printf("FetchAndStoreSeedCovers: rows.Err: %v", err)
		return
	}
	if len(pending) == 0 {
		return
	}

	log.Printf("FetchAndStoreSeedCovers: backfilling %d cover(s) from Open Library", len(pending))
	start := time.Now()
	saved := 0
	for _, m := range pending {
		if err := ctx.Err(); err != nil {
			log.Printf("FetchAndStoreSeedCovers: context cancelled, skipping remaining %d book(s): %v", len(pending)-saved, err)
			break
		}
		book, err := FetchOpenLibraryBook(ctx, m.isbn)
		if err != nil {
			log.Printf("FetchAndStoreSeedCovers: OL fetch for ISBN %s: %v", m.isbn, err)
			continue
		}
		if book.CoverURL == "" {
			log.Printf("FetchAndStoreSeedCovers: no cover URL from OL for ISBN %s", m.isbn)
			continue
		}
		filename, err := SaveCoverFromURL(book.CoverURL)
		if err != nil {
			log.Printf("FetchAndStoreSeedCovers: SaveCoverFromURL for ISBN %s: %v", m.isbn, err)
			continue
		}
		if err := dm.UpdateBookCover(m.id, filename); err != nil {
			log.Printf("FetchAndStoreSeedCovers: UpdateBookCover for book %d: %v", m.id, err)
			continue
		}
		saved++
	}
	log.Printf("FetchAndStoreSeedCovers: saved %d/%d cover(s) in %v", saved, len(pending), time.Since(start).Round(time.Millisecond))
}

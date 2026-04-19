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
	"time"

	"golang.org/x/crypto/bcrypt"

	_ "modernc.org/sqlite"
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

type StaffMember struct {
	ID        int
	Username  string
	Role      string
	CreatedAt string
}

type Patron struct {
	ID         int
	Name       string
	Email      *string
	Phone      *string
	JoinedDate string
	Metadata   *string
	Username   string
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

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		log.Fatalf("Failed to enable WAL mode: %v", err)
	}

	dm := &DatabaseManager{db: db}
	dm.createSchema()
	return dm
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
		book_id        INTEGER REFERENCES books(id),
		patron_id      INTEGER REFERENCES patrons(id),
		checked_out_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		due_date       DATETIME,
		returned_at    DATETIME
	);

	CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role          TEXT NOT NULL CHECK(role IN('admin', 'staff', 'patron')),
		patron_id     INTEGER REFERENCES patrons(id),
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token      TEXT PRIMARY KEY,
		user_id    INTEGER NOT NULL REFERENCES users(id),
		csrf_token TEXT NOT NULL,
		expires_at DATETIME NOT NULL
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

func (dm *DatabaseManager) GetUserByID(id int) (*User, error) {
	user := &User{}
	err := dm.db.QueryRow(
		"SELECT id, username, password_hash, role, patron_id FROM users where id = ?", id,
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
		"UPDATE users SET password_hash = ? WHERE id = ?",
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
		SELECT u.id, u.username, u.password_hash, u.role, u.patron_id, s.csrf_token
		FROM sessions s
		JOIN users u on s.user_id = u.id
		WHERE s.token = ? AND datetime(s.expires_at) > datetime('now')`,
		token,
	).Scan(&session.User.ID, &session.User.Username, &session.User.PasswordHash, &session.User.Role, &session.User.PatronID, &session.CSRFToken)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (dm *DatabaseManager) DeleteSession(token string) error {
	_, err := dm.db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
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
		if err := dm.CreateUser("patron1", string(hash), "patron", nil); err != nil {
			log.Fatalf("Failed to seed patron1 user: %v", err)
		}
		log.Println("Seeded patron1 user")
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

func (dm *DatabaseManager) GetAllPatrons() ([]Patron, error) {
	rows, err := dm.db.Query(`
		SELECT p.id, p.name, p.email, p.phone, p.joined_date, p.metadata, u.username
		FROM patrons p
		JOIN users u ON u.patron_id = p.id
		ORDER BY p.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patrons []Patron
	for rows.Next() {
		var p Patron
		if err := rows.Scan(&p.ID, &p.Name, &p.Email, &p.Phone, &p.JoinedDate, &p.Metadata, &p.Username); err != nil {
			return nil, err
		}
		patrons = append(patrons, p)
	}
	return patrons, rows.Err()
}

func (dm *DatabaseManager) GetPatronByID(id int) (*Patron, error) {
	p := &Patron{}
	err := dm.db.QueryRow(`
		SELECT p.id, p.name, p.email, p.phone, p.joined_date, p.metadata, u.username
		FROM patrons p
		JOIN users u ON u.patron_id = p.id
		WHERE p.id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Email, &p.Phone, &p.JoinedDate, &p.Metadata, &p.Username)
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

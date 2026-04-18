package main

import (
	"database/sql"
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
                WHERE ba.book_id = ?`, id)
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
		name TEXT NOT NULL UNIQUE
	);

	CREATE TABLE IF NOT EXISTS book_authors (
		book_id   INTEGER REFERENCES books(id),
		author_id INTEGER REFERENCES authors(id),
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

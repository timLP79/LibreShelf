package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func HandleCatalog(c *gin.Context) {
	dm := getDB(c)
	books, err := dm.GetAllBooks()
	if err != nil {
		log.Printf("Failed to fetch books: %v", err)
		c.Status(http.StatusInternalServerError)
		renderTemplate(c, "error", gin.H{
			"Title":   "Error",
			"Status":  500,
			"Message": "Unable to load catalog.",
		})
		return
	}

	renderTemplate(c, "catalog", gin.H{
		"Title":         "Catalog",
		"Books":         books,
		"Success":       readAndClearFlash(c, flashKindSuccess),
		"SuccessDetail": readAndClearFlashDetail(c),
		"Error":         readAndClearFlash(c, flashKindError),
	})
}

func HandleBookDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Status(http.StatusNotFound)
		renderTemplate(c, "error", gin.H{
			"Title":   "Not Found",
			"Status":  404,
			"Message": "Page not found",
		})
		return
	}

	dm := getDB(c)
	book, err := dm.GetBookByID(id)
	if err == sql.ErrNoRows {
		c.Status(http.StatusNotFound)
		renderTemplate(c, "error", gin.H{
			"Title":   "Not Found",
			"Status":  404,
			"Message": "Book not found",
		})
		return
	}
	if err != nil {
		log.Printf("Failed to fetch book %d: %v", id, err)
		c.Status(http.StatusInternalServerError)
		renderTemplate(c, "error", gin.H{
			"Title":   "Error",
			"Status":  500,
			"Message": "Unable to load book details.",
		})
		return
	}

	loans, err := dm.GetLoanHistory(id)
	if err != nil {
		log.Printf("Failed to fetch loan history for book %d: %v", id, err)
	}

	renderTemplate(c, "book_detail", gin.H{
		"Title":         book.Title,
		"Book":          book,
		"Loans":         loans,
		"Success":       readAndClearFlash(c, flashKindSuccess),
		"SuccessDetail": readAndClearFlashDetail(c),
		"Error":         readAndClearFlash(c, flashKindError),
	})
}

func HandleOpenLibraryLookup(c *gin.Context) {
	cleaned := stripISBNFormatting(c.Param("isbn"))
	if !IsValidISBN(cleaned) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_isbn"})
		return
	}

	book, err := FetchOpenLibraryBook(c.Request.Context(), cleaned)
	if errors.Is(err, ErrOpenLibraryNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if err != nil {
		log.Printf("HandleOpenLibraryLookup: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "upstream_unavailable"})
		return
	}

	c.JSON(http.StatusOK, book)
}

func HandleBookNew(c *gin.Context) {
	renderTemplate(c, "book_form", gin.H{
		"Title":          "Add Book",
		"FormTitle":      "Add Book",
		"FormSubtitle":   "Add a new book to the catalog",
		"FormAction":     "/books",
		"SubmitLabel":    "Create Book",
		"AuthorsText":    "",
		"ShowAddAnother": true,
		"Success":        readAndClearFlash(c, flashKindSuccess),
		"SuccessDetail":  readAndClearFlashDetail(c),
		"Error":          readAndClearFlash(c, flashKindError),
	})
}

func renderBookCreateForm(c *gin.Context, book *Book, authorsText, errMsg string) {
	c.Status(http.StatusBadRequest)
	renderTemplate(c, "book_form", gin.H{
		"Title":          "Add Book",
		"FormTitle":      "Add Book",
		"FormSubtitle":   "Add a new book to the catalog",
		"FormAction":     "/books",
		"SubmitLabel":    "Create Book",
		"Book":           book,
		"AuthorsText":    authorsText,
		"ShowAddAnother": true,
		"Error":          errMsg,
	})
}

func HandleBookCreate(c *gin.Context) {
	dm := getDB(c)

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		renderBookCreateForm(c, nil, "", "Invalid form submission.")
		return
	}

	title := normalizeFreeText(c.PostForm("title"))
	isbnRaw := strings.TrimSpace(c.PostForm("isbn"))
	authorsText := strings.TrimSpace(c.PostForm("authors"))
	publisher := normalizeFreeText(c.PostForm("publisher"))
	description := strings.TrimSpace(c.PostForm("description"))
	genre := normalizeFreeText(c.PostForm("genre"))
	yearStr := strings.TrimSpace(c.PostForm("year"))
	quantityStr := strings.TrimSpace(c.PostForm("quantity"))

	book := &Book{Title: title}
	if isbnRaw != "" {
		cleaned := stripISBNFormatting(isbnRaw)
		book.ISBN = &cleaned
	}
	if publisher != "" {
		book.Publisher = &publisher
	}
	if description != "" {
		book.Description = &description
	}
	if genre != "" {
		book.Genre = &genre
	}
	if yearStr != "" {
		if year, err := strconv.Atoi(yearStr); err == nil {
			book.Year = &year
		}
	}
	if q, err := strconv.Atoi(quantityStr); err == nil {
		book.QuantityTotal = q
		book.QuantityAvailable = q
	}

	if title == "" || len(title) > 255 {
		renderBookCreateForm(c, book, authorsText, "Title is required (1-255 characters).")
		return
	}

	var authors []string
	for _, raw := range strings.Split(authorsText, ",") {
		name := normalizeFreeText(raw)
		if name != "" {
			authors = append(authors, name)
		}
	}
	if len(authors) == 0 {
		renderBookCreateForm(c, book, authorsText, "At least one author is required.")
		return
	}

	if book.QuantityTotal < 1 {
		renderBookCreateForm(c, book, authorsText, "Quantity must be a positive integer.")
		return
	}

	if yearStr != "" && (book.Year == nil || *book.Year < 1500 || *book.Year > 2100) {
		renderBookCreateForm(c, book, authorsText, "Year must be between 1500 and 2100.")
		return
	}

	if book.ISBN != nil && !IsValidISBN(*book.ISBN) {
		renderBookCreateForm(c, book, authorsText, "ISBN must be 10 or 13 characters (digits, with optional hyphens or spaces).")
		return
	}

	coverURL := strings.TrimSpace(c.PostForm("cover_url"))
	if fh, err := c.FormFile("cover"); err == nil {
		saved, err := SaveUploadedCover(fh)
		if err != nil {
			switch {
			case errors.Is(err, ErrCoverTooLarge):
				renderBookCreateForm(c, book, authorsText, "Cover file exceeds 2MB limit.")
			case errors.Is(err, ErrCoverBadExtension):
				renderBookCreateForm(c, book, authorsText, "Cover must be a JPG, PNG, or WebP image.")
			case errors.Is(err, ErrCoverBadMimeType):
				renderBookCreateForm(c, book, authorsText, "Cover file contents do not match its extension.")
			default:
				log.Printf("HandleBookCreate: SaveUploadedCover: %v", err)
				renderBookCreateForm(c, book, authorsText, "Could not save cover image.")
			}
			return
		}
		book.CoverFilename = &saved
	} else if coverURL != "" {
		saved, err := SaveCoverFromURL(coverURL)
		if err != nil {
			log.Printf("HandleBookCreate: SaveCoverFromURL(%q): %v", coverURL, err)
			renderBookCreateForm(c, book, authorsText, "Could not download cover from Open Library. Try uploading a file instead.")
			return
		}
		book.CoverFilename = &saved
	}

	if book.ISBN != nil {
		if existing, err := dm.GetBookByISBN(*book.ISBN); err == nil {
			renderBookCreateForm(c, book, authorsText, fmt.Sprintf("A book with ISBN %s already exists (%q).", *book.ISBN, existing.Title))
			return
		} else if err != sql.ErrNoRows {
			log.Printf("HandleBookCreate: GetBookByISBN: %v", err)
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
	}

	bookID, err := dm.CreateBook(book, authors)
	if err != nil {
		log.Printf("HandleBookCreate: CreateBook: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	setFlash(c, flashKindSuccess, "book_created")
	setFlashDetail(c, book.Title)

	if c.PostForm("submit_action") == "add_another" {
		c.Redirect(http.StatusFound, "/books/new")
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/books/%d", bookID))
}

func HandleBookEdit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Status(http.StatusNotFound)
		renderTemplate(c, "error", gin.H{
			"Title":   "Not Found",
			"Status":  404,
			"Message": "Page not found",
		})
		return
	}

	dm := getDB(c)
	book, err := dm.GetBookByID(id)
	if err == sql.ErrNoRows {
		c.Status(http.StatusNotFound)
		renderTemplate(c, "error", gin.H{
			"Title":   "Not Found",
			"Status":  404,
			"Message": "Book not found",
		})
		return
	}
	if err != nil {
		log.Printf("HandleBookEdit: GetBookByID: %v", err)
		c.Status(http.StatusInternalServerError)
		renderTemplate(c, "error", gin.H{
			"Title":   "Error",
			"Status":  500,
			"Message": "Unable to load book for editing.",
		})
		return
	}

	renderTemplate(c, "book_form", gin.H{
		"Title":          "Edit Book",
		"FormTitle":      "Edit Book",
		"FormSubtitle":   book.Title,
		"FormAction":     fmt.Sprintf("/books/%d/edit", id),
		"SubmitLabel":    "Save Changes",
		"Book":           book,
		"AuthorsText":    book.Authors,
		"ShowAddAnother": false,
		"Error":          readAndClearFlash(c, flashKindError),
	})
}

func renderBookEditForm(c *gin.Context, id int, book *Book, authorsText, errMsg string) {
	c.Status(http.StatusBadRequest)
	renderTemplate(c, "book_form", gin.H{
		"Title":          "Edit Book",
		"FormTitle":      "Edit Book",
		"FormSubtitle":   book.Title,
		"FormAction":     fmt.Sprintf("/books/%d/edit", id),
		"SubmitLabel":    "Save Changes",
		"Book":           book,
		"AuthorsText":    authorsText,
		"ShowAddAnother": false,
		"Error":          errMsg,
	})
}

func HandleBookUpdate(c *gin.Context) {
	dm := getDB(c)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Status(http.StatusNotFound)
		renderTemplate(c, "error", gin.H{
			"Title":   "Not Found",
			"Status":  404,
			"Message": "Page not found",
		})
		return
	}

	existing, err := dm.GetBookByID(id)
	if err == sql.ErrNoRows {
		c.Status(http.StatusNotFound)
		renderTemplate(c, "error", gin.H{
			"Title":   "Not Found",
			"Status":  404,
			"Message": "Book not found",
		})
		return
	}
	if err != nil {
		log.Printf("HandleBookUpdate: GetBookByID: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		renderBookEditForm(c, id, existing, existing.Authors, "Invalid form submission.")
		return
	}

	title := normalizeFreeText(c.PostForm("title"))
	isbnRaw := strings.TrimSpace(c.PostForm("isbn"))
	authorsText := strings.TrimSpace(c.PostForm("authors"))
	publisher := normalizeFreeText(c.PostForm("publisher"))
	description := strings.TrimSpace(c.PostForm("description"))
	genre := normalizeFreeText(c.PostForm("genre"))
	yearStr := strings.TrimSpace(c.PostForm("year"))
	quantityStr := strings.TrimSpace(c.PostForm("quantity"))

	book := &Book{ID: id, Title: title, CoverFilename: existing.CoverFilename}
	if isbnRaw != "" {
		cleaned := stripISBNFormatting(isbnRaw)
		book.ISBN = &cleaned
	}
	if publisher != "" {
		book.Publisher = &publisher
	}
	if description != "" {
		book.Description = &description
	}
	if genre != "" {
		book.Genre = &genre
	}
	if yearStr != "" {
		if year, err := strconv.Atoi(yearStr); err == nil {
			book.Year = &year
		}
	}
	if q, err := strconv.Atoi(quantityStr); err == nil {
		book.QuantityTotal = q
		book.QuantityAvailable = q
	}

	if title == "" || len(title) > 255 {
		renderBookEditForm(c, id, book, authorsText, "Title is required (1-255 characters).")
		return
	}

	var authors []string
	for _, raw := range strings.Split(authorsText, ",") {
		name := normalizeFreeText(raw)
		if name != "" {
			authors = append(authors, name)
		}
	}
	if len(authors) == 0 {
		renderBookEditForm(c, id, book, authorsText, "At least one author is required.")
		return
	}

	if book.QuantityTotal < 1 {
		renderBookEditForm(c, id, book, authorsText, "Quantity must be a positive integer.")
		return
	}

	if yearStr != "" && (book.Year == nil || *book.Year < 1500 || *book.Year > 2100) {
		renderBookEditForm(c, id, book, authorsText, "Year must be between 1500 and 2100.")
		return
	}

	if book.ISBN != nil && !IsValidISBN(*book.ISBN) {
		renderBookEditForm(c, id, book, authorsText, "ISBN must be 10 or 13 characters (digits, with optional hyphens or spaces).")
		return
	}

	// Cover routing: new upload > OL URL > preserve existing.
	// oldCoverToDelete tracks the file that should be removed from disk
	// AFTER UpdateBook succeeds -- deleting early would orphan the row's
	// cover if the DB write failed.
	var oldCoverToDelete string
	coverURL := strings.TrimSpace(c.PostForm("cover_url"))
	if fh, err := c.FormFile("cover"); err == nil {
		saved, err := SaveUploadedCover(fh)
		if err != nil {
			switch {
			case errors.Is(err, ErrCoverTooLarge):
				renderBookEditForm(c, id, book, authorsText, "Cover file exceeds 2MB limit.")
			case errors.Is(err, ErrCoverBadExtension):
				renderBookEditForm(c, id, book, authorsText, "Cover must be a JPG, PNG, or WebP image.")
			case errors.Is(err, ErrCoverBadMimeType):
				renderBookEditForm(c, id, book, authorsText, "Cover file contents do not match its extension.")
			default:
				log.Printf("HandleBookUpdate: SaveUploadedCover: %v", err)
				renderBookEditForm(c, id, book, authorsText, "Could not save cover image.")
			}
			return
		}
		if existing.CoverFilename != nil {
			oldCoverToDelete = *existing.CoverFilename
		}
		book.CoverFilename = &saved
	} else if coverURL != "" {
		saved, err := SaveCoverFromURL(coverURL)
		if err != nil {
			log.Printf("HandleBookUpdate: SaveCoverFromURL(%q): %v", coverURL, err)
			renderBookEditForm(c, id, book, authorsText, "Could not download cover from Open Library. Try uploading a file instead.")
			return
		}
		if existing.CoverFilename != nil {
			oldCoverToDelete = *existing.CoverFilename
		}
		book.CoverFilename = &saved
	}

	// Duplicate-ISBN guard: another book (different id) already owns this
	// ISBN. Must exclude the book being edited from the conflict check,
	// otherwise a no-op edit that leaves ISBN unchanged would collide
	// with itself.
	if book.ISBN != nil {
		if other, err := dm.GetBookByISBN(*book.ISBN); err == nil && other.ID != id {
			renderBookEditForm(c, id, book, authorsText, fmt.Sprintf("A book with ISBN %s already exists (%q).", *book.ISBN, other.Title))
			return
		} else if err != nil && err != sql.ErrNoRows {
			log.Printf("HandleBookUpdate: GetBookByISBN: %v", err)
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
	}

	if err := dm.UpdateBook(id, book, authors); err != nil {
		log.Printf("HandleBookUpdate: UpdateBook: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	if oldCoverToDelete != "" {
		if err := os.Remove(filepath.Join(coversDir(), oldCoverToDelete)); err != nil && !os.IsNotExist(err) {
			log.Printf("HandleBookUpdate: cover cleanup (%s): %v", oldCoverToDelete, err)
		}
	}

	setFlash(c, flashKindSuccess, "book_updated")
	setFlashDetail(c, book.Title)
	c.Redirect(http.StatusFound, fmt.Sprintf("/books/%d", id))
}

func HandleBookDelete(c *gin.Context) {
	dm := getDB(c)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Status(http.StatusNotFound)
		renderTemplate(c, "error", gin.H{
			"Title":   "Not Found",
			"Status":  404,
			"Message": "Page not found",
		})
		return
	}

	book, err := dm.GetBookByID(id)
	if err == sql.ErrNoRows {
		c.Status(http.StatusNotFound)
		renderTemplate(c, "error", gin.H{
			"Title":   "Not Found",
			"Status":  404,
			"Message": "Book not found",
		})
		return
	}
	if err != nil {
		log.Printf("HandleBookDelete: GetBookByID: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	if err := dm.DeleteBook(id); err != nil {
		if errors.Is(err, ErrBookHasLoans) {
			setFlash(c, flashKindError, "book_has_loans")
			c.Redirect(http.StatusFound, fmt.Sprintf("/books/%d", id))
			return
		}
		log.Printf("HandleBookDelete: DeleteBook: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	// Best-effort cover cleanup AFTER the DB row is gone. Orphan files on
	// disk are better than pointing at a filename that no longer exists.
	if book.CoverFilename != nil {
		if err := os.Remove(filepath.Join(coversDir(), *book.CoverFilename)); err != nil && !os.IsNotExist(err) {
			log.Printf("HandleBookDelete: cover cleanup (%s): %v", *book.CoverFilename, err)
		}
	}

	setFlash(c, flashKindSuccess, "book_deleted")
	setFlashDetail(c, book.Title)
	c.Redirect(http.StatusFound, "/catalog")
}

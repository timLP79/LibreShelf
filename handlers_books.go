package main

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

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
		"Title": "Catalog",
		"Books": books,
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
		"Title": book.Title,
		"Book":  book,
		"Loans": loans,
	})
}

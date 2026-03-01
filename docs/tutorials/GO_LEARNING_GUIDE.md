# Go Learning Guide - Building a Web App

This guide teaches Go syntax and concepts as we build a web application together.

---

## Understanding `main.go` - Line by Line

Let's build this file together and I'll explain each concept:

### Part 1: The `package` declaration

```go
package main
```

**What this means:**
- Every Go file starts with a `package` declaration
- `package main` is special — it means "this is an executable program"
- Other packages (like `package database` or `package handlers`) are libraries
- Only `package main` can have a `func main()` that runs when you execute the program

### Part 2: Imports

```go
import (
	"github.com/gin-gonic/gin"
	"html/template"
)
```

**What this means:**
- `import` brings in code from other packages
- Parentheses `( )` let you import multiple packages at once
- `"html/template"` — built into Go (part of standard library)
- `"github.com/gin-gonic/gin"` — external package you installed with `go get`
- The last part of the path (`gin`, `template`) is how you use them in code

**How you use imports:**
```go
router := gin.Default()        // "gin" comes from the import path
tmpl := template.New("foo")    // "template" comes from the import path
```

### Part 3: Package-level variable

```go
var templates map[string]*template.Template
```

**Breaking this down:**
- `var` declares a variable
- `templates` is the variable name
- `map[string]*template.Template` is the type

**What's a map?**
- Like a dictionary/object in JavaScript: key → value
- `map[string]...` means "keys are strings"
- `*template.Template` means "values are pointers to Template objects"

**What's a pointer (`*`)?**
- In Go, `*` means "a pointer to" (memory address)
- `template.Template` is a value (makes a copy)
- `*template.Template` is a reference (points to the original)
- For large objects like templates, pointers are more efficient

**Why is this outside any function?**
- Variables declared at package level are accessible to all functions in the file
- This will be shared between `main()` and `handleIndex()`

### Part 4: The `main()` function

```go
func main() {
```

**What this means:**
- `func` declares a function
- `main()` is special — it runs automatically when you execute the program
- No parameters `()` and no return type

### Part 5: Initialize the map

```go
	templates = make(map[string]*template.Template)
```

**Breaking this down:**
- `make()` is a built-in function that creates maps, slices, and channels
- `map[string]*template.Template` is the type (same as the variable declaration)
- Without `make()`, the map would be `nil` (null) and you'd crash when trying to use it
- Now `templates` is an empty map ready to store key-value pairs

### Part 6: Parse templates

```go
	templates["index"] = template.Must(template.ParseFiles(
		"templates/layout.html",
		"templates/index.html",
	))
```

**Breaking this down piece by piece:**

**`template.ParseFiles("templates/layout.html", "templates/index.html")`**
- Reads both files and combines them into one template
- Returns two values: `(*template.Template, error)`
- Go functions can return multiple values!

**`template.Must(...)`**
- Wraps the result from `ParseFiles`
- If there's an error, it panics (crashes the program immediately)
- If no error, it returns just the `*template.Template` (strips off the error)
- Use `Must()` when you want the program to crash at startup if templates are broken

**`templates["index"] = ...`**
- Stores the parsed template in the map with key `"index"`
- Later you can retrieve it: `templates["index"]`

### Part 7: Create the router

```go
	router := gin.Default()
```

**Breaking this down:**
- `:=` is short variable declaration (combines `var router ...` and assignment)
- `gin.Default()` creates a new web server with logging and error recovery built in
- `router` is now a `*gin.Engine` (the router type)

**Note:** You can only use `:=` inside functions, not at package level.

### Part 8: Register a route

```go
	router.GET("/", handleIndex)
```

**What this means:**
- Register a handler for `GET` requests to the path `"/"`
- `handleIndex` is the function that will run (we define it below)
- Note: We pass the function itself, not `handleIndex()` (don't call it here)

### Part 9: Start the server

```go
	router.Run(":3000")
```

**What this means:**
- Start the HTTP server on port 3000
- `":3000"` means "listen on all network interfaces, port 3000"
- This function blocks forever (keeps running until you Ctrl+C)

**End of `main()`:**
```go
}
```

---

## The Handler Function

```go
func handleIndex(c *gin.Context) {
```

**Breaking this down:**
- `func handleIndex` declares a function named `handleIndex`
- `(c *gin.Context)` — takes one parameter:
  - Name: `c`
  - Type: `*gin.Context` (pointer to a Context)
  - `gin.Context` contains the HTTP request, response, and helper methods
- No return type — handlers don't return anything, they write to the response

### Setting the HTTP status

```go
	c.Writer.WriteHeader(200)
```

**What this means:**
- `c.Writer` is the HTTP response writer
- `WriteHeader(200)` sets the status code to 200 (OK)
- You could use `404`, `500`, etc.

### Rendering the template

```go
	templates["index"].ExecuteTemplate(c.Writer, "layout", gin.H{
		"Title": "Hello World",
	})
```

**Breaking this down:**

**`templates["index"]`**
- Look up the template we stored earlier
- Returns a `*template.Template`

**`.ExecuteTemplate(c.Writer, "layout", ...)`**
- `ExecuteTemplate` renders a specific template by name
- `c.Writer` — where to write the HTML output (the HTTP response)
- `"layout"` — which template definition to execute (from `{{define "layout"}}`)
- Third parameter is the data to pass to the template

**`gin.H{ ... }`**
- `gin.H` is a shortcut type for `map[string]interface{}`
- `interface{}` means "any type" (like `any` in TypeScript)
- This creates a map with one key-value pair: `"Title"` → `"Hello World"`
- In the template, `{{.Title}}` will become `"Hello World"`

**End of function:**
```go
}
```

---

## Go Syntax Summary

| Concept | Syntax | Example |
|---------|--------|---------|
| Package declaration | `package name` | `package main` |
| Import | `import "path"` or `import ( ... )` | `import "html/template"` |
| Variable declaration | `var name type` | `var templates map[string]*template.Template` |
| Short variable declaration | `name := value` | `router := gin.Default()` |
| Function declaration | `func name(params) returnType { }` | `func handleIndex(c *gin.Context) { }` |
| Map type | `map[keyType]valueType` | `map[string]*template.Template` |
| Pointer type | `*Type` | `*gin.Context` |
| Create map | `make(map[K]V)` | `make(map[string]*template.Template)` |
| Map assignment | `mapName[key] = value` | `templates["index"] = ...` |
| Call function | `functionName(args)` | `gin.Default()` |
| Method call | `object.Method(args)` | `router.GET("/", handleIndex)` |
| Composite literal (struct/map) | `Type{field: value}` | `gin.H{"Title": "Hello World"}` |

---

## Key Go Concepts

1. **Strong typing** — every variable has a specific type
2. **Multiple return values** — functions can return `(value, error)`
3. **Pointers** — `*Type` is a reference, not a copy
4. **Short declarations** — `:=` infers the type automatically
5. **First-class functions** — you can pass functions as arguments
6. **Package-level vs function-level** — `var` at top is shared, `:=` is local

---

## Complete `main.go` Code

```go
package main

import (
	"github.com/gin-gonic/gin"
	"html/template"
)

var templates map[string]*template.Template

func main() {
	// Load templates
	templates = make(map[string]*template.Template)
	templates["index"] = template.Must(template.ParseFiles(
		"templates/layout.html",
		"templates/index.html",
	))

	// Setup router
	router := gin.Default()
	router.GET("/", handleIndex)

	// Start server
	router.Run(":3000")
}

func handleIndex(c *gin.Context) {
	c.Writer.WriteHeader(200)
	templates["index"].ExecuteTemplate(c.Writer, "layout", gin.H{
		"Title": "Hello World",
	})
}
```

---

---

## Structs — Grouping Related Data

A **struct** is Go's equivalent of a class — it groups related data together.

```go
type DatabaseManager struct {
    db *sql.DB
}
```

- `type DatabaseManager struct` — declares a new type named `DatabaseManager`
- `db *sql.DB` — one field: a pointer to a database connection
- The lowercase `db` means it's **unexported** (private to the package)

In Go, uppercase = public (accessible from anywhere), lowercase = private (only within the package). This is the entire visibility system — no `public`/`private` keywords needed.

---

## Constructor Pattern — `NewDatabaseManager`

Go doesn't have constructors. The convention is a function named `New<Type>`:

```go
func NewDatabaseManager(dbPath string) *DatabaseManager {
    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }
    return &DatabaseManager{db: db}
}
```

- Takes a `string` parameter, returns a `*DatabaseManager` (pointer)
- `&DatabaseManager{db: db}` — creates the struct and returns its address
- The `&` operator takes the address of a value (creates a pointer)

---

## Error Handling — The Go Idiom

Go functions that can fail return an `error` as their last value. The idiom is:

```go
db, err := sql.Open("sqlite", dbPath)
if err != nil {
    log.Fatalf("Failed to open database: %v", err)
}
```

- `err != nil` means an error occurred (`nil` = no error)
- `log.Fatalf` prints the error message and immediately exits the program
- `%v` in the format string means "print this value in its default format"

You'll see this pattern constantly in Go. Every function that can fail follows it. There are no exceptions — errors are just values you check explicitly.

**`Fatalf` vs `Errorf`:**
- `log.Fatalf` — print and exit (use for unrecoverable errors at startup)
- `t.Errorf` — report test failure but keep running
- `t.Fatalf` — report test failure and stop the test immediately

---

## Methods — Functions Attached to Types

A **method** is a function with a receiver — it's attached to a type:

```go
func (dm *DatabaseManager) createSchema() {
    // dm is the DatabaseManager this method is called on
    dm.db.Exec(schema)
}
```

- `(dm *DatabaseManager)` is the **receiver** — like `self` in Python or `this` in JS
- `dm` is just a name (convention: 1-2 letters matching the type)
- Called as: `dm.createSchema()`

The receiver is a pointer (`*DatabaseManager`) so the method can modify the struct. If it were a value receiver (`DatabaseManager`), it would get a copy and changes wouldn't persist.

---

## Underscore Imports — Side-Effect Imports

```go
import _ "modernc.org/sqlite"
```

The `_` means: "import this package only for its side effects — I won't call anything from it directly."

SQLite drivers register themselves with Go's `database/sql` package when they load. Without the `_` import, the driver never loads and `sql.Open("sqlite", ...)` fails at runtime. Without the `_`, the compiler rejects it as an unused import.

---

## Type Assertions

When you store a value as an interface (any type), you need a type assertion to get the concrete type back:

```go
return c.MustGet("db").(*DatabaseManager)
```

- `c.MustGet("db")` returns `interface{}` — Go's "any type"
- `.(*DatabaseManager)` asserts "I know this is actually a `*DatabaseManager`"
- If it's the wrong type at runtime, Go panics — so only use this when you're sure

---

## Closures — Functions That Capture Variables

```go
func DatabaseMiddleware(dm *DatabaseManager) gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Set("db", dm)
        c.Next()
    }
}
```

`DatabaseMiddleware` is a **factory function** — it returns another function. The inner function "closes over" `dm`, capturing it from the outer scope. Every time the inner function runs (on each HTTP request), it still has access to `dm` even though `DatabaseMiddleware` has already returned.

This is a **closure**. It's how Go passes configuration into middleware without global variables.

---

## Range Loops — Iterating Over Slices

```go
templateNames := []string{"index", "catalog", "book_detail"}
for _, name := range templateNames {
    templates[name] = template.Must(template.ParseFiles(
        "templates/layout.html",
        "templates/"+name+".html",
    ))
}
```

- `[]string{...}` is a **slice** — Go's dynamic array
- `for _, name := range templateNames` — loop over every element
- `_` discards the index (0, 1, 2...) — we only need the value
- String concatenation uses `+` like most languages

---

## Environment Variables

```go
port := os.Getenv("PORT")
if port == "" {
    port = "3000"
}
```

`os.Getenv` returns the value of an environment variable, or an empty string if it's not set. This is how you configure an app differently in development vs production without changing code:

```bash
PORT=8080 ./go-full-stack   # uses 8080
./go-full-stack              # uses 3000 (default)
```

---

## URL Parameters

```go
router.GET("/books/:id", HandleBookDetail)

func HandleBookDetail(c *gin.Context) {
    id := c.Param("id")
    // id = "42" when URL is /books/42
}
```

The `:id` in the route is a **wildcard parameter**. Gin captures whatever comes after `/books/` and makes it available via `c.Param("id")`.

---

## Go Syntax Summary (Updated)

| Concept | Syntax | Example |
|---------|--------|---------|
| Package declaration | `package name` | `package main` |
| Import | `import "path"` | `import "html/template"` |
| Side-effect import | `import _ "path"` | `import _ "modernc.org/sqlite"` |
| Variable declaration | `var name type` | `var templates map[string]*template.Template` |
| Short declaration | `name := value` | `router := gin.Default()` |
| Struct declaration | `type Name struct { }` | `type DatabaseManager struct { db *sql.DB }` |
| Method | `func (r *Type) Name() { }` | `func (dm *DatabaseManager) createSchema()` |
| Constructor | `func NewType(...) *Type` | `func NewDatabaseManager(path string) *DatabaseManager` |
| Error check | `if err != nil { }` | `if err != nil { log.Fatalf(...) }` |
| Slice literal | `[]Type{values}` | `[]string{"index", "catalog"}` |
| Range loop | `for i, v := range slice` | `for _, name := range templateNames` |
| Type assertion | `value.(Type)` | `c.MustGet("db").(*DatabaseManager)` |
| Address of | `&value` | `&DatabaseManager{db: db}` |
| Environment var | `os.Getenv("NAME")` | `os.Getenv("PORT")` |
| URL parameter | `c.Param("name")` | `c.Param("id")` |

---

## Complete File Reference

| File | Key Concepts |
|------|-------------|
| `main.go` | Package, imports, `var`, `func main`, router setup, range loop, env vars |
| `db.go` | Struct, constructor, methods, error handling, underscore import |
| `handlers.go` | Functions, closures, middleware, type assertions, URL params |
| `main_test.go` | Test functions, `t.Helper`, `t.Cleanup`, `httptest`, table-driven testing |

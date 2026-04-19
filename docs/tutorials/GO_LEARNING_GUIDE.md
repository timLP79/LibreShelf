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

---

# Part 2 - Intermediate Go: Concepts from CP3 through CP5

Everything up to this point covered the skeleton: a single `main.go`, one handler, one struct, one database, one test file. From CP3 onward the codebase grew to multiple handlers, transactions, file uploads, HTTP clients, middleware chains, cryptographic primitives, and a real test suite. Each section below picks one concept, shows the exact code from this project that uses it, and breaks it down line by line.

---

## `defer` - Scheduling Cleanup

```go
tx, err := dm.db.Begin()
if err != nil {
    return err
}
defer tx.Rollback()
```

**What `defer` does:**
- Schedules a function call to run **when the surrounding function returns**
- Fires on normal return, early return, and panic - all three
- Multiple `defer` statements run in LIFO (last-in, first-out) order

**Why we need it here:**
- You begin a transaction, then need to either `Commit` or `Rollback` before the function ends
- If you forget to call `Rollback` on an error path, you leak a DB transaction
- `defer tx.Rollback()` guarantees cleanup fires even if you add a new early-return branch later
- Once `tx.Commit()` succeeds, the deferred `Rollback` becomes a no-op (SQLite returns "transaction already committed" silently)

**Other places you use defer in this project:**
```go
resp, err := client.Do(req)      // openlibrary.go
if err != nil { return nil, err }
defer resp.Body.Close()

dst, err := os.Create(destPath)   // covers.go
if err != nil { return "", err }
defer dst.Close()
```

**Go vs JS:** JavaScript has `try / finally`. Go has `defer`. Same guarantee - cleanup runs regardless of how the function exits - but cleaner syntax, since the cleanup lives next to the resource acquisition instead of all the way at the bottom.

---

## Database Transactions - The `DEC-022` Pattern

```go
func (dm *DatabaseManager) CreatePatron(name, email, phone, hash string) (int, string, error) {
    tx, err := dm.db.Begin()
    if err != nil {
        return 0, "", err
    }
    defer tx.Rollback()

    result, err := tx.Exec("INSERT INTO patrons (name, email, phone) VALUES (?, ?, ?)",
        name, email, phone)
    if err != nil {
        return 0, "", err
    }
    patronID, _ := result.LastInsertId()

    if _, err := tx.Exec(
        "INSERT INTO users (username, password_hash, role, patron_id) VALUES (?, ?, 'patron', ?)",
        username, hash, patronID,
    ); err != nil {
        return 0, "", err
    }

    if err := tx.Commit(); err != nil {
        return 0, "", err
    }
    return int(patronID), username, nil
}
```

**Breaking this down:**

**`dm.db.Begin()`**
- Starts a new transaction, returns `(*sql.Tx, error)`
- Until `Commit()` or `Rollback()`, nothing is visible to other connections

**`tx.Exec(...)` vs `dm.db.Exec(...)`**
- `tx.Exec` runs inside the transaction
- `dm.db.Exec` runs outside it, as a separate connection
- **Rule:** once you have a `tx`, use it for every query in that logical unit. Mixing tx and non-tx queries is a source of hard-to-find bugs

**`defer tx.Rollback()`**
- If any early return fires, Rollback runs and the partial writes disappear
- If `tx.Commit()` succeeds, the deferred Rollback becomes a no-op

**`tx.Commit()`**
- Makes all the writes atomic and durable
- Must be explicit. Nothing is saved until you call it

**Why transactions matter in this app:**
- `CreatePatron` writes to `patrons` AND `users` - one without the other is inconsistent
- `DeletePatron` cleans sessions, users, and patrons - partial delete leaves orphaned rows
- `UpdateUserPassword` updates the hash AND wipes active sessions - without the session wipe, stale tokens survive a password reset

Project standard: any multi-statement write goes in a transaction. See `DECISIONS.md` DEC-022.

---

## Sentinel Errors

```go
var ErrBookHasLoans = errors.New("book has loan history and cannot be deleted")
```

**Breaking this down:**
- `errors.New(...)` creates a new error value
- Assigning it to a package-level `var` gives it a stable identity that callers can compare against
- Convention: sentinel error names start with `Err`

**Returning it - in the DB method:**
```go
if count > 0 {
    return ErrBookHasLoans
}
```

**Checking for it - in the handler:**
```go
err := dm.DeleteBook(id)
if errors.Is(err, ErrBookHasLoans) {
    setFlash(c, flashKindError, "book_has_loans")
    c.Redirect(http.StatusSeeOther, "/books")
    return
}
```

**Why `errors.Is(err, target)` instead of `err == target`?**
- Both work for a plain sentinel
- `errors.Is` also walks error wrappers (next section), so a wrapped `ErrBookHasLoans` still matches
- Treat `errors.Is` as the default - `==` only works on unwrapped errors

**Built-in sentinels you meet:**
- `sql.ErrNoRows` - the row didn't exist (e.g. `SELECT ... WHERE id = ?` found nothing)
- `io.EOF` - reader has no more data

**Project's sentinel errors:**
- `ErrBookHasLoans`, `ErrPatronHasLoans` - in `db.go`
- `ErrOpenLibraryNotFound` - in `openlibrary.go`
- `ErrCoverTooLarge`, `ErrCoverBadExtension`, `ErrCoverBadMimeType` - in `covers.go`

**Pattern: sentinel errors let the business logic in the DB layer signal "this is a known, expected failure mode" without returning magic strings or status codes.** The handler decides how to surface it (flash banner? HTTP status?). That separation keeps the DB layer HTTP-free.

---

## Error Wrapping - `%w`

```go
if err != nil {
    return nil, fmt.Errorf("open library request: %w", err)
}
```

**What `%w` does:**
- Like `%v`, but **wraps** the underlying error so it can be unwrapped later
- Final message reads: `"open library request: dial tcp: connection refused"`
- `errors.Is(err, target)` and `errors.As(err, &targetType)` walk the wrapper chain

**Why wrap at all?**
- Adds context: which part of the code hit this error
- Preserves the original error identity, so callers can still identify it with `errors.Is`
- Plain `fmt.Errorf("...: %v", err)` prints the same text but **loses** the original error identity - `errors.Is` stops matching

**Rule of thumb:** when you return an error from something you called, wrap with `%w` and add one short phrase of context. Over time you get errors like:

```
HandleBookCreate: save cover: open library request: dial tcp: connection refused
```

Each layer adds one phrase. You can still call `errors.Is(err, someSentinel)` and it walks the whole chain.

---

## Variadic Parameters

```go
log.Printf("HandleBookCreate: saved cover %s for book %d", filename, bookID)
```

**The three dots in the signature:**
```go
func Printf(format string, args ...interface{})
```

- `args ...interface{}` means "accept any number of arguments of any type"
- Inside the function, `args` is a slice: `[]interface{}`
- Callers pass the args one at a time, not as a slice

**When you need to pass a slice as variadic, spread it with `...`:**
```go
parts := []interface{}{filename, bookID}
log.Printf("saved cover %s for book %d", parts...)  // spread the slice
```

**Where you've seen this:** every `log.Printf`, `fmt.Errorf`, `t.Errorf`, `strings.Join` (kind of - its second arg is a separator, not variadic), and `router.Use(middleware...)` in the route setup.

---

## Context - Deadlines, Cancellation, and Plumbing

```go
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()
dm.FetchAndStoreSeedCovers(ctx)
```

**What a `context.Context` carries:**
- **Deadlines / timeouts** - "give up after 60 seconds"
- **Cancellation** - "the caller has moved on, stop working"
- **Request-scoped values** - rarely used in this project

**`context.Background()`**
- The root context. No deadline, never cancelled
- Use only at the top of a program (main, init, tests)

**`context.WithTimeout(parent, duration)`**
- Returns a child context that gets cancelled after `duration`
- Returns a `cancel` function - **always defer-call it** to release resources early if the work finishes before the timeout

**How it propagates down the call chain:**
```go
func FetchOpenLibraryBook(ctx context.Context, isbn string) (*OpenLibraryBook, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    // ...
    client := &http.Client{Timeout: openLibraryTimeout}
    resp, err := client.Do(req)  // aborts if ctx fires
}
```

The HTTP client respects `ctx.Done()` automatically. Any library that takes a context will promptly abort when the context is cancelled.

**In this app:** `main.go:41` sets a 300s budget for the whole seed-cover backfill. `FetchAndStoreSeedCovers` checks `ctx.Err()` at the top of each iteration and breaks out if the deadline has fired. The remaining books get skipped and the next restart picks them up.

**Key insight:** context is how Go does "cooperative cancellation." No thread killing, no exceptions. The caller signals "stop", and well-behaved callees notice and return.

---

## Struct Tags and JSON

```go
type OpenLibraryBook struct {
    Title       string   `json:"title,omitempty"`
    Authors     []string `json:"authors,omitempty"`
    PublishYear int      `json:"publish_year,omitempty"`
    Publisher   string   `json:"publisher,omitempty"`
    CoverURL    string   `json:"cover_url,omitempty"`
}
```

**What the backticks are:**
- Raw string literals (no escape processing)
- Go uses them for **struct tags**: metadata attached to a field

**Reading the tag `json:"title,omitempty"`:**
- `json:` - the tag's namespace (other libraries use `db:`, `validate:`, etc.)
- `"title"` - when marshaling to JSON, use the key `title` (instead of `Title`)
- `omitempty` - if the field has its zero value (empty string, 0, nil slice), skip it entirely

**Marshaling (struct to JSON):**
```go
b := OpenLibraryBook{Title: "Dune"}
data, _ := json.Marshal(b)
// data = []byte(`{"title":"Dune"}`)
// Note: no authors, publish_year, publisher, cover_url - omitempty stripped them
```

**Unmarshaling (JSON to struct):**
```go
var payload olResponse
if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
    return nil, fmt.Errorf("open library decode: %w", err)
}
```

- `json.NewDecoder` streams from an `io.Reader` - efficient for HTTP response bodies
- `.Decode(&payload)` takes a **pointer** to the target - it needs write access
- Fields that don't match any JSON key are left at their zero values
- JSON keys with no matching struct field are silently ignored

**Gotcha: lowercase fields are invisible to `encoding/json`:**
- `json.Marshal` uses reflection, which can only see **exported** (uppercase) fields
- A field declared `title string` (lowercase) won't appear in JSON even if you give it a tag

That's why all the JSON-serializable fields in `olBook`, `OpenLibraryBook`, etc. are uppercase.

---

## Cookies

```go
c.SetSameSite(http.SameSiteStrictMode)
c.SetCookie("flash_success", code, 60, "/", "", secure, true)
```

**`c.SetCookie(name, value, maxAge, path, domain, secure, httpOnly)`:**

| Arg | Meaning | Example here |
|---|---|---|
| name | cookie name | `"flash_success"` |
| value | cookie value | `"staff_created"` (a slug, not English) |
| maxAge | seconds until expiry; negative deletes immediately | `60` |
| path | URL path where the cookie is sent | `"/"` (everywhere) |
| domain | which host - empty means "only this host" | `""` |
| secure | only send over HTTPS | `true` in production |
| httpOnly | hide from JavaScript | `true` |

**Why these flags matter:**
- `httpOnly=true` - JS can't read the cookie, so a successful XSS can't steal it
- `secure=true` - the cookie never travels over plain HTTP; blocks network-level theft
- `SameSite=Strict` - the cookie doesn't get sent on cross-site navigation; defends against CSRF

**Reading a cookie:**
```go
code, err := c.Cookie("flash_success")
if err != nil || code == "" {
    return ""
}
```
`c.Cookie` returns `(string, error)` - the error is `http.ErrNoCookie` when the cookie isn't set.

**Deleting a cookie** (one-shot flash pattern):
```go
c.SetCookie("flash_success", "", -1, "/", "", secure, true)
```
Negative `maxAge` tells the browser to delete the cookie immediately. Used in `readAndClearFlash` so a page refresh doesn't re-show the banner.

---

## Middleware Chains and Route Groups

```go
admin := router.Group("/")
admin.Use(RequireAuth(), RequireRole("admin"))
admin.GET("/staff", HandleStaffList)
admin.POST("/staff", HandleStaffCreate)
admin.POST("/staff/:id/delete", HandleStaffDelete)
```

**`router.Group("/")`**
- Creates a sub-router that inherits from `router`
- Anything registered on the group (middleware, routes) applies only to routes registered in that group
- The argument is a path prefix (empty in this case, since the routes already spell out `/staff`)

**`.Use(middleware...)`**
- Registers middleware that runs before every handler in the group
- Variadic - pass one or many
- Order matters: `RequireAuth` runs first, `RequireRole` second, then the handler

**How a middleware actually works:**
```go
func RequireAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        session, err := c.Cookie("session")
        if err != nil || session == "" {
            c.Redirect(http.StatusSeeOther, "/login")
            c.Abort()
            return
        }
        c.Set("user", user)
        c.Next()
    }
}
```

**The two flow-control calls:**
- `c.Next()` - run the next middleware / the final handler in the chain
- `c.Abort()` - stop the chain; remaining middleware and the final handler don't run
- `c.AbortWithStatus(404)` - abort AND set a status

**Why this matters:**
- Auth middleware checks the session; `c.Abort()` stops unauthorized requests before they reach the handler
- Role middleware does the same for admin-only routes
- Each handler just assumes "if I'm running, the user is authed and has the right role"
- **Route boundaries are enforced by group membership, not by `if` statements in every handler**

**This is a closure factory again.** `RequireAuth()` is a function that returns a function. The returned function is what Gin calls per request. The factory pattern lets you parameterize middleware: `RequireRole("admin")` vs `RequireRole("staff")`.

---

## Multipart Forms

```go
if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
    c.String(http.StatusBadRequest, "invalid form")
    return
}
title := c.Request.FormValue("title")
file, header, err := c.Request.FormFile("cover")
```

**When you need this:**
- An HTML form with `enctype="multipart/form-data"` (required for file uploads)
- The body is encoded as a series of parts separated by boundary markers, not as key=value pairs

**`ParseMultipartForm(maxMemory)`**
- Parses the body and populates `r.MultipartForm`
- `maxMemory` is the memory budget for parsing - anything beyond it spills to temporary files on disk
- It is **not** an upload size limit (that's enforced elsewhere, via `coverMaxBytes` in `covers.go`)

**Bit shifts - `32 << 20`:**
- `<<` is a left bit shift
- `32 << 20` = `32 * 2^20` = `32 * 1,048,576` = **32 MiB**
- Reads as "thirty-two mebibytes" once you recognize the idiom
- Stdlib and docs convention - you could equivalently write `32 * 1024 * 1024`

**Reading form values:**
```go
title := c.Request.FormValue("title")                // plain text field
file, header, err := c.Request.FormFile("cover")     // uploaded file
```

- `file` is an `io.ReadCloser` - read it with `io.Copy` or `io.ReadAll`
- `header` is a `*multipart.FileHeader` - gives `.Size`, `.Filename`, `.Header`
- Always `defer file.Close()` after the error check

---

## File I/O and Magic-Byte MIME Detection

```go
buf := make([]byte, 512)
n, _ := file.Read(buf)
mimeType := http.DetectContentType(buf[:n])

switch mimeType {
case "image/jpeg", "image/png", "image/webp":
    // OK
default:
    return "", ErrCoverBadMimeType
}

// Rewind so the full file can be copied to disk
if _, err := file.Seek(0, io.SeekStart); err != nil {
    return "", err
}
```

**Why not trust the uploaded filename's extension?**
- A malicious client can rename `evil.exe` to `cover.jpg` before uploading
- The `Content-Type` header the browser sends can also be lied about
- Only the actual file bytes can be trusted

**`http.DetectContentType(bytes)`:**
- Reads up to the first 512 bytes (the "magic bytes" - file format signatures)
- Returns a MIME type: `"image/jpeg"`, `"image/png"`, `"application/octet-stream"` for unknown, etc.
- Based on a hardcoded table of file-format signatures built into the standard library

**`file.Seek(0, io.SeekStart)`:**
- After reading 512 bytes, the file's read position has advanced
- `Seek` moves it back to byte 0 so subsequent code can copy the full file
- Second arg is a "whence": `io.SeekStart`, `io.SeekCurrent`, `io.SeekEnd`

**Copying the file to disk:**
```go
dst, err := os.Create(destPath)
if err != nil {
    return "", err
}
defer dst.Close()

if _, err := io.Copy(dst, file); err != nil {
    return "", err
}
```

- `os.Create(path)` creates/truncates a file, returns `*os.File`
- `io.Copy(dst, src)` streams bytes from a reader to a writer with a small internal buffer - **never** loads the entire file into memory
- Works for any reader-to-writer combo (HTTP response to file, file to response, etc.)

---

## Cryptographic Primitives

### Password Hashing - `bcrypt`

```go
hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
```

- `bcrypt.GenerateFromPassword` returns a salted hash
- `bcrypt.DefaultCost` (currently 10) controls how slow the hash is - slower = harder to brute-force
- Store the returned `hash` bytes as a string in the `password_hash` column

**Verifying a password:**
```go
err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(submittedPassword))
// err == nil means the password matched
```

- **Never** compare hashes with `==` or `strings.Equal`. Always use the library's constant-time comparison
- `err != nil` can mean "wrong password" OR "malformed hash" - treat both as a login failure, but log the second case

### Secure Random Bytes - `crypto/rand`

```go
b := make([]byte, 32)
if _, err := rand.Read(b); err != nil {
    return "", err
}
token := hex.EncodeToString(b)
```

- `crypto/rand.Read` uses the OS's cryptographically secure random source
- `math/rand` is **NOT** cryptographically secure - never use it for tokens, session IDs, passwords, or filenames of user-uploaded files
- `hex.EncodeToString(b)` turns bytes into a hex string (64 chars for 32 bytes) that's safe in URLs and cookies

Used here for session tokens, CSRF tokens, and random cover filenames.

### Constant-Time Comparison - `crypto/subtle`

```go
if subtle.ConstantTimeCompare([]byte(submitted), []byte(expected)) != 1 {
    c.AbortWithStatus(http.StatusForbidden)
    return
}
```

**Why not `submitted == expected`?**
- `==` on strings short-circuits on the first differing byte
- An attacker measuring response times can gradually learn the correct token byte by byte
- Called a **timing attack**

**`subtle.ConstantTimeCompare`:**
- Always walks the full length of both byte slices
- Returns 1 if equal, 0 otherwise (yes, integer - not `bool`)
- Takes the same time regardless of how early / late the difference is

Used for CSRF token verification, where the token value is a secret the attacker is trying to guess.

---

## Buffered Template Rendering - The CP4 Fix

```go
var buf bytes.Buffer
if err := templates["catalog"].ExecuteTemplate(&buf, "layout", data); err != nil {
    log.Printf("render: %v", err)
    c.String(http.StatusInternalServerError, "internal error")
    return
}
c.Writer.Write(buf.Bytes())
```

**Why not render directly to `c.Writer`?**

In the original naive pattern:
```go
templates["catalog"].ExecuteTemplate(c.Writer, "layout", data)
```
- As soon as `ExecuteTemplate` writes the first byte, Gin has already sent headers (status 200, content-type) to the browser
- If the template errors halfway through (e.g. nil pointer on a field), you've already committed to 200 with half a page
- The browser sees broken HTML + 200 OK and you can't take it back

**Buffer-first pattern:**
- Render into an in-memory `bytes.Buffer` first
- If rendering fails, nothing has been written to the response yet - return a clean 500
- If rendering succeeds, copy the complete buffer to the response in one write

**`bytes.Buffer`:**
- Implements `io.Writer` - templates, JSON encoders, anything that writes to a stream can write to a buffer
- `.Bytes()` returns the accumulated bytes
- Zero value is usable (`var buf bytes.Buffer` - no `make`, no `new`)

See DECISIONS.md for the security rationale. The issue was that error paths in templates could leak partial pages, including internal state before the error occurred.

---

## Regex - `regexp`

```go
var yearRegex = regexp.MustCompile(`\b(1[5-9]\d{2}|20\d{2}|2100)\b`)

func parsePublishYear(s string) int {
    match := yearRegex.FindString(s)
    if match == "" {
        return 0
    }
    n, _ := strconv.Atoi(match)
    return n
}
```

**`regexp.MustCompile`:**
- Compiles the pattern once at package init
- `Must` panics if the regex is malformed - same idiom as `template.Must`
- Assigning the compiled regex to a package-level variable means it's compiled exactly once for the program's lifetime. Using `regexp.Compile` inside a function would recompile on every call

**Raw string literals with backticks:**
- `` `\b(1[5-9]\d{2}|20\d{2}|2100)\b` ``
- No escape processing - you write `\b` and `\d` literally
- Use backticks for any regex. Reserve `"..."` for non-regex strings

**Matching methods:**
- `.FindString(s)` - first match, as a string
- `.FindAllString(s, n)` - first n matches (or -1 for all)
- `.MatchString(s)` - just a boolean
- `.ReplaceAllString(s, repl)` - find and replace
- `.FindStringSubmatch(s)` - returns the full match plus captures

---

## Maps as Dispatch / Lookup Tables

```go
var flashMessages = map[string]string{
    "password_mismatch":        "Password and confirmation did not match.",
    "weak_password":            "Password does not meet complexity requirements.",
    "staff_created":            "Staff account created.",
    "cannot_delete_last_admin": "At least one admin account must remain.",
    "book_created":             "Added to the catalog:",
    "patron_created":           "Patron added:",
    // ... and so on
}

msg, ok := flashMessages[code]
if !ok {
    log.Printf("flash: unknown code %q for kind %q", code, kind)
    return ""
}
return msg
```

**The two-value map lookup:**
- `value, ok := m[key]` - `ok` is `true` if the key exists
- If the key is missing, `value` is the zero value of the map's value type (empty string for a `map[string]string`)
- Use the two-value form whenever you need to distinguish "missing" from "present but zero"

**Why this pattern is powerful:**
- Adding a new flash message is a single line in one file
- No switch statement to maintain
- The compiler won't check exhaustiveness (no enum-style coverage), but the `log.Printf` on missing lookup turns typos into server-log noise instead of silent empty banners

**Other good uses of this pattern:**
- HTTP method to handler dispatch
- File extension to MIME type lookup
- Error code to HTTP status code
- Any 1:1 lookup table where the key set grows over time

---

## Table-Driven Tests and Subtests

```go
func TestGenerateBaseUsername(t *testing.T) {
    cases := []struct {
        name string
        in   string
        want string
    }{
        {"basic two words", "John Smith", "jsmith"},
        {"single name lowercased", "Cher", "cher"},
        {"strips outer whitespace", "  Jane  Doe  ", "jdoe"},
        {"strips non-alphanumerics", "John O'Brien", "jobrien"},
        {"empty input returns empty", "", ""},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := generateBaseUsername(tc.in)
            if got != tc.want {
                t.Errorf("got %q, want %q", got, tc.want)
            }
        })
    }
}
```

**Breaking this down:**

**`cases := []struct { ... }{ ... }`**
- A slice of anonymous struct values
- Each element describes one input/output pair
- The field names (`name`, `in`, `want`) are only visible inside this test

**`t.Run(name, func(t *testing.T) { ... })`**
- Creates a **subtest** - a named child of the outer test
- Each subtest has its own `*testing.T`
- Failures in one subtest don't stop the others from running

**Why this beats one function per case:**
- One test function per function-under-test keeps the file organized
- Adding a case is one line, not a new `func TestX_SomeCase`
- `go test -v` prints each subtest's name so failures point exactly at which case broke
- You can target a single subtest: `go test -run TestGenerateBaseUsername/single_name`

**`t.Errorf` vs `t.Fatalf`:**
- `Errorf` - record the failure, continue the test
- `Fatalf` - record the failure, stop this test/subtest immediately
- Use `Fatalf` when later assertions would panic or produce misleading noise after this one fails (e.g. you can't unmarshal JSON, so a later field-equality check would just echo "field was empty")

---

## `httptest` - In-Memory HTTP Testing

```go
func TestHandleBookCreate(t *testing.T) {
    router, dm := setupTestRouter(t)
    sess, csrf := loginAs(t, dm, "admin", "admin")

    body := url.Values{}
    body.Set("title", "New Book")
    body.Set("isbn", "9781234567890")
    body.Set("year", "2024")
    body.Set("quantity_total", "1")
    body.Set("csrf_token", csrf)

    req := httptest.NewRequest(http.MethodPost, "/books",
        strings.NewReader(body.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.AddCookie(sess)

    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    if w.Code != http.StatusSeeOther {
        t.Fatalf("status = %d, want 303", w.Code)
    }
}
```

**Breaking this down:**

**`httptest.NewRequest(method, url, body)`**
- Builds an `*http.Request` suitable for testing
- No network involved - it's an in-memory request
- The `body` is any `io.Reader`. `strings.NewReader(s)` wraps a string

**`httptest.NewRecorder()`**
- Returns a `*httptest.ResponseRecorder` - an in-memory `http.ResponseWriter`
- Captures status, headers, and body:
  - `w.Code` - the HTTP status
  - `w.Body.String()` - the response body
  - `w.Result().Header.Get("Location")` - any header

**`router.ServeHTTP(w, req)`**
- Runs the request through the router as if it were a real HTTP call
- Middleware, handlers, everything fires in the normal order

**Why in-memory beats spinning up a real server:**
- No ports to allocate, no goroutines to wait on, no networking quirks
- Each test is a self-contained function call
- Parallel tests don't fight over ports
- Predictable, fast, no flakiness

**When you DO want a real server:**
```go
srv := httptest.NewServer(router)
defer srv.Close()
resp, err := http.Get(srv.URL + "/books")
```
Used for integration tests that need to exercise real HTTP behavior: redirect following, cookie jars, keep-alives.

**Helpers in this project:**
- `setupTestRouter(t)` - builds a router with the production middleware chain, plus a fresh temp-DB
- `loginAs(t, dm, username, role)` - creates a user, hits the login endpoint, returns `(*http.Cookie, csrfToken)`
- `logoutHelper(...)` - hits the logout endpoint

Those live in `main_test.go`. Use them so every test gets the same middleware chain the real router does.

---

## HTTP Client with Timeout

```go
client := &http.Client{Timeout: 10 * time.Second}
resp, err := client.Do(req)
if err != nil {
    return nil, fmt.Errorf("open library request: %w", err)
}
defer resp.Body.Close()
```

**Never use `http.Get` or `http.DefaultClient` for calls to external services:**
- `http.DefaultClient` has **no timeout** - a slow or hung server wedges your handler forever
- A custom `&http.Client{Timeout: ...}` bounds every request

**What the `Timeout` field covers:**
- DNS lookup + TCP connect + TLS handshake + writing the request + reading the **full response body**
- Not a per-phase timeout - the entire round trip must finish within it
- If you want finer control, construct a `*http.Transport` with separate dial/TLS/response-header timeouts

**Always close the response body:**
```go
resp, err := client.Do(req)
if err != nil {
    return err
    // Don't defer Close here - resp is nil when err != nil
}
defer resp.Body.Close()
```

- `resp.Body` is an `io.ReadCloser`
- Forgetting to close leaks the underlying TCP connection back to the pool
- The pattern is always "check err, return early on err, THEN defer Close"

---

## Syntax Summary (Part 2)

| Concept | Syntax | Example in this project |
|---|---|---|
| Defer | `defer fn()` | `defer tx.Rollback()` |
| Begin transaction | `tx, err := db.Begin()` | `db.go:602` |
| Commit/Rollback | `tx.Commit()` / `tx.Rollback()` | `db.go:646` |
| Sentinel error | `var ErrX = errors.New("...")` | `ErrBookHasLoans` in `db.go` |
| Error check (wrapper-aware) | `errors.Is(err, Target)` | `handlers_books.go:528` |
| Error wrap | `fmt.Errorf("ctx: %w", err)` | `openlibrary.go:117` |
| Variadic param | `args ...T` | `log.Printf(format, args...)` |
| Spread slice to variadic | `fn(slice...)` | `router.Use(middleware...)` |
| Context timeout | `ctx, cancel := context.WithTimeout(parent, dur)` | `main.go:41` |
| Pass context to HTTP | `http.NewRequestWithContext(ctx, ...)` | `openlibrary.go:102` |
| Struct tag | `` `key:"value,option"` `` | `` `json:"title,omitempty"` `` |
| JSON decode | `json.NewDecoder(r).Decode(&v)` | `openlibrary.go:126` |
| JSON encode | `json.Marshal(v)` | used in the OL proxy |
| Set cookie | `c.SetCookie(name, val, age, path, domain, secure, httpOnly)` | `flash.go:71` |
| Read cookie | `val, err := c.Cookie(name)` | `flash.go:84` |
| Same-site mode | `c.SetSameSite(http.SameSiteStrictMode)` | `flash.go:70` |
| Route group | `g := router.Group("/")` | `main.go` |
| Group middleware | `g.Use(mw1, mw2)` | `main.go` |
| Middleware continue | `c.Next()` | auth middleware |
| Middleware abort | `c.Abort()` | auth middleware |
| Parse multipart | `c.Request.ParseMultipartForm(32 << 20)` | `handlers_books.go:144` |
| Bit shift | `n << k` | `32 << 20` = 32 MiB |
| Get form field | `c.Request.FormValue("title")` | `handlers_books.go` |
| Get uploaded file | `c.Request.FormFile("cover")` | `handlers_books.go` |
| MIME sniff | `http.DetectContentType(bytes)` | `covers.go` |
| File seek | `f.Seek(0, io.SeekStart)` | `covers.go` |
| Copy stream | `io.Copy(dst, src)` | `covers.go` |
| bcrypt hash | `bcrypt.GenerateFromPassword(pw, cost)` | staff and patron create |
| bcrypt verify | `bcrypt.CompareHashAndPassword(hash, pw)` | login handler |
| Secure random | `rand.Read(b)` from `crypto/rand` | session tokens |
| Constant-time compare | `subtle.ConstantTimeCompare(a, b) == 1` | CSRF verification |
| Buffered render | `buf := bytes.Buffer{}` + `ExecuteTemplate(&buf, ...)` + `Write(buf.Bytes())` | every handler |
| Compile regex once | `var r = regexp.MustCompile(...)` | `openlibrary.go:55` |
| Regex first match | `r.FindString(s)` | `openlibrary.go:58` |
| Map two-value lookup | `v, ok := m[k]` | `flash.go:91` |
| Subtest | `t.Run(name, func(t *testing.T){ ... })` | `validators_test.go` |
| HTTP test request | `httptest.NewRequest(method, url, body)` | all `*_test.go` |
| HTTP test response | `httptest.NewRecorder()` | all `*_test.go` |
| HTTP client timeout | `&http.Client{Timeout: dur}` | `openlibrary.go:114`, `covers.go` |

---

## Complete File Reference (Expanded)

| File | Key Concepts Introduced |
|---|---|
| `main.go` | Package, imports, `var`, `func main`, router setup, range loop, env vars, route groups, middleware registration, `context.WithTimeout` |
| `db.go` | Struct, constructor, methods, error handling, underscore import, transactions (`Begin`/`Commit`/`Rollback`), `defer`, sentinel errors, nested `tx.Exec`, prepared queries with `?` placeholders, `sql.ErrNoRows` |
| `handlers.go` | Functions, closures, middleware, type assertions, URL params, buffered template rendering |
| `handlers_auth.go` | Login/logout, bcrypt password verify, session cookie issuance, CSRF token generation and verify with `crypto/subtle`, constant-time login check |
| `handlers_staff.go` | Admin-only CRUD, sentinel-error handling, last-admin guard, flash-cookie messaging, role-based authorization |
| `handlers_books.go` | Book CRUD, multipart form parsing (`ParseMultipartForm`), file uploads, OL API proxy, cover routing logic, `ErrBookHasLoans` guard |
| `handlers_patrons.go` | Patron CRUD, transactional create, `generateBaseUsername` + collision-retry loop, `ErrPatronHasLoans` guard |
| `openlibrary.go` | HTTP client with timeout, JSON decode with struct tags, `http.NewRequestWithContext`, regex-based field extraction, sentinel error (`ErrOpenLibraryNotFound`), error wrapping |
| `covers.go` | File I/O (`os.Create`, `io.Copy`), magic-byte MIME sniffing (`http.DetectContentType`), `file.Seek`, crypto random filename (`crypto/rand`), byte-size limits, sentinel errors |
| `flash.go` | Cookie read/write, `SameSite=Strict`, `HttpOnly`, `Secure`, map-as-dispatch-table, code-slug-not-text pattern, companion detail cookie |
| `validators.go` | Password policy, ISBN validation, `generateBaseUsername` string manipulation, `strings.Fields`, `strings.Builder`, rune iteration |
| `main_test.go` | `setupTestRouter` helper, `loginAs`, `logoutHelper`, route-boundary tests, `t.Helper`, `t.Cleanup` |
| `db_test.go` | DB method tests, transactional-write verification |
| `handlers_*_test.go` | Table-driven HTTP handler tests, `httptest.NewRecorder`, cookie + CSRF plumbing in tests |
| `validators_test.go` | Pure-function tests, extensive subtest matrix via `t.Run` |

---

## Concept Index - Where to Find Each Pattern

| Want to see... | Go to |
|---|---|
| `defer` + transactions | `db.go:602` (`seedOneBook`), `db.go:1770` (`CreatePatron`) |
| Sentinel error + `errors.Is` | `db.go` (declaration) + `handlers_books.go:528` (check) |
| Error wrapping with `%w` | `openlibrary.go:117` |
| Context with timeout | `main.go:41` + `openlibrary.go:102` |
| JSON struct tags + decoding | `openlibrary.go:23-53` + `openlibrary.go:126` |
| Cookie security flags | `flash.go:64-72` |
| Middleware factory pattern | `handlers_auth.go` (auth / role middleware) |
| Route groups with role-based `.Use` | `main.go` route registration block |
| Multipart form parsing | `handlers_books.go:144` |
| Magic-byte MIME check | `covers.go` |
| `bcrypt` hash + verify | `handlers_staff.go` (hash) + `handlers_auth.go` (verify) |
| Constant-time compare | CSRF middleware in `handlers_auth.go` |
| Buffered template rendering | any handler's render block (e.g. `handlers_books.go`) |
| Compiled regex | `openlibrary.go:55` |
| Map-as-dispatch-table | `flash.go:24` |
| Table-driven subtests | `validators_test.go` (many examples) |
| `httptest.NewRecorder` usage | every `*_test.go` file |
| HTTP client with timeout | `openlibrary.go:114`, `covers.go` |

---

## Key Go Concepts (Part 2)

1. **Resource management is explicit.** No garbage-collected file handles or DB transactions. `defer` is your cleanup guarantee.
2. **Errors are values, not exceptions.** `err != nil` is the universal control flow. `errors.Is`, `errors.As`, and `%w` give you inspection and context without losing identity.
3. **Context is how you cancel.** Any function that does I/O or calls out to the network should take a `context.Context` as its first argument.
4. **The standard library is enough.** `encoding/json`, `net/http`, `html/template`, `database/sql`, `crypto/*`, `regexp`, `httptest` - you've used all of them without a third-party replacement.
5. **Small interfaces, composition over inheritance.** `io.Reader`, `io.Writer`, `io.Closer` show up everywhere. Templates write to writers, HTTP responses are writers, buffers are writers. One interface, many implementations.
6. **Security is a design choice at every layer.** Parameterized SQL queries, magic-byte MIME detection, constant-time comparison, `SameSite=Strict` cookies, buffered template rendering, role-gated route groups - each is a specific defense against a specific class of attack.
7. **Test with the real router.** `httptest` + `setupTestRouter` + `loginAs` lets you exercise the exact middleware chain production uses. No mocks, no separate "test middleware."

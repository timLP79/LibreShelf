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

---

# Part 3 - Production Go: Concepts from CP6 and CP7

Part 2 took you from a single `main.go` to a real test suite. Part 3 covers the concepts that landed when the project grew loans, a public kiosk, role-differentiated dashboards, an admin backup/import flow, and a security-hardened HTTP surface. Each section is a real snippet from the project, broken down line by line.

---

## `time.Time` and Date Arithmetic

```go
dueDate := time.Now().AddDate(0, 0, DefaultLoanTermDays)
err = dm.CheckoutBook(bookID, patronID, dueDate)
```

**`time.Now()`**
- Returns the current local time as a `time.Time` value
- `time.Time` is a struct, not an integer or string - it carries date, time-of-day, and location

**`.AddDate(years, months, days)`**
- Returns a new `time.Time` shifted by the given amounts
- All three are signed - `AddDate(0, 0, -7)` rewinds a week
- Handles month/year wrap-around correctly (March 31 + 1 month = April 30 or May 1 depending on calendar logic - check the docs)

**`time.Duration` for shorter offsets:**
```go
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
```
- `time.Second`, `time.Minute`, `time.Hour` are `time.Duration` constants
- Multiply: `60 * time.Second`, `5 * time.Minute`
- Use `AddDate` for day/month/year offsets, `time.Duration` arithmetic for sub-day offsets

**Format and parse:**
```go
t.Format("2006-01-02")               // "2026-05-02"
time.Parse("2006-01-02", "2026-05-02")
```
- Go's date layout is the **reference time** `Mon Jan 2 15:04:05 MST 2006` - rearrange those tokens to describe your format
- `2006` = year, `01` = month, `02` = day, `15` = hour, `04` = min, `05` = sec
- Counterintuitive at first; you'll memorize it

**In this project:** `time.Now().Format("2006-01-02 15:04:05")` is used to compare a SQLite-stored `due_date` string against the current time in `db.go:296`. SQLite has no native datetime type - it stores dates as strings - so all comparisons cross the Go/SQL boundary as text.

---

## Derived State - The DEC-024 Pattern

The `loans` table has no `status` column. Instead:

```sql
-- active:    returned_at IS NULL AND due_date >= DATE('now')
-- overdue:   returned_at IS NULL AND due_date < DATE('now')
-- returned:  returned_at IS NOT NULL
```

**Why no status column?**
- A status column **denormalizes** the data: it duplicates information already implicit in `due_date + returned_at`
- Denormalization is a source of drift: every state transition has to update both the source columns AND the status, in lockstep, in every code path. Forgetting one path leaves the row in an inconsistent state
- Deriving the state in queries means the derivation is the single source of truth

**Trade-off:** a status column lets you `WHERE status = 'overdue'` instead of writing the predicate. The savings is one short SQL clause per query. The cost is a class of bug that won't show up in any test until production data is messy.

**Rule of thumb:** if a piece of state can be derived deterministically from other columns, derive it. Only store it separately if (a) the derivation is expensive and you've measured a real perf problem, or (b) you need to capture point-in-time history that the derivation can't reconstruct.

**Same pattern shows up in the Go layer.** The dashboard template doesn't get a "status" string per loan - it gets `MyLoanCount`, `OverdueLoans`, and `NextDueDate`, each computed by a dedicated `Count*` query. No "loan status enum" type ever exists in the Go code either.

See `DECISIONS.md` DEC-024 for the full rationale.

---

## Sentinel-Error Fan-Out with `switch err`

```go
err = dm.CheckoutBook(bookID, patronID, dueDate)
switch err {
case nil:
    setFlash(c, flashKindSuccess, "loan_checkout_success")
    setFlashDetail(c, book.Title)
case ErrPatronHasOverdue:
    setFlash(c, flashKindError, "loan_blocked_overdue")
case ErrPatronAtLoanLimit:
    setFlash(c, flashKindError, "loan_blocked_limit")
case ErrNoCopiesAvailable:
    setFlash(c, flashKindError, "loan_no_copies")
default:
    log.Printf("HandleCheckout: CheckoutBook(book=%d, patron=%d): %v", bookID, patronID, err)
    c.String(http.StatusInternalServerError, "Internal Server Error")
    return
}
```

**Why `switch err` instead of stacked `if errors.Is`:**
- Reads top-to-bottom as a complete decision tree: success path, three known business errors, unknown error
- `default` is the explicit "I haven't accounted for this" branch - logs and returns 500
- `case nil` puts the success path in the same shape as the error paths
- One DB call, one fan-out block, one redirect at the bottom

**When to use `switch err` vs `errors.Is`:**
- Plain sentinels you expect to compare directly: `switch err`
- Errors that might be wrapped with `%w` somewhere up the call chain: `if errors.Is(err, Target)` because `switch` does a literal `==` check that doesn't walk wrapper chains
- Mixed: handle the wrapped sentinel with `errors.Is` before the `switch`, then fan out the rest

**The flash codes are slugs, not English.** `"loan_blocked_overdue"` is a stable identifier; the user-facing text lives in the `flashMessages` map (Part 2: "Maps as Dispatch / Lookup Tables"). Keeps copy editing out of the handler.

---

## Sub-Packages and the `internal/` Convention

```
internal/safezip/
  doc.go
  extract.go
  extract_test.go
```

**Where the project's package layout grew:** all earlier code lives in `package main` at the root. `internal/safezip` is the first sub-package: a focused chunk of logic (safe ZIP extraction) with its own tests, importable from `main` as `"github.com/timLP79/cs408-go-stack/internal/safezip"`.

**Why pull it out of `package main`?**
- Reusable: any handler that needs to extract a ZIP can call `safezip.SafeExtract`
- Testable in isolation: `extract_test.go` tests the package without standing up Gin or a DB
- Boundary: the package exposes a small API (`SafeExtract`, `SafeExtractWithLimits`, the four sentinel errors) and hides everything else

**The `internal/` directory is a special compiler rule:**
- A package under `internal/` can only be imported by packages **rooted at the same parent**
- `internal/safezip` is importable by `main` (same root: `github.com/timLP79/cs408-go-stack/`)
- It would NOT be importable by some unrelated repo even if they `go get`'d it
- Use this for "library code I want clean boundaries on, but not a public API" - which is exactly what `safezip` is

**The `doc.go` convention:**
```go
// Package safezip provides ZIP extraction with protection against
// path-traversal attacks (Zip Slip), absolute-path entries, symlinks,
// and zip-bomb attacks.
// ...
package safezip
```
- A file named `doc.go` whose only contents are the package comment
- `go doc ./internal/safezip` and `pkg.go.dev` both surface this comment
- Keeps the package overview in one obvious place instead of being attached to whichever file happens to come first alphabetically

---

## Two-Pass Validation - Fail Closed Before Touching Disk

```go
func SafeExtractWithLimits(zipPath, destDir string, limits Limits) error {
    r, err := zip.OpenReader(zipPath)
    if err != nil {
        return err
    }
    defer r.Close()

    absDest, err := filepath.Abs(destDir)
    if err != nil {
        return err
    }

    var totalSize int64
    for _, f := range r.File {
        if err := validateEntry(f, absDest, limits); err != nil {
            return err
        }
        if !f.FileInfo().IsDir() {
            totalSize += int64(f.UncompressedSize64)
            if limits.MaxTotalSize > 0 && totalSize > limits.MaxTotalSize {
                return fmt.Errorf("%w: archive total exceeds limit", ErrTooLarge)
            }
        }
    }

    for _, f := range r.File {
        if err := extractEntry(f, absDest); err != nil {
            return err
        }
    }
    return nil
}
```

**The two-pass shape:**
1. **First loop:** validate every entry. Read no bytes from disk yet. If any entry is bad, return early - nothing has been written.
2. **Second loop:** extract every entry. By this point all entries are known good.

**Why not validate-and-extract in one loop?**
- A malicious archive could have 99 safe entries followed by one Zip Slip entry. A one-pass extractor would write the 99 files before discovering the bad one. Cleanup is then required, error-prone, and the partial state is an attack surface.
- Two-pass means the disk only changes if every entry passes. **Atomic from the caller's perspective.**

**Total-size check inside the validation loop:**
- Each entry's claimed `UncompressedSize64` is summed
- If the running total exceeds `MaxTotalSize`, abort
- This is **the declared size** - `extractEntry` enforces the actual size with `io.LimitReader` (next section)

**`zip.OpenReader(path)`:**
- Returns `*zip.ReadCloser` which has a `.File []*zip.File` field
- Each `*zip.File` is a header (name, mode, declared size) plus a way to open the entry's bytes
- `r.Close()` closes the underlying file handle - always defer it

**`filepath.Abs(destDir)`:**
- Resolves relative paths against the current working directory
- The Zip Slip check (next section) needs an absolute destination to compare against

---

## Zip Slip Defense - Path Containment

```go
func validateEntry(f *zip.File, absDest string, limits Limits) error {
    if f.Mode()&os.ModeSymlink != 0 {
        return fmt.Errorf("%w: %q", ErrSymlink, f.Name)
    }
    if strings.Contains(f.Name, "\\") {
        return fmt.Errorf("%w: %q (backslash not permitted)", ErrZipSlip, f.Name)
    }
    if filepath.IsAbs(f.Name) {
        return fmt.Errorf("%w: %q", ErrAbsolutePath, f.Name)
    }
    target := filepath.Join(absDest, f.Name)
    rel, err := filepath.Rel(absDest, target)
    if err != nil {
        return fmt.Errorf("%w: %q", ErrZipSlip, f.Name)
    }
    if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
        return fmt.Errorf("%w: %q", ErrZipSlip, f.Name)
    }
    if !f.FileInfo().IsDir() && limits.MaxFileSize > 0 {
        if int64(f.UncompressedSize64) > limits.MaxFileSize {
            return fmt.Errorf("%w: %q", ErrTooLarge, f.Name)
        }
    }
    return nil
}
```

**The four classes of attack this defends against:**

**1. Symlinks** - `f.Mode()&os.ModeSymlink != 0`
- A ZIP entry can declare itself a symlink. Extract one and you have a file on disk that points at `/etc/passwd` or anywhere else
- `os.ModeSymlink` is a bit in `os.FileMode`. The `&` mask isolates it
- Reject the entry outright - this project has no use case for symlinks in backups

**2. Backslashes** - `strings.Contains(f.Name, "\\")`
- ZIP entry names should use `/` even on Windows-produced archives (the spec)
- A backslash in the name is a sign of a broken or malicious tool. Reject it

**3. Absolute paths** - `filepath.IsAbs(f.Name)`
- An entry named `/etc/passwd` would extract to `/etc/passwd`, not under `destDir`
- `filepath.IsAbs` checks for a leading `/` (or drive letter on Windows)
- Reject any absolute path

**4. Path traversal (Zip Slip)** - the `filepath.Rel` check
- An entry named `../../etc/passwd` joins with `destDir` to escape the destination
- `filepath.Join` normalizes `..` segments, so `Join("/safe", "../etc")` = `/etc`
- `filepath.Rel(absDest, target)` returns the path of `target` relative to `absDest`
- If the relative path starts with `..` (or IS `..`), the entry escaped the destination
- The `+ string(filepath.Separator)` is to avoid false positives like `..foo` matching `..` as a prefix

**This is a "deny escape" pattern, not "allow safe":**
- We don't try to enumerate the safe characters (impossible)
- We compute where the entry WOULD land and reject it if that location is outside our destination
- Containment by construction, not by filtering input

**Tested:** `internal/safezip/extract_test.go` includes a Zip Slip rejection test - constructs a malicious archive in memory and asserts `SafeExtract` returns `ErrZipSlip`.

---

## `io.LimitReader` - Defense Against Zip Bombs

```go
limited := io.LimitReader(rc, int64(f.UncompressedSize64)+1)
n, err := io.Copy(out, limited)
if err != nil {
    return err
}
if n > int64(f.UncompressedSize64) {
    return fmt.Errorf("%w: %q (zip bomb)", ErrTooLarge, f.Name)
}
```

**What a zip bomb is:**
- A ZIP entry whose declared `UncompressedSize64` is small but whose actual decompressed bytes are massive
- Or an entry whose decompression doesn't terminate (e.g., a malformed deflate stream)
- Naive extractor: `io.Copy(out, rc)` happily writes gigabytes to disk

**`io.LimitReader(r, n)`:**
- Wraps a reader so it returns at most `n` bytes, then EOF
- Caps how many bytes `io.Copy` can possibly transfer

**The `+1` trick:**
- Limit reads to `declared + 1` bytes
- If the actual count `n` exceeds `declared`, the entry was lying about its size
- The extra byte is a sentinel: it's there *only* to detect the lie

**Defense in depth:**
- Go 1.21+ `archive/zip` already enforces the declared size during decompression
- This check is belt-and-braces - if a future Go version regresses, or you swap in a different decompressor, the cap still holds
- The package doc (`internal/safezip/doc.go`) flags this explicitly

**`io.Copy(dst, src)`:**
- Streams bytes from reader to writer using a small internal buffer (32KB by default)
- Returns `(bytesWritten int64, err error)`
- Never loads the whole file into memory - works on a 100GB file the same as a 1KB file

---

## `sync.RWMutex` - Many Readers, One Writer

```go
type DatabaseManager struct {
    mu     sync.RWMutex
    db     *sql.DB
    dbPath string
}
```

**`sync.Mutex` vs `sync.RWMutex`:**
- `sync.Mutex` - exclusive: one goroutine in the critical section at a time
- `sync.RWMutex` - read/write: many readers can hold the lock at once, but writers exclude everyone (readers AND other writers)

**Use `RWMutex` when:**
- Read traffic vastly outnumbers write traffic
- The critical section is non-trivial (otherwise `Mutex` is simpler and faster - reader/writer bookkeeping has overhead)

**This project's pattern:**
- Every request that touches the DB takes `dm.mu.RLock()` via `DBReadLock` middleware - many requests run concurrently
- The backup-import handler takes `dm.mu.Lock()` directly - blocks until every active reader has released, then has exclusive access to swap the underlying `*sql.DB`

**The middleware that takes the read lock:**
```go
func DBReadLock(c *gin.Context) {
    dm := getDB(c)
    dm.mu.RLock()
    defer dm.mu.RUnlock()
    c.Next()
}
```
- `RLock()` increments a reader counter (or blocks if a writer is queued)
- `defer dm.mu.RUnlock()` runs after the handler chain completes
- `c.Next()` is the call that actually runs the rest of the chain - the lock is held across the whole request

**The handler that takes the write lock:**
```go
dm.mu.Lock()
defer dm.mu.Unlock()
// close old DB, swap files, open new DB, restore sessions
```
- `Lock()` blocks until every reader has released and no other writer is active
- The whole DB swap runs under exclusive access; no concurrent reader sees the half-swapped state

---

## The Read-Lock Cannot Upgrade Gotcha

```go
// Admin write routes -- swap the DB out from under everyone else.
// No DBReadLock; the import handler takes dm.mu.Lock() directly,
// since Go's sync.RWMutex cannot upgrade a read lock to a write lock.
adminWrite := router.Group("/")
adminWrite.Use(RequireAuth, RequireAdmin, CSRFProtect)
adminWrite.POST("/admin/backup/import", HandleBackupImport)
```

**The bug if you forget this:**
- Add `DBReadLock` to the import group "for consistency"
- `DBReadLock` takes `RLock`. Inside the handler, you call `dm.mu.Lock()` to upgrade
- Go's `sync.RWMutex` does NOT support upgrading. The `Lock()` call deadlocks: it waits for all readers (including the current goroutine) to release, but the current goroutine is blocked waiting for `Lock()`
- The handler hangs forever, every subsequent request piles up behind it, the server is wedged

**Why Go doesn't support upgrade:**
- Two goroutines both holding read locks both calling `Lock()` would deadlock with each other
- Detecting this safely requires explicit ordering - which is more complexity than just "have separate paths for readers and writers"

**The fix this project uses:**
- A separate route group (`adminWrite`) without `DBReadLock`
- The import handler takes `dm.mu.Lock()` itself - no read lock involved, no upgrade attempted
- The comment in `main.go:174-176` is load-bearing - it warns the next person who tries to "tidy up" the route registration

**General rule:** if a handler needs to write and any middleware in its group takes a read lock, split it into a different group. Don't try to upgrade.

---

## `os.Rename` - Atomic File Swap with Rollback

```go
if err := os.Rename(dbPath, dbBakPath); err != nil {
    if !os.IsNotExist(err) {
        log.Printf("HandleBackupImport: rename db -> bak: %v", err)
        recover()
        c.String(http.StatusInternalServerError, "Internal Server Error")
        return
    }
} else {
    undo = append(undo, func() { os.Rename(dbBakPath, dbPath) })
}

if err := os.Rename(snapshotPath, dbPath); err != nil {
    log.Printf("HandleBackupImport: install new db: %v", err)
    recover()
    c.String(http.StatusInternalServerError, "Internal Server Error")
    return
}
undo = append(undo, func() { os.Remove(dbPath) })
```

**`os.Rename(old, new)`:**
- On the same filesystem, this is an atomic operation - either the rename happens completely or not at all
- POSIX guarantees no in-between state visible to other processes
- Across filesystems, it falls back to copy+delete which is NOT atomic - that's why temp files for the snapshot live in the same directory as the target

**The rollback stack pattern:**
```go
var undo []func()
rollback := func() {
    for i := len(undo) - 1; i >= 0; i-- {
        undo[i]()
    }
}
```
- Each successful step appends a closure that undoes that step
- On error, run all queued undos in **reverse** order (LIFO)
- This is hand-rolled `defer` - regular `defer` runs unconditionally, but rollback should only run on the error path

**Why LIFO matters:**
- Step 1: rename old DB -> `db.bak`. Undo: rename it back.
- Step 2: rename old covers -> `covers.bak`. Undo: rename them back.
- Step 3: rename new DB into place. Undo: delete the new DB.
- If step 3 fails, rollback runs: delete new DB, restore covers, restore old DB. The reverse order matters because step 3 created a file at `dbPath` and the step-1 undo (restore from `db.bak`) would fail if `dbPath` were still occupied.

**The `os.IsNotExist(err)` branch:**
- The first `Rename` is for the old DB - but on a fresh install, there might not be an old DB yet
- `os.IsNotExist` returns true only for "file not found" errors. Other errors (permission denied, disk full) still abort
- If the old DB doesn't exist, skip the undo registration - there's nothing to restore

**Combined with `dm.mu.Lock()`:** the swap is atomic from the application's perspective (no concurrent reader sees a half-swap) AND atomic from the filesystem's perspective (each individual rename is all-or-nothing). The two layers stack.

---

## `VACUUM INTO` - Live Database Snapshot

```go
func (dm *DatabaseManager) SnapshotTo(destPath string) error {
    escaped := strings.ReplaceAll(destPath, "'", "''")
    _, err := dm.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", escaped))
    return err
}
```

**What `VACUUM INTO` does (SQLite):**
- Writes a fresh, defragmented copy of the entire database to `destPath`
- The copy is a consistent point-in-time snapshot - it's atomic with respect to other writes
- The destination must NOT already exist; SQLite refuses to overwrite

**Why not `cp data/database.sqlite backup.sqlite`?**
- A plain file copy mid-write captures the database in an inconsistent state (open transactions, dirty pages, WAL contents not yet checkpointed)
- The resulting file may not even open
- `VACUUM INTO` is the supported way to take a hot backup

**Why the manual escaping?**
```go
escaped := strings.ReplaceAll(destPath, "'", "''")
_, err := dm.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", escaped))
```
- Normally you'd use `?` placeholders: `dm.db.Exec("INSERT ... VALUES (?)", value)`
- But SQLite does NOT accept parameter binding for the destination of `VACUUM INTO` - it must be a literal string in the SQL
- So the path has to be inlined. `'` is escaped to `''` (SQL string-literal convention)
- **This is safe ONLY because `destPath` comes from `os.MkdirTemp`, never from user input**
- The doc comment on `SnapshotTo` makes this constraint explicit: "Callers should construct destPath from a process-controlled source"

**Whenever you have to inline a string into SQL, write down WHY. The next person reading the code needs to know it's not a SQL-injection bug waiting to happen.**

---

## Single-Function Middleware vs Closure Factory

```go
// Single-function middleware - no per-instance config
func SecurityHeaders(c *gin.Context) {
    h := c.Writer.Header()
    h.Set("X-Content-Type-Options", "nosniff")
    h.Set("X-Frame-Options", "DENY")
    h.Set("Referrer-Policy", "same-origin")
    h.Set("Content-Security-Policy", "...")
    if os.Getenv("APP_ENV") == "production" {
        h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
    }
    c.Next()
}

// Closure-factory middleware - takes config
func DatabaseMiddleware(dm *DatabaseManager) gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Set("db", dm)
        c.Next()
    }
}
```

**`SecurityHeaders`** is registered as `router.Use(SecurityHeaders)` - the function itself satisfies `gin.HandlerFunc`. No factory needed because there's nothing to parameterize.

**`DatabaseMiddleware(dm)`** is registered as `router.Use(DatabaseMiddleware(dm))` - calling the factory with the `dm` returns the actual middleware. The factory pattern is necessary because the inner function needs to close over `dm`.

**The signature `gin.HandlerFunc`:**
```go
type HandlerFunc func(*gin.Context)
```
- Any function matching that shape IS a `HandlerFunc` - no interface assertion needed
- This is structural typing for functions

**When to choose which:**
- **No config needed** - bare function (`SecurityHeaders`, `DBReadLock`)
- **Per-route config** - factory (`RequireRole("admin")`, `DatabaseMiddleware(dm)`)
- **Per-request behavior change based on state outside the call** - factory that captures the state

---

## Defensive HTTP Headers - The CP7 Hardening

```go
func SecurityHeaders(c *gin.Context) {
    h := c.Writer.Header()
    h.Set("X-Content-Type-Options", "nosniff")
    h.Set("X-Frame-Options", "DENY")
    h.Set("Referrer-Policy", "same-origin")
    h.Set("Content-Security-Policy",
        "default-src 'self'; "+
            "style-src 'self' 'unsafe-inline'; "+
            "img-src 'self' data:; "+
            "frame-ancestors 'none'; "+
            "base-uri 'self'; "+
            "form-action 'self'")
    if os.Getenv("APP_ENV") == "production" {
        h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
    }
    c.Next()
}
```

**Each header, what it defends:**

- **`X-Content-Type-Options: nosniff`** - browsers won't MIME-sniff a response. If you serve a `.txt` file the browser treats it as text, not HTML. Defends against an attacker uploading content that the browser would otherwise auto-promote to executable HTML or JavaScript.

- **`X-Frame-Options: DENY`** - the page cannot be loaded inside an `<iframe>`. Defends against clickjacking (an attacker frames your site, overlays a transparent button, tricks a logged-in user into clicking).

- **`Referrer-Policy: same-origin`** - the `Referer` header is only sent when navigating within the same origin. Stops your URLs (which can contain session-relevant info) from leaking to external sites.

- **`Content-Security-Policy: default-src 'self'; ...`** - a whitelist of which origins can serve scripts, styles, images, frames. `'self'` means "this origin only." Defends against XSS by refusing to execute injected `<script src="evil.com/x.js">` even if the attacker manages to inject the tag.

- **`Strict-Transport-Security`** (HSTS) - tells the browser "always use HTTPS for this domain for the next year." Defends against SSL-strip downgrade attacks. Gated on `APP_ENV=production` because the EC2 deploy is HTTP-only on a bare IP - advertising HSTS over HTTP is wrong.

**The `'unsafe-inline'` for `style-src`:**
- The templates rely on `style="..."` attributes in many places (Bootstrap helpers, conditional inline styles)
- A strict CSP would block all of those
- This is a documented trade-off: tightening it requires a template-wide refactor to move all inline styles into stylesheets. Logged in the function comment so the next person doesn't think it's an oversight.

**Router-wide application:**
```go
router.Use(SecurityHeaders)
```
- Registered before any route group, before any other middleware
- Applies to every response - including error responses (404, 500) - because Gin runs middleware before deciding which handler matches

**Tests pin the behavior:**
```go
func TestHSTSGatedOnAppEnv(t *testing.T) {
    // verifies HSTS is OFF by default
    // ...
    t.Setenv("APP_ENV", "production")
    // verifies HSTS is ON
}
```
`t.Setenv(key, value)` sets an environment variable for the duration of the test and restores the previous value at cleanup. Cleaner than manual `os.Setenv` + `defer os.Unsetenv` and safe for parallel-test use within the same test.

See `DECISIONS.md` DEC-028 for the rationale behind the full security-hardening package.

---

## `SetTrustedProxies` - Don't Trust X-Forwarded-* Blindly

```go
if err := router.SetTrustedProxies([]string{"127.0.0.1"}); err != nil {
    log.Fatalf("failed to set trusted proxies: %v", err)
}
```

**The default Gin behavior:**
- Trusts `X-Forwarded-For`, `X-Forwarded-Proto`, etc. from any client
- Means any client can spoof their IP address by sending `X-Forwarded-For: 1.2.3.4`
- Your access logs lie; rate limiting (if added) keys off the wrong IP; geolocation is wrong

**`SetTrustedProxies([]string{"127.0.0.1"})`:**
- Tell Gin to only trust `X-Forwarded-*` headers when they come from the listed proxies
- The EC2 deploy fronts the Go app with nginx on `127.0.0.1` - that's the only proxy in the topology
- Everything else (including direct connections from the public internet, if `:3000` were exposed) gets its actual remote address used as the client IP

**Topology assumption baked into the code:**
- If the deployment ever moves to multiple proxy hops, or to a load balancer not on localhost, this list has to update
- The comment in `main.go:102-106` flags this so the next person knows what to change

**Why `log.Fatalf` on failure:**
- The argument validation can fail (e.g., malformed IP)
- A failed `SetTrustedProxies` leaves the default insecure behavior in place
- Failing closed at startup is better than silently running insecurely

---

## Environment-Variable Feature Gates

```go
if os.Getenv("LIBRESHELF_SKIP_SEED") == "" {
    dm.SeedBooks()
    seedCoverCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
    dm.FetchAndStoreSeedCovers(seedCoverCtx)
    cancel()
}
```

**The pattern:**
- Default behavior: env var unset, seed runs (developer-friendly default)
- Opt-out: set `LIBRESHELF_SKIP_SEED=1` (or any non-empty value), seed skipped

**Where it's useful:**
- Testing the backup-import flow against a clean DB - the import has nothing to verify against if 100 seed books are already there
- Staging deployments that should not carry test data
- CI environments where seed-time matters

**Why not a CLI flag?**
- Env vars compose with systemd unit files and `docker run -e` cleanly
- No need to thread the flag through `flag.Parse()` and the main wiring
- Easy for a deploy script to set without touching the binary's argv

**Same shape elsewhere:**
- `APP_ENV=production` gates HSTS (`handlers.go:37`)
- `APP_ENV=production` gates the `Secure` cookie attribute (`flash.go:77`, `handlers_auth.go:130`)
- `PORT`, `DATA_DIR`, `DB_NAME` configure the runtime (defaults applied if unset)

**Empty-string check vs presence check:**
- `os.Getenv` returns `""` for both "unset" and "set to empty string"
- For boolean toggles, `== ""` is "default behavior; non-default if set to anything"
- For values you actually want to read, use `os.LookupEnv(key)` which returns `(value, ok bool)` to distinguish unset from empty

---

## Public Routes Outside Any Group

```go
// Public routes
router.GET("/login", HandleLogin)
router.POST("/login", LoginCSRFProtect, HandleLoginPost)
router.GET("/kiosk", HandleKiosk)
router.GET("/kiosk/books/:id", HandleKioskBookDetail)

// Authenticated routes -- any logged in user
auth := router.Group("/")
auth.Use(RequireAuth, CSRFProtect, DBReadLock)
// ...
```

**The kiosk is intentionally public:**
- `/kiosk` and `/kiosk/books/:id` are registered directly on `router`, not on any group
- They get `SecurityHeaders` (router-wide middleware) and `DatabaseMiddleware(dm)` (also router-wide)
- They do NOT get `RequireAuth`, `CSRFProtect`, or `DBReadLock`

**Wait - they need DB access. How does that work without `DBReadLock`?**
- Read-only kiosk pages don't take the read lock
- The trade-off: a backup import (write lock) could theoretically conflict with a kiosk page that's mid-query
- For this project's use case (admin imports happen rarely, kiosk pages are short read queries), the contention is acceptable
- If kiosk pages did writes (favorites, ratings), they'd need to move under a group with `DBReadLock`

**Per-handler middleware:**
```go
router.POST("/login", LoginCSRFProtect, HandleLoginPost)
```
- `gin.GET/POST/...` accept multiple handlers - the last one is the actual handler, the rest are middleware that runs first
- Used here because `/login POST` is the only endpoint that needs the special login-CSRF flow (different from the session-based CSRF used by authenticated routes)
- For one-off middleware, this is cleaner than creating a group of size one

**The route registration block as documentation:**
- Glance at `main.go:126-179` and you can see exactly which middleware applies to which routes
- Public, auth, patron-only, staff, admin, admin-write are five clear tiers
- No surprises hidden in handlers about "is this user authed?" - the answer is in the group membership

---

## `t.Setenv` and `t.TempDir` - Test Isolation Helpers

```go
func TestHSTSGatedOnAppEnv(t *testing.T) {
    // ... default-env assertions ...

    t.Setenv("APP_ENV", "production")

    // ... production-env assertions ...
}
```

**`t.Setenv(key, value)`:**
- Sets the env var for the duration of the test
- Restores the previous value (or unsets, if it wasn't set) at test cleanup
- Cannot be used in parallel tests (the env is process-global) - calling `t.Setenv` automatically marks the test as not safe for `t.Parallel()`

**Why not `os.Setenv` + `defer os.Unsetenv`?**
- `os.Setenv` doesn't restore the previous value - just unsets
- `defer` doesn't run on test failure paths cleanly when the test panics
- `t.Setenv` integrates with the test framework's cleanup chain

**Sibling helper - `t.TempDir`:**
```go
dir := t.TempDir()  // creates a unique temp directory
// ... use dir ...
// directory is removed automatically when the test ends
```
- Creates a per-test temp directory under `os.TempDir()`
- Auto-removed when the test ends (success or failure)
- Used in `internal/safezip/extract_test.go` for the destination of test extractions

**`t.Cleanup(fn)` - the general form:**
```go
t.Cleanup(func() {
    // runs when this test (or subtest) ends
})
```
- Like `defer` but lives on the `*testing.T`
- Survives panics, runs in reverse order of registration
- Useful for resources that don't have a built-in helper (open DB connections, started servers)

---

## `httptest.NewServer` - Real HTTP for Outbound-Client Tests

```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // pretend to be Open Library
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{"title": "Dune", "authors": [...]}`))
}))
defer srv.Close()

// point the OL client at the test server
oldBase := openLibraryBase
openLibraryBase = srv.URL
defer func() { openLibraryBase = oldBase }()

book, err := FetchOpenLibraryBook(ctx, "9780441013593")
```

**Why this and not just `httptest.NewRecorder`?**
- `NewRecorder` works inside the request-handling layer (you have a `*http.Request`, you call your handler, you read the recorder)
- `NewServer` spins up a real HTTP server on a real localhost port
- Use `NewServer` when the code under test makes an outbound HTTP call - it needs a real URL to talk to

**Where this project uses it:**
- `openlibrary_test.go` - tests `FetchOpenLibraryBook` against a fake OL server
- `covers_test.go` - tests `DownloadCover` against a fake image server

**The pattern in detail:**
1. `httptest.NewServer(handler)` starts a server on a random free port. Returns `*httptest.Server` with a `.URL` field like `"http://127.0.0.1:34521"`.
2. Override the base URL in the code under test (your code shouldn't have hardcoded `https://openlibrary.org` - it should reference a package-level variable that tests can swap)
3. Run the code under test - it makes a real HTTP call to your fake server
4. `defer srv.Close()` shuts down the server when the test ends

**Race-free port assignment:**
- `NewServer` lets the OS pick a free port (that's what `:0` means in `net.Listen`)
- No collisions between parallel tests, even across packages
- No CI flakiness from "port already in use"

**This was the CP7 coverage push.** Earlier tests used mocks for the OL HTTP client. Switching to `httptest.NewServer` exercises the real HTTP layer (request building, header parsing, error handling) instead of stubbing it out. Higher coverage AND higher fidelity. Closed under PR #76; see `cs408-go-stack-al3` for the full scope.

---

## `os.IsNotExist` and Error-Type Checks

```go
if err := os.Rename(dbPath, dbBakPath); err != nil {
    if !os.IsNotExist(err) {
        log.Printf("HandleBackupImport: rename db -> bak: %v", err)
        recover()
        c.String(http.StatusInternalServerError, "Internal Server Error")
        return
    }
}
```

**The shape:**
- `os.IsNotExist(err)` returns true if `err` indicates a "file or directory does not exist"
- Treat that as "expected on a fresh install" - don't error out
- Treat any other error as a real problem (permission denied, disk full, etc.)

**Companion functions:**
- `os.IsExist(err)` - the inverse, for `os.Mkdir` etc.
- `os.IsPermission(err)` - permission-denied errors

**Modern alternative:**
```go
if errors.Is(err, os.ErrNotExist) {
    // ...
}
```
- `errors.Is` walks wrapper chains; `os.IsNotExist` does not
- For new code, prefer `errors.Is(err, os.ErrNotExist)` - same idiom as `errors.Is(err, sql.ErrNoRows)` from Part 2
- `os.IsNotExist` is older and predates the wrapper machinery; both still work

**The general pattern for OS errors:**
- Treat "doesn't exist" as a known-expected case in code that conditionally creates resources
- Treat all other errors as exceptional and bail out
- Never just `if err != nil { return nil }` - that hides real errors as "not found"

---

## Syntax Summary (Part 3)

| Concept | Syntax | Example in this project |
|---|---|---|
| Current time | `time.Now()` | `handlers_loans.go:52` |
| Add days/months/years | `t.AddDate(y, m, d)` | `time.Now().AddDate(0, 0, 14)` |
| Date duration | `n * time.Second` | `300 * time.Second` |
| Format date | `t.Format("2006-01-02")` | reference time is `Mon Jan 2 15:04:05 MST 2006` |
| Sub-package import | `"github.com/.../internal/pkg"` | `"github.com/timLP79/cs408-go-stack/internal/safezip"` |
| Package doc file | `// Package x ...\npackage x` in `doc.go` | `internal/safezip/doc.go` |
| Open ZIP | `r, err := zip.OpenReader(path)` | `internal/safezip/extract.go:35` |
| Iterate ZIP entries | `for _, f := range r.File` | `extract.go:47, 59` |
| Absolute path | `filepath.Abs(p)` | `extract.go:41` |
| Path join | `filepath.Join(a, b)` | `extract.go:77` |
| Path is absolute | `filepath.IsAbs(p)` | `extract.go:74` |
| Relative path | `filepath.Rel(base, target)` | `extract.go:78` |
| Detect symlink in mode | `mode & os.ModeSymlink != 0` | `extract.go:68` |
| Bounded reader | `io.LimitReader(r, n)` | `extract.go:111` |
| Read-write lock | `var mu sync.RWMutex` | `db.go:443` |
| Take read lock | `mu.RLock(); defer mu.RUnlock()` | `handlers.go:64-65` |
| Take write lock | `mu.Lock(); defer mu.Unlock()` | `handlers_admin.go:150-151` |
| Atomic file rename | `os.Rename(old, new)` | `handlers_admin.go:188, 217` |
| Stack of cleanup closures | `var undo []func(); for i := len(undo)-1; i >= 0; i--` | `handlers_admin.go:162-167` |
| File-not-exist check | `os.IsNotExist(err)` (or `errors.Is(err, os.ErrNotExist)`) | `handlers_admin.go:189` |
| Set response header | `c.Writer.Header().Set(k, v)` | `handlers.go:27-36` |
| Trust local proxy only | `router.SetTrustedProxies([]string{"127.0.0.1"})` | `main.go:107` |
| Single-function middleware | `router.Use(MyMiddleware)` | `router.Use(SecurityHeaders)` |
| Switch on sentinel error | `switch err { case nil: ... case ErrX: ... default: ... }` | `handlers_loans.go:54-68` |
| Per-route middleware | `router.POST(path, mw, handler)` | `main.go:128` |
| Set env var in test | `t.Setenv(k, v)` | `handlers_security_test.go:86` |
| Per-test temp dir | `dir := t.TempDir()` | `internal/safezip/extract_test.go` |
| Real HTTP test server | `srv := httptest.NewServer(handler); defer srv.Close()` | `openlibrary_test.go`, `covers_test.go` |
| Env-gated branch | `if os.Getenv("FLAG") == "" { ... }` | `main.go:38` |
| Distinguish unset from empty | `v, ok := os.LookupEnv(k)` | when you need to know |

---

## Concept Index - Where to Find Each CP6/CP7 Pattern

| Want to see... | Go to |
|---|---|
| Date arithmetic for due dates | `handlers_loans.go:52` |
| Sentinel-error fan-out via `switch err` | `handlers_loans.go:54-68` |
| Derived state (no status column) | `db.go:308-342` (loan queries) + DEC-024 |
| Two-pass ZIP validation | `internal/safezip/extract.go:46-64` |
| Zip Slip defense (the four checks) | `internal/safezip/extract.go:67-91` |
| `io.LimitReader` for zip-bomb defense | `internal/safezip/extract.go:111-118` |
| `sync.RWMutex` declaration | `db.go:443` |
| Read-lock middleware | `handlers.go:62-67` |
| Write-lock handler | `handlers_admin.go:150-151` |
| Why no upgrade is possible | `main.go:174-179` (route group + comment) |
| Atomic rename with rollback stack | `handlers_admin.go:162-232` |
| `VACUUM INTO` for live snapshot | `db.go:436-440` |
| `SecurityHeaders` middleware | `handlers.go:25-41` |
| `SetTrustedProxies` | `main.go:107-109` |
| `LIBRESHELF_SKIP_SEED` env gate | `main.go:38-50` |
| `APP_ENV=production` HSTS gate | `handlers.go:37-39` |
| Public routes outside any group | `main.go:127-130` |
| `t.Setenv` in tests | `handlers_security_test.go:86` |
| `httptest.NewServer` for outbound-call tests | `openlibrary_test.go`, `covers_test.go` |

---

## Key Go Concepts (Part 3)

1. **Derive over store.** Whenever a piece of state is a deterministic function of other columns, derive it instead of duplicating it. Denormalization is a source of drift bugs that don't show up until production data gets messy.
2. **Fail closed before touching the world.** Two-pass validation, `os.IsNotExist`-only soft handling, rollback stacks for partial work - the ZIP extractor and the import handler both treat the world as untouched until everything that could go wrong has been checked.
3. **Concurrency is explicit and limited.** `sync.RWMutex` is the only lock in the entire codebase. The wiring around it (DBReadLock middleware + a separate adminWrite group) is verbose but visible. No hidden locks, no atomic-everything.
4. **Atomic operations come from the OS, not the language.** `os.Rename` for files. `VACUUM INTO` for SQLite. The Go code coordinates these primitives; it doesn't try to invent its own atomicity.
5. **Defense in depth at the HTTP boundary.** `SecurityHeaders` is a single short middleware, but it stacks five different defenses against five different classes of attack. None of them is "the security layer" - each is a small piece of a larger picture that includes parameterized SQL, magic-byte MIME sniffing, constant-time comparisons, `SameSite` cookies, and role-gated route groups.
6. **Configuration via environment variables, not flags.** `PORT`, `DATA_DIR`, `LIBRESHELF_SKIP_SEED`, `APP_ENV` - all read from the env, all with sensible defaults. Plays well with systemd, with Docker, and with the deploy script. The runtime knows nothing about its deployment topology except what the env tells it.
7. **Tests should exercise the real boundary.** `httptest.NewServer` over mocks for outbound HTTP. `setupTestRouter` over a stubbed router for handlers. `t.Setenv` over manual env juggling. The closer the test setup is to production, the more real bugs the tests catch.

---

## Index of CP6/CP7 Decision Records

For the design rationale behind these patterns, see `DECISIONS.md`:

| DEC | Topic | Lives in |
|---|---|---|
| DEC-024 | Loan state derived from `due_date + returned_at`, no status column | `db.go` schema, `db.go` loan queries |
| DEC-025 | `/loans` is one page with an `active|overdue` filter; per-patron grouping deferred | `handlers_loans.go:109-137` |
| DEC-026 | `loans.fine_cents INTEGER NOT NULL DEFAULT 0` reserved on schema; no fine logic | `db.go:576` (schema) |
| DEC-027 | Backup design: ZIP export via `VACUUM INTO`, ZIP import via in-process swap under write lock | `db.go:436-440` + `handlers_admin.go` |
| DEC-028 | `SecurityHeaders` middleware + `SetTrustedProxies` + Go 1.25.0 -> 1.25.9 to clear stdlib CVEs | `handlers.go:25-41` + `main.go:107-109` |
| DEC-029 | `/admin` as a tools index drilling into dedicated pages, not an inline panel | `templates/admin.html` + `handlers_admin.go` |

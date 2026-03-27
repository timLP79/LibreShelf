# LibreShelf — Security Reference

This document covers the security model, known risks, and mitigations for LibreShelf.
See [`plan.md`](./plan.md) for the per-checkpoint security schedule.

---

## Threat Model

LibreShelf is designed for **trusted internal networks** — a school, office, or home library.

**Assumed environment:**
- Deployed behind nginx on a private or semi-private network
- Staff-facing routes require login — unauthenticated users are redirected to `/login`
- Two roles: **admin** (full access) and **patron** (optional login for kiosk features)
- The kiosk is fully public — no login required to browse; patrons may optionally log in to save searches, favorite books, and request holds on checked-out titles. Checkout and return are staff-only actions.

---

## Protections Built Into the Stack

### XSS (Cross-Site Scripting)
Go's `html/template` package **automatically escapes all output**. Any string rendered
in a template is HTML-encoded — `<script>` becomes `&lt;script&gt;`. This protection
is on by default and covers every template in LibreShelf.

**What to avoid:**
```go
// UNSAFE — bypasses escaping entirely
tmpl.Execute(w, template.HTML(userInput))

// SAFE — escaping applied automatically
tmpl.Execute(w, userInput)
```

Never use `template.HTML()`, `template.JS()`, or `template.URL()` on user-supplied data.

---

### SQL Injection
Go's `database/sql` package uses **parameterized queries** with `?` placeholders.
The driver sends the query and parameters separately — user input can never be
interpreted as SQL.

**Always do this:**
```go
// SAFE — parameterized query
row := dm.db.QueryRow("SELECT * FROM books WHERE id = ?", id)

// NEVER do this — SQL injection vulnerability
row := dm.db.QueryRow("SELECT * FROM books WHERE id = " + id)
```

This rule applies to every query in `db.go` without exception.

---

### CDN Supply Chain
Bootstrap CSS and JS are served from `static/` — not from a CDN. A compromised
CDN cannot inject malicious scripts into LibreShelf pages.

---

## File Upload Security (CP3 — Cover Images)

When accepting image uploads for book covers:

1. **Validate MIME type** — check the actual file content, not just the extension:
   ```go
   // Read first 512 bytes and detect content type
   buffer := make([]byte, 512)
   n, _ := file.Read(buffer)
   contentType := http.DetectContentType(buffer[:n])
   if contentType != "image/jpeg" && contentType != "image/png" {
       // reject
   }
   ```

2. **Restrict extensions** — only allow `.jpg`, `.jpeg`, `.png`, `.webp`

3. **Limit file size** — reject files over a reasonable limit (e.g. 5MB):
   ```go
   file, err := c.FormFile("cover")
   if file.Size > 5*1024*1024 {
       c.Status(http.StatusRequestEntityTooLarge)
       return
   }
   ```

4. **Sanitize the filename** — never use the user-supplied filename directly:
   ```go
   // Generate a safe filename
   filename := fmt.Sprintf("%d%s", bookID, filepath.Ext(header.Filename))
   savePath := filepath.Join("static", "images", "covers", filename)
   ```

5. **Store outside web root if possible** — or ensure the upload directory has no
   execute permissions.

---

## ZIP Import Security (CP6 — Zip Slip)

**Zip Slip** is a critical vulnerability where a malicious ZIP contains files with
paths like `../../etc/passwd`. When extracted naively, these overwrite files outside
the intended directory.

**Always validate extracted paths:**
```go
func safeExtractPath(dest, filePath string) (string, error) {
    target := filepath.Join(dest, filePath)
    // Ensure the resolved path starts with the destination directory
    if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
        return "", fmt.Errorf("illegal file path: %s", filePath)
    }
    return target, nil
}
```

Additional ZIP import rules:
- Only extract expected file types (`.sqlite`, `.jpg`, `.png`, `.webp`)
- Limit total extracted size to prevent ZIP bomb attacks
- Validate the DB schema after import before bringing the app back online
- Reject ZIPs with more than a reasonable number of files (e.g. 10,000)

---

## HTTP Security Headers (CP7)

Add a middleware in `main.go` to set security headers on every response:

```go
func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "DENY")
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
        c.Next()
    }
}
```

Register it in `main.go` before the routes:
```go
router.Use(SecurityHeaders())
```

**What each header does:**

| Header | Protection |
|--------|-----------|
| `X-Content-Type-Options: nosniff` | Prevents browser from guessing MIME types — stops MIME-sniffing attacks |
| `X-Frame-Options: DENY` | Prevents the app from being embedded in an iframe — stops clickjacking |
| `Referrer-Policy` | Controls what URL is sent in the `Referer` header on navigation |
| `Permissions-Policy` | Disables browser features the app doesn't use |

---

## HTTPS / TLS

**Not available for this class deployment.**

Let's Encrypt does not issue certificates for bare IP addresses — a domain name is
required. The current EC2 instance uses a raw IP address assigned by the professor,
so HTTPS is not configurable in this environment.

**What this means in practice:**
- The app runs on HTTP only — traffic is not encrypted in transit
- The `Secure` cookie flag must be disabled (see Cookie Security below) — otherwise
  the session cookie is never sent back to the server and login breaks entirely
- This is acceptable for a class project demo; it would not be acceptable in production

**If a domain name becomes available in the future**, nginx TLS termination with
Let's Encrypt would look like this (kept for reference):

```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;

    ssl_certificate     /etc/letsencrypt/live/your-domain/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain/privkey.pem;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    return 301 https://$host$request_uri;
}
```

---

## Trusted Proxies (Fix the Gin Warning)

Gin currently warns: *"You trusted all proxies, this is NOT safe."*

Fix this in `main.go` to tell Gin that only nginx (on localhost) is a trusted proxy:

```go
router := gin.Default()
router.SetTrustedProxies([]string{"127.0.0.1"})
```

This ensures that `X-Forwarded-For` headers are only accepted from nginx, not spoofed
by external clients.

---

## Authentication (CP2)

LibreShelf uses server-side sessions with two roles: `admin` and `patron`.

### Password Hashing — bcrypt

Passwords are hashed with `bcrypt` from `golang.org/x/crypto/bcrypt`.
**Never store plain text, MD5, or SHA passwords.**

```go
import "golang.org/x/crypto/bcrypt"

// Hashing (on account creation or password change)
hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

// Verification (on login)
err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(inputPassword))
if err != nil {
    // wrong password — err is bcrypt.ErrMismatchedHashAndPassword
}
```

`bcrypt.DefaultCost` (currently 10) makes hashing intentionally slow — it takes ~100ms.
This is a feature: it makes brute-force attacks expensive.

### Session Flow

1. User POSTs credentials to `/login`
2. Server verifies password with `bcrypt.CompareHashAndPassword`
3. On success: generate a cryptographically random session token:
   ```go
   token := make([]byte, 32)
   _, err := crypto_rand.Read(token)
   sessionToken := hex.EncodeToString(token)
   ```
4. Store `(token, user_id, expires_at)` in the `sessions` table
5. Set cookie on the response (see Cookie Security below)
6. On subsequent requests: read cookie, look up token in `sessions`, load user

### Cookie Security
```go
// Secure flag requires HTTPS — must be false for HTTP-only deployments.
// If set to true on an HTTP server, the browser will never send the cookie
// back and login will silently break.
secure := os.Getenv("APP_ENV") == "production"

c.SetCookie(
    "session",     // name
    sessionToken,  // value
    28800,         // max age (8 hours)
    "/",           // path
    "",            // domain
    secure,        // Secure — true only when HTTPS is available
    true,          // HttpOnly — always true, blocks JS access
)
```

| Attribute | Purpose |
|-----------|---------|
| `HttpOnly` | Prevents JavaScript from reading the cookie — blocks XSS-based session theft |
| `Secure` | Cookie only sent over HTTPS — disabled here since we are HTTP-only |
| `SameSite=Strict` | Cookie not sent on cross-site requests — prevents CSRF |

> **Note:** `Secure: false` is a known limitation of this deployment. The `HttpOnly`
> and `SameSite=Strict` attributes still protect against XSS-based theft and CSRF.
> If HTTPS becomes available, set `APP_ENV=production` and the flag enables automatically.

### Session Hijacking Prevention
- Tokens generated with `crypto/rand` (cryptographically secure) — never `math/rand`
- Sessions stored server-side in the `sessions` DB table — invalidated on logout
- Session token regenerated after login — prevents session fixation attacks
- 8-hour expiry — user must re-login after expiry
- On logout: delete the session row from the DB immediately

### CSRF Protection
> **Status: NOT YET IMPLEMENTED** — tracked in [#32](https://github.com/timLP79/cs408-go-stack/issues/32), scheduled for CP4.

Every state-changing form (`POST`, `PUT`, `DELETE`) needs a CSRF token:
- Server generates a random token and stores it in the session
- Token is embedded as a hidden field in each form:
  ```html
  <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
  ```
- Server validates the token before processing the form
- Requests without a valid token are rejected with `403 Forbidden`

### Auth Middleware

Two middleware functions protect routes in `main.go`:

```go
// RequireAuth — any logged-in user
func RequireAuth(c *gin.Context) {
    session := getSession(c)
    if session == nil {
        c.Redirect(http.StatusFound, "/login")
        c.Abort()
        return
    }
    c.Set("user", session.User)
    c.Next()
}

// RequireAdmin — admin role only
func RequireAdmin(c *gin.Context) {
    user := c.MustGet("user").(*User)
    if user.Role != "admin" {
        c.Status(http.StatusForbidden)
        renderTemplate(c, "error", gin.H{
            "Status":  403,
            "Message": "Admin access required",
        })
        c.Abort()
        return
    }
    c.Next()
}
```

Applied in `main.go`:
```go
// Public routes — no login required
router.GET("/kiosk", HandleKiosk)

auth := router.Group("/")
auth.Use(RequireAuth)
{
    auth.GET("/", HandleIndex)
    auth.GET("/catalog", HandleCatalog)

    admin := auth.Group("/")
    admin.Use(RequireAdmin)
    {
        admin.GET("/patrons", HandlePatrons)
        admin.GET("/admin", HandleAdmin)
    }
}

// Kiosk optional-auth actions — LoadUser attaches session if present, but does not redirect
router.GET("/kiosk/favorites", LoadUser, HandleKioskFavorites)
router.POST("/kiosk/favorites", LoadUser, HandleKioskAddFavorite)
router.POST("/kiosk/holds", LoadUser, RequireAuth, HandleKioskRequestHold)
```

---

## Data Privacy

- `data/database.sqlite` is **gitignored** — never committed to the repository
- Patron emails are **optional** — never required, never logged
- Set restricted permissions on the data directory on the server:
  ```bash
  chmod 700 data/
  ```
- Do not include patron data in error messages or log output

---

## Dependency Security

Before final deployment (CP7):
```bash
# Verify all dependencies match go.sum checksums
go mod verify

# Check for known vulnerabilities in dependencies
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

Keep `go.sum` committed to the repository — it ensures reproducible, tamper-evident builds.

---

## Quick Security Checklist

### Every CP
- [ ] All DB queries use `?` parameterized placeholders — no string concatenation
- [ ] User input is validated server-side before use
- [ ] Error responses don't leak internal paths, stack traces, or SQL

### CP3
- [ ] Cover image uploads: MIME type checked, extension restricted, size limited, filename sanitized

### CP6
- [ ] ZIP import: path traversal check on every extracted file, size limits enforced

### CP8
- [ ] Security headers middleware added
- [ ] Trusted proxies configured
- [ ] HTTPS configured in nginx (if domain name becomes available)
- [ ] `go mod verify` passes
- [ ] `govulncheck ./...` clean
- [ ] `data/` directory permissions set to `700` on server

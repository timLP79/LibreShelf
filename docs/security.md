# LibreShelf — Security Reference

This document covers the security model, known risks, and mitigations for LibreShelf.
See [`plan.md`](./plan.md) for the per-checkpoint security schedule.

---

## Threat Model

LibreShelf is designed for **trusted internal networks** — a school, office, or home library.

**Assumed environment:**
- Deployed behind nginx on a private or semi-private network
- Staff-facing routes require login; unauthenticated users are redirected to `/login`
- Three roles: **admin** (full access including staff management and system settings), **staff** (day-to-day operations: book/patron CRUD, checkouts, exports), and **patron** (optional login for kiosk features)
- The kiosk is fully public; no login required to browse. Patrons may optionally log in to save searches, favorite books, and request holds on checked-out titles. Checkout and return are handled by admin and staff.

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

## Authentication (CP2, expanded in CP4)

LibreShelf uses server-side sessions with three roles: `admin`, `staff`, and `patron`
(see DEC-014 for the role model).

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
// Secure flag requires HTTPS; must be false for HTTP-only deployments.
// If set to true on an HTTP server, the browser will never send the cookie
// back and login will silently break.
secure := os.Getenv("APP_ENV") == "production"

c.SetSameSite(http.SameSiteStrictMode)  // must be called before SetCookie
c.SetCookie(
    "session",     // name
    sessionToken,  // value
    28800,         // max age (8 hours)
    "/",           // path
    "",            // domain
    secure,        // Secure; true only when HTTPS is available
    true,          // HttpOnly; always true, blocks JS access
)
```

| Attribute | Purpose |
|-----------|---------|
| `HttpOnly` | Prevents JavaScript from reading the cookie; blocks XSS-based session theft |
| `Secure` | Cookie only sent over HTTPS; disabled here since we are HTTP-only |
| `SameSite=Strict` | Cookie not sent on cross-site requests; first line of defense against CSRF |

> **Note:** `Secure: false` is a known limitation of this deployment. The `HttpOnly`
> and `SameSite=Strict` attributes still protect against XSS-based theft and CSRF.
> If HTTPS becomes available, set `APP_ENV=production` and the flag enables automatically.

> **History note:** CP2 documented `SameSite=Strict` as part of the session design but the
> original code never called `c.SetSameSite(http.SameSiteStrictMode)` before `c.SetCookie`,
> so browsers used their default (Lax). The missing call was added in CP4 during the CSRF
> work (#32). See DEC-004 addendum.

### Session Hijacking Prevention
- Tokens generated with `crypto/rand` (cryptographically secure) — never `math/rand`
- Sessions stored server-side in the `sessions` DB table — invalidated on logout
- Session token regenerated after login — prevents session fixation attacks
- 8-hour expiry — user must re-login after expiry
- On logout: delete the session row from the DB immediately

### CSRF Protection (CP4, implemented)

Every state-changing request is protected by a CSRF token. Two separate mechanisms handle
the two cases: authenticated requests use a session-bound synchronizer token; unauthenticated
login requests use a pre-session double-submit cookie. See DEC-017 for the full design
rationale.

**Session-bound token (authenticated routes):**
- On successful login, a cryptographically random token is generated alongside the session
  token and stored on the `sessions` row in the `csrf_token` column.
- `RequireAuth` and `LoadUser` middleware read the session from the DB and set both the
  user and the CSRF token into the gin request context (`c.Set("csrfToken", ...)`).
- `renderTemplate` auto-injects `CSRFToken` into template data (same pattern as `User`).
- Forms embed the token as a hidden field:
  ```html
  <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
  ```
- `CSRFProtect` middleware attached to the auth/staff/admin route groups validates the form
  field against the context token using `subtle.ConstantTimeCompare` on any unsafe-method
  request (POST/PUT/PATCH/DELETE). Safe methods (GET/HEAD/OPTIONS) bypass the check.
- Requests without a valid token are rejected with `403 Forbidden`.

**Pre-session double-submit cookie (`POST /login`):**
- `GET /login` generates a random token, sets it as a `csrf_login` cookie (HttpOnly,
  SameSite=Strict, 10-minute lifetime), and embeds the same token in the login form.
- `LoginCSRFProtect` middleware on `POST /login` reads the cookie and the form field and
  rejects with 403 if either is missing or they do not match.
- On successful login, the `csrf_login` cookie is cleared; the session-bound token takes
  over for all subsequent requests.

**Route wiring:**
```go
router.POST("/login", LoginCSRFProtect, HandleLoginPost)

auth := router.Group("/")
auth.Use(RequireAuth, CSRFProtect)
auth.POST("/logout", HandleLogout)
// ... other auth routes

staff := router.Group("/")
staff.Use(RequireAuth, RequireStaff, CSRFProtect)
// ... staff routes

admin := router.Group("/")
admin.Use(RequireAuth, RequireAdmin, CSRFProtect)
// ... admin routes
```

**Test coverage:** `TestLoginCSRFRejectsMissingCookie`, `TestLoginCSRFRejectsMismatchedToken`,
`TestCSRFProtectAllowsGet`, `TestCSRFProtectRejectsMissingToken`,
`TestCSRFProtectRejectsMismatchedToken`, `TestCSRFProtectAcceptsMatchingToken`, and
`TestAuthenticatedPOSTWithCSRF` (end-to-end `RequireAuth` + `CSRFProtect` + `HandleLogout`).

### Auth Middleware (three roles, chained)

Four middleware functions protect routes in `main.go`. They are chained so each is
single-purpose and stateless (see DEC-014):

- `RequireAuth` validates the session cookie, loads the session, and sets both the user
  and the CSRF token into the request context. Redirects to `/login` on failure.
- `RequireStaff` checks that the role is admin or staff (rejects patron). Must run after
  `RequireAuth`.
- `RequireAdmin` checks that the role is admin. Must run after `RequireAuth`.
- `LoadUser` is the optional-auth variant used on the public kiosk: it attaches the user
  and CSRF token if a session is present but does not redirect otherwise.

```go
// RequireAuth: any logged-in user
func RequireAuth(c *gin.Context) {
    token, err := c.Cookie("session")
    if err != nil {
        c.Redirect(http.StatusFound, "/login")
        c.Abort()
        return
    }
    dm := getDB(c)
    session, err := dm.GetSession(token)
    if err != nil {
        c.Redirect(http.StatusFound, "/login")
        c.Abort()
        return
    }
    c.Set("user", session.User)
    c.Set("csrfToken", session.CSRFToken)
    c.Next()
}

// RequireStaff: admin or staff (rejects patron)
// RequireAdmin: admin only
// Both must run after RequireAuth in the middleware chain.
```

Applied in `main.go` (see also DEC-017 for the CSRF middleware on each group):
```go
// Public routes
router.GET("/login", HandleLogin)
router.POST("/login", LoginCSRFProtect, HandleLoginPost)
router.GET("/kiosk", HandleKiosk)

// Authenticated (any logged-in user)
auth := router.Group("/")
auth.Use(RequireAuth, CSRFProtect)
auth.GET("/", HandleIndex)
auth.GET("/catalog", HandleCatalog)
auth.GET("/books/:id", HandleBookDetail)
auth.POST("/logout", HandleLogout)

// Staff (admin + staff)
staff := router.Group("/")
staff.Use(RequireAuth, RequireStaff, CSRFProtect)
staff.GET("/patrons", HandlePatrons)
staff.GET("/admin", HandleAdmin)

// Admin-only
admin := router.Group("/")
admin.Use(RequireAuth, RequireAdmin, CSRFProtect)
// (admin-only routes added in later checkpoints)
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

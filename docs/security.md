# LibreShelf — Security Reference

This document covers the security model, known risks, and mitigations for LibreShelf.
See [`plan.md`](./plan.md) for the per-checkpoint security schedule.

---

## Threat Model

LibreShelf is designed for **trusted internal networks** — a school, office, or home library.
It is not designed to be exposed directly to the public internet without additional hardening
(authentication, rate limiting, TLS).

**Assumed environment:**
- Deployed behind nginx on a private or semi-private network
- Users are trusted staff or patrons who have physical access to the location
- The kiosk is publicly accessible within the building — not on the open internet

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

## HTTPS / TLS (CP7)

All traffic should be encrypted in transit. nginx handles TLS termination.

**nginx config addition:**
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

For a free certificate, use [Let's Encrypt](https://letsencrypt.org/) via `certbot`.

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

## If Authentication Is Added

LibreShelf v1 has no authentication. If it is added in a future version:

### Cookie Security
```go
c.SetCookie(
    "session",     // name
    sessionToken,  // value
    3600,          // max age (seconds)
    "/",           // path
    "",            // domain
    true,          // Secure — HTTPS only
    true,          // HttpOnly — not accessible to JavaScript
)
```

| Attribute | Purpose |
|-----------|---------|
| `HttpOnly` | Prevents JavaScript from reading the cookie — blocks XSS-based session theft |
| `Secure` | Cookie only sent over HTTPS — prevents interception on plain HTTP |
| `SameSite=Strict` | Cookie not sent on cross-site requests — prevents CSRF |

### Session Hijacking Prevention
- Generate session tokens using `crypto/rand` (cryptographically secure), not `math/rand`
- Store sessions server-side — invalidate them on logout
- Regenerate the session token after login (prevents session fixation)
- Set short timeouts (e.g. 8 hours) with sliding renewal on activity

### CSRF Protection
Every state-changing form (`POST`, `PUT`, `DELETE`) needs a CSRF token:
- Server generates a random token and stores it in the session
- Token is embedded as a hidden field in each form
- Server validates the token before processing the form
- Requests without a valid token are rejected with `403 Forbidden`

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

### CP7
- [ ] Security headers middleware added
- [ ] Trusted proxies configured
- [ ] HTTPS configured in nginx
- [ ] `go mod verify` passes
- [ ] `govulncheck ./...` clean
- [ ] `data/` directory permissions set to `700` on server

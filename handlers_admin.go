package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"libreshelf/internal/safezip"
)

type BackupStats struct {
	Books        int
	Patrons      int
	ActiveLoans  int
	OverdueLoans int
	TotalLoans   int
}

func fetchBackupStats(dm *DatabaseManager) (BackupStats, error) {
	var s BackupStats
	var err error
	if s.Books, err = dm.CountBooks(); err != nil {
		return s, err
	}
	if s.Patrons, err = dm.CountPatrons(); err != nil {
		return s, err
	}
	if s.ActiveLoans, err = dm.CountActiveLoans(); err != nil {
		return s, err
	}
	if s.OverdueLoans, err = dm.CountOverdueLoans(); err != nil {
		return s, err
	}
	if s.TotalLoans, err = dm.CountTotalLoans(); err != nil {
		return s, err
	}
	return s, nil
}

func HandleBackupAdmin(c *gin.Context) {
	dm := getDB(c)
	stats, err := fetchBackupStats(dm)
	if err != nil {
		log.Printf("HandleBackupAdmin: fetchBackupStats: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	renderTemplate(c, "backup_admin", gin.H{
		"Title":   "Backup and Restore",
		"Stats":   stats,
		"Error":   readAndClearFlash(c, flashKindError),
		"Success": readAndClearFlash(c, flashKindSuccess),
	})
}

// dataDirOrDefault returns DATA_DIR or the "data" default. Mirrors the
// logic in main.go and covers.go so the import/export handlers don't
// have to import each other's helpers.
func dataDirOrDefault() string {
	if d := os.Getenv("DATA_DIR"); d != "" {
		return d
	}
	return "data"
}

// HandleBackupImport accepts a multipart upload of a backup ZIP, validates
// it via safezip (Zip Slip / symlink / absolute / size protections), and
// swaps it into place under a write lock. The previous database and
// covers directory are kept as <name>.bak for one-step rollback. Live
// sessions are preserved across the swap so the admin clicking restore
// stays logged in.
func HandleBackupImport(c *gin.Context) {
	if c.PostForm("confirm") != "on" {
		c.String(http.StatusBadRequest, "Confirmation checkbox required")
		return
	}

	file, err := c.FormFile("backup_zip")
	if err != nil {
		c.String(http.StatusBadRequest, "Missing or invalid backup ZIP upload")
		return
	}

	dataDir := dataDirOrDefault()
	stagingRoot := filepath.Join(dataDir, ".staging")
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		log.Printf("HandleBackupImport: staging mkdir: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	workDir, err := os.MkdirTemp(stagingRoot, "import-")
	if err != nil {
		log.Printf("HandleBackupImport: work dir: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	defer os.RemoveAll(workDir)

	uploadPath := filepath.Join(workDir, "upload.zip")
	if err := c.SaveUploadedFile(file, uploadPath); err != nil {
		log.Printf("HandleBackupImport: save upload: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	extractDir := filepath.Join(workDir, "extracted")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		log.Printf("HandleBackupImport: extract dir: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if err := safezip.SafeExtract(uploadPath, extractDir); err != nil {
		log.Printf("HandleBackupImport: SafeExtract: %v", err)
		c.String(http.StatusBadRequest, fmt.Sprintf("Backup ZIP rejected: %v", err))
		return
	}

	snapshotPath := filepath.Join(extractDir, "database.sqlite")
	if _, err := os.Stat(snapshotPath); err != nil {
		c.String(http.StatusBadRequest, "Backup is missing database.sqlite")
		return
	}
	extractedCovers := filepath.Join(extractDir, "covers")
	coversInBackup := false
	if info, err := os.Stat(extractedCovers); err == nil && info.IsDir() {
		coversInBackup = true
	}

	dm := getDB(c)
	dbPath := dm.dbPath
	dbBakPath := dbPath + ".bak"
	coversPath := filepath.Join(dataDir, "covers")
	coversBakPath := coversPath + ".bak"

	savedSessions, err := dm.DumpSessions()
	if err != nil {
		log.Printf("HandleBackupImport: DumpSessions: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	if err := dm.db.Close(); err != nil {
		log.Printf("HandleBackupImport: close DB (continuing): %v", err)
	}

	// WAL/SHM sidecar files are tied to the old DB by filename, not by
	// content. Remove them so the freshly-installed DB starts clean.
	os.Remove(dbPath + "-wal")
	os.Remove(dbPath + "-shm")

	var undo []func()
	rollback := func() {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
	}
	// recover puts the original DB back in dm.db so subsequent requests
	// don't crash with a nil pointer. Must be called from every error
	// path after we've taken the write lock.
	recover := func() {
		rollback()
		if newDB, err := openDB(dbPath); err == nil {
			dm.db = newDB
		} else {
			log.Printf("HandleBackupImport: failed to reopen DB after rollback: %v", err)
		}
	}

	if _, err := os.Stat(dbBakPath); err == nil {
		if err := os.RemoveAll(dbBakPath); err != nil {
			log.Printf("HandleBackupImport: remove old db.bak: %v", err)
			recover()
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
	}
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

	if _, err := os.Stat(coversBakPath); err == nil {
		if err := os.RemoveAll(coversBakPath); err != nil {
			log.Printf("HandleBackupImport: remove old covers.bak: %v", err)
			recover()
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
	}
	if _, err := os.Stat(coversPath); err == nil {
		if err := os.Rename(coversPath, coversBakPath); err != nil {
			log.Printf("HandleBackupImport: rename covers -> bak: %v", err)
			recover()
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		undo = append(undo, func() { os.Rename(coversBakPath, coversPath) })
	}

	if err := os.Rename(snapshotPath, dbPath); err != nil {
		log.Printf("HandleBackupImport: install new db: %v", err)
		recover()
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	undo = append(undo, func() { os.Remove(dbPath) })

	if coversInBackup {
		if err := os.Rename(extractedCovers, coversPath); err != nil {
			log.Printf("HandleBackupImport: install new covers: %v", err)
			recover()
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		undo = append(undo, func() { os.RemoveAll(coversPath) })
	}

	newDB, err := openDB(dbPath)
	if err != nil {
		log.Printf("HandleBackupImport: open new db: %v", err)
		recover()
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	dm.db = newDB

	if err := dm.RestoreSessions(savedSessions); err != nil {
		// Don't roll back the entire import for this. Worst case the
		// admin gets logged out and signs back in.
		log.Printf("HandleBackupImport: RestoreSessions (continuing): %v", err)
	}

	setFlash(c, flashKindSuccess, "backup_imported")
	c.Redirect(http.StatusSeeOther, "/admin/backup")
}

// HandleBackupExport snapshots the database and streams a backup ZIP to
// the client. The ZIP includes the full database (including the sessions
// table) and the covers directory, excluding *.bak files. Admin-only
// via the route group middleware; the response is treated as sensitive.
func HandleBackupExport(c *gin.Context) {
	dm := getDB(c)

	tempDir, err := os.MkdirTemp("", "libreshelf-backup-")
	if err != nil {
		log.Printf("HandleBackupExport: MkdirTemp: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	defer os.RemoveAll(tempDir)

	snapshotPath := filepath.Join(tempDir, "snapshot.sqlite")
	if err := dm.SnapshotTo(snapshotPath); err != nil {
		log.Printf("HandleBackupExport: SnapshotTo: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	zipPath := filepath.Join(tempDir, "backup.zip")
	if err := buildBackupZip(coversDir(), snapshotPath, zipPath); err != nil {
		log.Printf("HandleBackupExport: buildBackupZip: %v", err)
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	downloadName := fmt.Sprintf("libreshelf-backup-%s.zip",
		time.Now().UTC().Format("2006-01-02T15-04-05Z"))
	c.FileAttachment(zipPath, downloadName)
}

// buildBackupZip writes a backup ZIP at zipPath containing the database
// snapshot (as "database.sqlite") and every regular file under coversDir
// (as "covers/<relpath>"), excluding *.bak. Entries use forward-slash
// paths regardless of host OS. The zip is Deflate-compressed.
func buildBackupZip(coversDir, snapshotPath, zipPath string) error {
	out, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)

	if err := addFileToZip(zw, snapshotPath, "database.sqlite"); err != nil {
		return fmt.Errorf("add database to zip: %w", err)
	}

	if _, err := os.Stat(coversDir); err == nil {
		walkErr := filepath.Walk(coversDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if strings.HasSuffix(info.Name(), ".bak") {
				return nil
			}
			rel, err := filepath.Rel(coversDir, path)
			if err != nil {
				return err
			}
			return addFileToZip(zw, path, "covers/"+filepath.ToSlash(rel))
		})
		if walkErr != nil {
			return fmt.Errorf("walk covers: %w", walkErr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat covers: %w", err)
	}

	return zw.Close()
}

func addFileToZip(zw *zip.Writer, src, name string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	fh, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	fh.Name = name
	fh.Method = zip.Deflate

	w, err := zw.CreateHeader(fh)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, in)
	return err
}

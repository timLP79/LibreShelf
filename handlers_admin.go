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
		"Title": "Backup and Restore",
		"Stats": stats,
	})
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

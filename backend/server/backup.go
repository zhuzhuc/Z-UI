package server

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"zui/storage"
)

type BackupHandler struct {
	store *storage.Store
}

func NewBackupHandler(store *storage.Store) *BackupHandler {
	return &BackupHandler{store: store}
}

// Download creates and sends a zip backup of the database and xray config.
func (h *BackupHandler) Download(c *gin.Context) {
	tmpFile := fmt.Sprintf("/tmp/z-ui-backup-%s.zip", time.Now().Format("20060102-150405"))
	if err := createBackupZip(tmpFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer os.Remove(tmpFile)

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="z-ui-backup-%s.zip"`, time.Now().Format("20060102-150405")))
	c.Header("Content-Type", "application/zip")
	c.File(tmpFile)
}

// Restore accepts an uploaded zip file and restores the database and config.
func (h *BackupHandler) Restore(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "upload file is required"})
		return
	}

	if !strings.HasSuffix(strings.ToLower(file.Filename), ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only .zip files are accepted"})
		return
	}

	// Save uploaded file to temp
	tmpFile := fmt.Sprintf("/tmp/z-ui-restore-%s.zip", time.Now().Format("20060102-150405"))
	if err := c.SaveUploadedFile(file, tmpFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer os.Remove(tmpFile)

	restored, err := restoreFromZip(tmpFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	recordAudit(c, h.store, "backup.restore", file.Filename, fmt.Sprintf("restored %d files", restored))
	c.JSON(http.StatusOK, gin.H{"message": "backup restored", "filesRestored": restored})
}

func createBackupZip(outputPath string) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	w := zip.NewWriter(outFile)
	defer w.Close()

	// Add database
	dbPath := storage.DefaultDBPath()
	if _, err := os.Stat(dbPath); err == nil {
		if err := addFileToZip(w, dbPath, "zui.db"); err != nil {
			return fmt.Errorf("backup db: %w", err)
		}
	}

	// Add xray config
	configPath := envOrDefault("XRAY_CONFIG", "./runtime/xray-config.json")
	if _, err := os.Stat(configPath); err == nil {
		if err := addFileToZip(w, configPath, "xray-config.json"); err != nil {
			return fmt.Errorf("backup xray config: %w", err)
		}
	}

	// Add secret file
	secretFile := envOrDefault("ZUI_SECRET_FILE", "./data/.panel_secret")
	if _, err := os.Stat(secretFile); err == nil {
		if err := addFileToZip(w, secretFile, ".panel_secret"); err != nil {
			return fmt.Errorf("backup secret: %w", err)
		}
	}

	return nil
}

func restoreFromZip(archivePath string) (int, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0, fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	restored := 0
	for _, f := range r.File {
		name := filepath.Base(f.Name)
		var destPath string

		switch name {
		case "zui.db":
			destPath = storage.DefaultDBPath()
		case "xray-config.json":
			destPath = envOrDefault("XRAY_CONFIG", "./runtime/xray-config.json")
		case ".panel_secret":
			destPath = envOrDefault("ZUI_SECRET_FILE", "./data/.panel_secret")
		default:
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return restored, fmt.Errorf("mkdir for %s: %w", destPath, err)
		}

		srcFile, err := f.Open()
		if err != nil {
			return restored, fmt.Errorf("open %s in zip: %w", f.Name, err)
		}

		destFile, err := os.Create(destPath)
		if err != nil {
			srcFile.Close()
			return restored, fmt.Errorf("create %s: %w", destPath, err)
		}

		if _, err := io.Copy(destFile, srcFile); err != nil {
			srcFile.Close()
			destFile.Close()
			return restored, fmt.Errorf("write %s: %w", destPath, err)
		}
		srcFile.Close()
		destFile.Close()
		restored++
	}
	return restored, nil
}

func addFileToZip(w *zip.Writer, filePath, zipName string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = zipName
	header.Method = zip.Deflate

	writer, err := w.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

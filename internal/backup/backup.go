package backup

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/yamovo/contentx/internal/config"
	"gorm.io/gorm"
)

// Manager handles database backups.
type Manager struct {
	cfg config.BackupConfig
	db  *gorm.DB
}

// NewManager creates a new backup manager.
func NewManager(cfg config.BackupConfig, db *gorm.DB) *Manager {
	return &Manager{cfg: cfg, db: db}
}

// Backup creates a database backup.
func (m *Manager) Backup() (string, error) {
	if err := os.MkdirAll(m.cfg.Dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup dir: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("backup-%s.sql", timestamp)
	path := filepath.Join(m.cfg.Dir, filename)

	// Get database driver from connection.
	dbName := m.db.Name()
	var cmd *exec.Cmd

	switch dbName {
	case "postgres":
		cmd = exec.Command("pg_dump", "--no-password", "-f", path)
	case "mysql":
		cmd = exec.Command("mysqldump", "--result-file="+path)
	default:
		// SQLite: just copy the file.
		return m.backupSQLite(path)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("backup command failed: %s: %w", string(output), err)
	}

	slog.Info("backup created", "path", path, "size", fileSize(path))

	// Cleanup old backups.
	m.cleanup()

	return path, nil
}

// backupSQLite copies the SQLite database file.
func (m *Manager) backupSQLite(destPath string) (string, error) {
	// For SQLite, use the .backup command via sql.
	sqlDB, err := m.db.DB()
	if err != nil {
		return "", err
	}

	// Simple approach: use VACUUM INTO.
	_, err = sqlDB.Exec("VACUUM INTO ?", destPath)
	if err != nil {
		// Fallback: file copy.
		return m.fileCopy(destPath)
	}

	slog.Info("SQLite backup created", "path", destPath)
	m.cleanup()
	return destPath, nil
}

// fileCopy copies the database file directly.
func (m *Manager) fileCopy(destPath string) (string, error) {
	src, err := os.Open(m.cfg.Dir + "/../contentx.db")
	if err != nil {
		return "", fmt.Errorf("failed to open source db: %w", err)
	}
	defer src.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dest.Close()

	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, wErr := dest.Write(buf[:n]); wErr != nil {
				return "", wErr
			}
		}
		if err != nil {
			break
		}
	}

	slog.Info("file copy backup created", "path", destPath)
	m.cleanup()
	return destPath, nil
}

// cleanup removes old backups beyond max count.
func (m *Manager) cleanup() {
	if m.cfg.MaxBackups <= 0 {
		return
	}

	entries, err := os.ReadDir(m.cfg.Dir)
	if err != nil {
		return
	}

	var files []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e)
		}
	}

	if len(files) <= m.cfg.MaxBackups {
		return
	}

	// Sort by name (timestamp-based names sort correctly).
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	// Remove oldest.
	toRemove := files[:len(files)-m.cfg.MaxBackups]
	for _, f := range toRemove {
		path := filepath.Join(m.cfg.Dir, f.Name())
		os.Remove(path)
		slog.Info("removed old backup", "path", path)
	}
}

// List returns available backups.
func (m *Manager) List() ([]BackupInfo, error) {
	entries, err := os.ReadDir(m.cfg.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var backups []BackupInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, _ := e.Info()
		backups = append(backups, BackupInfo{
			Name:      e.Name(),
			Path:      filepath.Join(m.cfg.Dir, e.Name()),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	return backups, nil
}

// BackupInfo represents a backup file.
type BackupInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

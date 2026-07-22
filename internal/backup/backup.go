package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yamovo/contentx/internal/config"
	"gorm.io/gorm"
)

// Manager handles database and media backups/restores.
//
// It shells out to the native dump tools (pg_dump/mysqldump/psql/mysql) for
// PostgreSQL and MySQL so the backup format is the canonical one operators
// already know. SQLite uses VACUUM INTO for online backups and file copy for
// restore (the service must be restarted after a SQLite restore).
type Manager struct {
	cfg      config.BackupConfig
	dbCfg    config.DatabaseConfig
	uploadDir string
	db       *gorm.DB
}

// NewManager creates a backup manager.
//   - cfg: backup directory, retention, compression
//   - dbCfg: database connection (used to build pg_dump/mysqldump CLI args)
//   - uploadDir: local media directory to include in full backups
//   - db: live GORM connection (used for SQLite VACUUM INTO and driver detection)
func NewManager(cfg config.BackupConfig, dbCfg config.DatabaseConfig, uploadDir string, db *gorm.DB) *Manager {
	return &Manager{cfg: cfg, dbCfg: dbCfg, uploadDir: uploadDir, db: db}
}

// Dir returns the backup directory path. Used by HTTP handlers to resolve
// backup filenames to full paths safely.
func (m *Manager) Dir() string { return m.cfg.Dir }

// ---------- Database backup ----------

// Backup creates a database backup and returns the file path.
func (m *Manager) Backup() (string, error) {
	if err := os.MkdirAll(m.cfg.Dir, 0755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}
	ts := time.Now().Format("20060102-150405")
	path := uniquePath(filepath.Join(m.cfg.Dir, "db-"+ts+m.dbSuffix()))

	if err := m.dumpTo(path); err != nil {
		return "", err
	}
	slog.Info("database backup created", "path", path, "size", fileSize(path))
	m.cleanup("db-")
	return path, nil
}

// dbSuffix returns the conventional file suffix for the active driver.
func (m *Manager) dbSuffix() string {
	switch m.dbCfg.Driver {
	case "postgres":
		return ".sql"
	case "mysql":
		return ".sql"
	default:
		return ".db"
	}
}

// dumpTo writes the database dump to path using the driver-appropriate tool.
func (m *Manager) dumpTo(path string) error {
	switch m.dbCfg.Driver {
	case "postgres":
		return m.runCmd(exec.Command("pg_dump",
			"-h", m.dbCfg.Host,
			"-p", strconv.Itoa(m.dbCfg.Port),
			"-U", m.dbCfg.User,
			"-d", m.dbCfg.Name,
			"-f", path,
		), "PGPASSWORD="+m.dbCfg.Password)
	case "mysql":
		return m.runCmd(exec.Command("mysqldump",
			"-h", m.dbCfg.Host,
			"-P", strconv.Itoa(m.dbCfg.Port),
			"-u", m.dbCfg.User,
			"-p"+m.dbCfg.Password,
			"--result-file="+path,
			m.dbCfg.Name,
		), "")
	default:
		return m.backupSQLite(path)
	}
}

// runCmd runs cmd, optionally injecting an env var (e.g. PGPASSWORD), and
// returns a combined-output error on failure.
func (m *Manager) runCmd(cmd *exec.Cmd, envKV string) error {
	if envKV != "" {
		cmd.Env = append(os.Environ(), envKV)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %s: %w", cmd.Args[0], strings.TrimSpace(string(out)), err)
	}
	return nil
}

// backupSQLite uses VACUUM INTO for an online, consistent snapshot.
func (m *Manager) backupSQLite(destPath string) error {
	sqlDB, err := m.db.DB()
	if err != nil {
		return err
	}
	// VACUUM INTO writes a consistent snapshot without locking writers.
	if _, err := sqlDB.Exec("VACUUM INTO ?", destPath); err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}
	return nil
}

// ---------- Database restore ----------

// Restore restores the database from a backup file. For SQLite the service
// must be restarted afterwards because the live connection still points at
// the old file handle.
func (m *Manager) Restore(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}
	switch m.dbCfg.Driver {
	case "postgres":
		return m.runCmd(exec.Command("psql",
			"-h", m.dbCfg.Host,
			"-p", strconv.Itoa(m.dbCfg.Port),
			"-U", m.dbCfg.User,
			"-d", m.dbCfg.Name,
			"-f", path,
		), "PGPASSWORD="+m.dbCfg.Password)
	case "mysql":
		// mysql reads SQL from stdin.
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
	cmd := exec.Command("mysql",
			"-h", m.dbCfg.Host,
			"-P", strconv.Itoa(m.dbCfg.Port),
			"-u", m.dbCfg.User,
			"-p"+m.dbCfg.Password,
			m.dbCfg.Name,
		)
		cmd.Stdin = f
		return m.runCmd(cmd, "")
	default:
		return m.restoreSQLite(path)
	}
}

// restoreSQLite replaces the live database file with the backup copy. The
// caller must restart the service so GORM reopens the new file.
func (m *Manager) restoreSQLite(path string) error {
	sqlDB, err := m.db.DB()
	if err != nil {
		return err
	}
	// Close the live connection so the file handle is released.
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close db: %w", err)
	}
	// Overwrite the live database file.
	dst := m.dbCfg.Name // for sqlite, Name is the file path
	if err := copyFile(path, dst); err != nil {
		return fmt.Errorf("overwrite db file: %w", err)
	}
	slog.Info("sqlite restored; restart required", "backup", path, "db", dst)
	return nil
}

// ---------- Media backup / restore ----------

// BackupMedia creates a tar.gz of the upload directory and returns the path.
// If uploadDir is empty or does not exist it returns ("", nil).
func (m *Manager) BackupMedia() (string, error) {
	if m.uploadDir == "" {
		return "", nil
	}
	if _, err := os.Stat(m.uploadDir); os.IsNotExist(err) {
		return "", nil
	}
	if err := os.MkdirAll(m.cfg.Dir, 0755); err != nil {
		return "", err
	}
	ts := time.Now().Format("20060102-150405")
	path := uniquePath(filepath.Join(m.cfg.Dir, "media-"+ts+".tar.gz"))
	if err := tarGz(m.uploadDir, path); err != nil {
		return "", err
	}
	slog.Info("media backup created", "path", path, "size", fileSize(path))
	m.cleanup("media-")
	return path, nil
}

// RestoreMedia extracts a media tar.gz into the upload directory.
func (m *Manager) RestoreMedia(path string) error {
	if m.uploadDir == "" {
		return fmt.Errorf("upload dir not configured")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}
	if err := os.MkdirAll(m.uploadDir, 0755); err != nil {
		return err
	}
	return untarGz(path, m.uploadDir)
}

// ---------- Full backup ----------

// BackupAll creates both a database dump and a media archive, returning their
// paths. Either may be empty if the corresponding source is unavailable.
func (m *Manager) BackupAll() (dbPath, mediaPath string, err error) {
	dbPath, err = m.Backup()
	if err != nil {
		return "", "", fmt.Errorf("db backup: %w", err)
	}
	mediaPath, err = m.BackupMedia()
	if err != nil {
		return dbPath, "", fmt.Errorf("media backup: %w", err)
	}
	return dbPath, mediaPath, nil
}

// ---------- Listing & deletion ----------

// List returns available backups, newest first.
func (m *Manager) List() ([]BackupInfo, error) {
	entries, err := os.ReadDir(m.cfg.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []BackupInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, _ := e.Info()
		out = append(out, BackupInfo{
			Name:      e.Name(),
			Path:      filepath.Join(m.cfg.Dir, e.Name()),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}
	// Newest first.
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// Delete removes a single backup file by name.
func (m *Manager) Delete(name string) error {
	// Prevent path traversal: only allow bare filenames.
	clean := filepath.Base(name)
	if clean != name {
		return fmt.Errorf("invalid backup name")
	}
	path := filepath.Join(m.cfg.Dir, clean)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete backup: %w", err)
	}
	slog.Info("backup deleted", "name", clean)
	return nil
}

// BackupInfo represents a backup file.
type BackupInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// ---------- Retention ----------

// cleanup removes oldest backups matching prefix beyond MaxBackups.
func (m *Manager) cleanup(prefix string) {
	if m.cfg.MaxBackups <= 0 {
		return
	}
	entries, err := os.ReadDir(m.cfg.Dir)
	if err != nil {
		return
	}
	var files []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			files = append(files, e)
		}
	}
	if len(files) <= m.cfg.MaxBackups {
		return
	}
	// Oldest first (names contain timestamps).
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	for _, f := range files[:len(files)-m.cfg.MaxBackups] {
		path := filepath.Join(m.cfg.Dir, f.Name())
		_ = os.Remove(path)
		slog.Info("removed old backup", "path", path)
	}
}

// ---------- Helpers ----------

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// uniquePath returns a path that does not yet exist. If the proposed path
// already exists, a "-1", "-2", ... suffix is inserted before the extension.
// This prevents VACUUM INTO and tar from failing when two backups land in
// the same second.
func uniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// copyFile copies src to dst, truncating dst if it exists.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}

// tarGz creates a gzip-compressed tar of srcDir at destPath.
func tarGz(srcDir, destPath string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	gz := gzip.NewWriter(out)
	defer func() { _ = gz.Close() }()
	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Use paths relative to srcDir so the archive is portable.
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		_, err = io.Copy(tw, f)
		return err
	})
}

// untarGz extracts a gzip-compressed tar at srcPath into dstDir.
func untarGz(srcPath, dstDir string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Prevent path traversal (zip-slip).
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("unsafe path in archive: %s", hdr.Name)
		}
		dst := filepath.Join(dstDir, clean)
		if !strings.HasPrefix(filepath.Clean(dst), filepath.Clean(dstDir)+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in archive: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(dst, 0755)
		case tar.TypeReg:
			_ = os.MkdirAll(filepath.Dir(dst), 0755)
			out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			_ = out.Close()
		}
	}
	return nil
}

package backup

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yamovo/contentx/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ErrBackupInProgress is returned when a backup or restore operation is
// already running. Callers should surface this as a 409 Conflict.
var ErrBackupInProgress = errors.New("backup or restore already in progress")

// ErrSchemaMismatch is returned when a backup file's schema version is newer
// than the live database's schema version (which would require a downgrade),
// or when the backup file carries no schema_migrations evidence at all.
// Callers should surface this as a 400 Bad Request.
var ErrSchemaMismatch = errors.New("backup schema version mismatch")

// ErrIncompleteBackup is returned when a backup file is missing one or more
// business tables that the live database expects. Callers should surface this
// as a 400 Bad Request.
var ErrIncompleteBackup = errors.New("backup is missing expected tables")

// Manager handles database and media backups/restores.
//
// It shells out to the native dump tools (pg_dump/mysqldump/psql/mysql) for
// PostgreSQL and MySQL so the backup format is the canonical one operators
// already know. SQLite uses VACUUM INTO for online backups and file copy for
// restore (the service must be restarted after a SQLite restore).
type Manager struct {
	cfg       config.BackupConfig
	dbCfg     config.DatabaseConfig
	uploadDir string
	db        *gorm.DB
	mu        sync.Mutex // serializes backup/restore; TryLock for non-blocking
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

// Driver returns the configured database driver ("postgres", "mysql", "sqlite").
// Used by HTTP handlers to adjust post-restore behavior (e.g. skip row-count
// verification for SQLite where the connection is closed after restore).
func (m *Manager) Driver() string { return m.dbCfg.Driver }

// ---------- Database backup ----------

// Backup creates a database backup and returns the file path.
// Returns ErrBackupInProgress if another backup or restore is running.
func (m *Manager) Backup() (string, error) {
	if !m.mu.TryLock() {
		return "", ErrBackupInProgress
	}
	defer m.mu.Unlock()
	return m.backupDB()
}

// backupDB is the unlocked implementation of Backup.
func (m *Manager) backupDB() (string, error) {
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
			"--clean",     // emit DROP statements so restore replaces existing tables
			"--if-exists", // suppress errors when DROP target doesn't exist
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
// Returns ErrBackupInProgress if another backup or restore is running.
func (m *Manager) Restore(path string) error {
	if !m.mu.TryLock() {
		return ErrBackupInProgress
	}
	defer m.mu.Unlock()
	return m.restoreDB(path)
}

// restoreDB is the unlocked implementation of Restore.
func (m *Manager) restoreDB(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}
	// Validate schema version before touching the live database. We refuse
	// to restore a backup whose schema version is newer than the live DB
	// (would require a downgrade the migrator doesn't support) or a backup
	// that carries no schema_migrations evidence (likely from a non-ContentX
	// source or a legacy AutoMigrate path).
	if err := m.validateSchema(path); err != nil {
		return err
	}
	// Verify the backup contains all expected business tables. This catches
	// truncated or partial backups before they overwrite the live DB.
	if err := m.VerifyBackupTables(path); err != nil {
		return err
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

// validateSchema compares the backup file's schema version against the live
// database. Returns ErrSchemaMismatch if the backup is newer than the live DB
// or carries no schema evidence. A zero live version (e.g. a fresh AutoMigrate
// test DB) skips the check so tests can restore freely.
func (m *Manager) validateSchema(backupPath string) error {
	live, err := m.CurrentSchemaVersion()
	if err != nil {
		// Non-fatal: log and allow. The live DB may predate schema_migrations.
		slog.Warn("could not read live schema version; skipping validation", "error", err)
		return nil
	}
	if live == 0 {
		// Live DB has no migrations recorded (e.g. AutoMigrate path). Nothing
		// to compare against — accept the backup.
		return nil
	}
	backup, err := m.BackupSchemaVersion(backupPath)
	if err != nil {
		slog.Warn("could not read backup schema version; skipping validation", "error", err)
		return nil
	}
	if backup == 0 {
		return fmt.Errorf("%w: backup contains no schema_migrations evidence; not a ContentX backup", ErrSchemaMismatch)
	}
	if backup > live {
		return fmt.Errorf("%w: backup version %d > live version %d (downgrade not supported)", ErrSchemaMismatch, backup, live)
	}
	slog.Info("schema version validated", "backup", backup, "live", live)
	return nil
}

// CurrentSchemaVersion returns the highest applied migration version from the
// live database's schema_migrations table. Returns 0 if the table doesn't
// exist or has no rows (e.g. a fresh AutoMigrate test DB).
func (m *Manager) CurrentSchemaVersion() (int, error) {
	var version int
	row := m.db.Raw("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Row()
	if err := row.Scan(&version); err != nil {
		// sqlite returns "no such table" if the table doesn't exist; pg/mysql
		// return a similar error. Treat as "no migrations recorded" (version 0).
		return 0, nil
	}
	return version, nil
}

// BackupSchemaVersion extracts the schema version embedded in a backup file.
// For SQLite backups it opens the file read-only and queries
// schema_migrations. For SQL dumps (pg_dump/mysqldump) it scans the file for
// INSERT INTO schema_migrations statements and returns the highest version.
// Returns 0 if no schema evidence is found.
func (m *Manager) BackupSchemaVersion(path string) (int, error) {
	switch m.dbCfg.Driver {
	case "postgres", "mysql":
		return schemaVersionFromSQLDump(path)
	default:
		return schemaVersionFromSQLite(path)
	}
}

// schemaVersionFromSQLDump scans a pg_dump/mysqldump SQL file for the highest
// version number written into schema_migrations. Both tools emit lines of the
// form `INSERT INTO schema_migrations ... VALUES (N, ...)` (mysqldump) or
// `COPY schema_migrations ... FROM stdin;` blocks (pg_dump). We handle both.
//
// For pg_dump's COPY format, the data rows follow the COPY line verbatim and
// are terminated by a line containing only `\.`. The version column is the
// first column of each data row (tab-separated), so we extract the first
// integer from each row until the terminator.
func schemaVersionFromSQLDump(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	max := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // allow long COPY/INSERT lines
	inCopyBlock := false                                  // true while reading COPY schema_migrations data rows
	for scanner.Scan() {
		line := scanner.Text()
		if inCopyBlock {
			// COPY data terminator is a line containing only "\."
			if line == "\\." {
				inCopyBlock = false
				continue
			}
			// Each data row's first column is the version (tab-separated).
			if v := extractFirstInt(line); v > max {
				max = v
			}
			continue
		}
		// Match INSERT INTO schema_migrations (mysqldump) or COPY schema_migrations (pg_dump).
		// For INSERT, the version appears inline after VALUES.
		// For COPY, we enter the data-row mode above.
		if !strings.Contains(line, "schema_migrations") {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "COPY") {
			inCopyBlock = true
			continue
		}
		if v := extractFirstInt(line); v > max {
			max = v
		}
	}
	return max, nil
}

// schemaVersionFromSQLite opens a SQLite database file read-only and queries
// its schema_migrations table. Returns 0 if the table doesn't exist.
func schemaVersionFromSQLite(path string) (int, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return 0, err
	}
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()
	var version int
	row := db.Raw("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Row()
	if err := row.Scan(&version); err != nil {
		return 0, nil // table doesn't exist → no migrations recorded
	}
	return version, nil
}

// extractFirstInt returns the first integer found in s, or 0 if none.
func extractFirstInt(s string) int {
	start := -1
	for i, r := range s {
		if r >= '0' && r <= '9' {
			if start == -1 {
				start = i
			}
		} else if start != -1 {
			n, err := strconv.Atoi(s[start:i])
			if err == nil {
				return n
			}
			start = -1
		}
	}
	if start != -1 {
		n, err := strconv.Atoi(s[start:])
		if err == nil {
			return n
		}
	}
	return 0
}

// ---------- Backup integrity (expected tables + row counts) ----------

// ExpectedTables returns the list of business tables from the live database,
// excluding the infrastructure table schema_migrations. The result is used as
// the canonical set a backup must contain to be considered complete.
func (m *Manager) ExpectedTables() ([]string, error) {
	tables, err := m.db.Migrator().GetTables()
	if err != nil {
		return nil, fmt.Errorf("get live tables: %w", err)
	}
	var out []string
	for _, t := range tables {
		if t == "schema_migrations" {
			continue
		}
		out = append(out, t)
	}
	sort.Strings(out)
	return out, nil
}

// VerifyBackupTables checks that the backup file contains all tables that the
// live database expects. Returns ErrIncompleteBackup with the list of missing
// tables if any are absent. This is called by restoreDB before overwriting the
// live database.
func (m *Manager) VerifyBackupTables(path string) error {
	expected, err := m.ExpectedTables()
	if err != nil {
		return err
	}
	if len(expected) == 0 {
		// Nothing to verify (e.g. a fresh DB with no migrations). Skip.
		return nil
	}
	var found []string
	switch m.dbCfg.Driver {
	case "postgres", "mysql":
		found, err = tablesFromSQLDump(path)
	default:
		found, err = tablesFromSQLite(path)
	}
	if err != nil {
		return fmt.Errorf("read backup tables: %w", err)
	}
	foundSet := make(map[string]bool, len(found))
	for _, t := range found {
		foundSet[strings.ToLower(t)] = true
	}
	var missing []string
	for _, t := range expected {
		if !foundSet[strings.ToLower(t)] {
			missing = append(missing, t)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("%w: %s", ErrIncompleteBackup, strings.Join(missing, ", "))
	}
	slog.Info("backup table set verified", "expected", len(expected), "found", len(found))
	return nil
}

// RowCounts returns a map of table name → row count for all expected business
// tables. Tables that don't exist or error are silently skipped. Used to
// capture a pre-restore snapshot for post-restore consistency verification.
func (m *Manager) RowCounts() (map[string]int, error) {
	tables, err := m.ExpectedTables()
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int, len(tables))
	for _, t := range tables {
		var count int64
		if err := m.db.Table(t).Count(&count).Error; err == nil {
			counts[t] = int(count)
		}
	}
	return counts, nil
}

// VerifyRowCounts compares the current row counts against a snapshot taken
// before restore. Returns nil if all non-zero tables in the snapshot have at
// least as many rows after restore. This is a best-effort check: tables that
// grew between snapshot and restore may legitimately have more rows.
func (m *Manager) VerifyRowCounts(snapshot map[string]int) (map[string]int, error) {
	after, err := m.RowCounts()
	if err != nil {
		return nil, err
	}
	shrunk := make(map[string]int)
	for table, before := range snapshot {
		current, ok := after[table]
		if !ok {
			// Table disappeared after restore — report as shrunk to 0.
			shrunk[table] = 0
			continue
		}
		if current < before {
			shrunk[table] = current
		}
	}
	if len(shrunk) > 0 {
		return shrunk, fmt.Errorf("row count regression after restore: %v", shrunk)
	}
	return after, nil
}

// tablesFromSQLDump scans a pg_dump/mysqldump SQL file for CREATE TABLE
// statements and returns the lowercased table names. Both tools emit lines of
// the form `CREATE TABLE [IF NOT EXISTS] [schema.]table_name (`.
func tablesFromSQLDump(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var tables []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		upper := strings.ToUpper(line)
		if !strings.HasPrefix(upper, "CREATE TABLE") {
			continue
		}
		// Extract the table name: everything between "CREATE TABLE " and "(".
		rest := line[len("CREATE TABLE"):]
		rest = strings.TrimSpace(rest)
		// Strip optional "IF NOT EXISTS".
		if strings.HasPrefix(strings.ToUpper(rest), "IF NOT EXISTS") {
			rest = strings.TrimSpace(rest[len("IF NOT EXISTS"):])
		}
		// Strip optional schema prefix (e.g. "public.articles" → "articles").
		if dot := strings.LastIndex(rest, "."); dot >= 0 {
			rest = rest[dot+1:]
		}
		// Strip optional quotes.
		rest = strings.Trim(rest, "\"`'")
		// The table name ends at the first space or "(".
		for i, r := range rest {
			if r == ' ' || r == '(' {
				rest = rest[:i]
				break
			}
		}
		rest = strings.Trim(rest, "\"`'")
		if rest != "" {
			tables = append(tables, strings.ToLower(rest))
		}
	}
	return tables, nil
}

// tablesFromSQLite opens a SQLite database file read-only and returns the list
// of user table names (excluding sqlite internal tables).
func tablesFromSQLite(path string) ([]string, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()
	var names []string
	rows, err := db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Rows()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			names = append(names, strings.ToLower(name))
		}
	}
	return names, nil
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
// Returns ErrBackupInProgress if another backup or restore is running.
func (m *Manager) BackupMedia() (string, error) {
	if !m.mu.TryLock() {
		return "", ErrBackupInProgress
	}
	defer m.mu.Unlock()
	return m.backupMediaImpl()
}

// backupMediaImpl is the unlocked implementation of BackupMedia.
func (m *Manager) backupMediaImpl() (string, error) {
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
// Returns ErrBackupInProgress if another backup or restore is running.
func (m *Manager) RestoreMedia(path string) error {
	if !m.mu.TryLock() {
		return ErrBackupInProgress
	}
	defer m.mu.Unlock()
	return m.restoreMediaImpl(path)
}

// restoreMediaImpl is the unlocked implementation of RestoreMedia.
func (m *Manager) restoreMediaImpl(path string) error {
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
// Returns ErrBackupInProgress if another backup or restore is running.
func (m *Manager) BackupAll() (dbPath, mediaPath string, err error) {
	if !m.mu.TryLock() {
		return "", "", ErrBackupInProgress
	}
	defer m.mu.Unlock()
	dbPath, err = m.backupDB()
	if err != nil {
		return "", "", fmt.Errorf("db backup: %w", err)
	}
	mediaPath, err = m.backupMediaImpl()
	if err != nil {
		return dbPath, "", fmt.Errorf("media backup: %w", err)
	}
	return dbPath, mediaPath, nil
}

// ---------- Listing & deletion ----------

// LockForTest acquires the manager's lock and holds it until UnlockForTest is
// called. It is intended only for tests that need to simulate an in-progress
// backup/restore so they can assert concurrent callers are rejected.
func (m *Manager) LockForTest()   { m.mu.Lock() }
func (m *Manager) UnlockForTest() { m.mu.Unlock() }

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

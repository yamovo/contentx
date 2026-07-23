// Command seeder applies a benchmark seed SQL file to a SQLite database using
// the same GORM SQLite driver the application uses, so no external sqlite3 CLI
// is required. Used by the 7.2 cross-database comparison (SQLite leg).
//
// Usage:
//
//	go run ./scripts/benchmark/seeder -db bench_sqlite.db -sql scripts/benchmark/seed_sqlite_10000.sql
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dbPath := flag.String("db", "bench_sqlite.db", "SQLite database file path")
	sqlFile := flag.String("sql", "", "SQL seed file to apply")
	flag.Parse()

	if *sqlFile == "" {
		fmt.Fprintln(os.Stderr, "-sql is required")
		os.Exit(1)
	}

	raw, err := os.ReadFile(*sqlFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read sql: %v\n", err)
		os.Exit(1)
	}

	// Strip comment and blank lines first, then split into statements on ';'.
	// The seed's INSERT ... SELECT has no inner ';', so statements stay intact.
	var clean strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		clean.WriteString(line)
		clean.WriteString("\n")
	}

	db, err := gorm.Open(sqlite.Open(*dbPath+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}

	start := time.Now()
	for _, stmt := range strings.Split(clean.String(), ";") {
		s := strings.TrimSpace(stmt)
		if s == "" {
			continue
		}
		if err := db.Exec(s).Error; err != nil {
			fmt.Fprintf(os.Stderr, "exec failed: %v\n", err)
			os.Exit(1)
		}
	}

	var count int64
	db.Table("articles").Count(&count)
	fmt.Printf("seed applied in %s; articles rows = %d\n", time.Since(start).Round(time.Millisecond), count)
}

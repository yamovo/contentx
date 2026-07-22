package migrations

import "github.com/yamovo/contentx/internal/database"

// pendingMigrations accumulates migrations registered via init() functions in
// individual migration files (001_*.go, 002_*.go, ...). File naming convention
// ensures init() order matches version order.
var pendingMigrations []database.Migration

// RegisterMigrations adds migrations to the pending list. Called from init()
// in each migration file.
func RegisterMigrations(migs ...database.Migration) {
	pendingMigrations = append(pendingMigrations, migs...)
}

// All returns all registered migrations sorted by their file registration
// order (which follows the 001_, 002_ filename convention).
func All() []database.Migration {
	return pendingMigrations
}

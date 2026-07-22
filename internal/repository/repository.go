// Package repository provides persistence interfaces (Repository pattern)
// decoupling the service layer from direct *gorm.DB usage.
//
// Each aggregate exposes a domain-specific Repository interface. A GORM-backed
// implementation is provided for production use. Services depend on the
// interface, which makes them trivially mockable in unit tests.
//
// Convention:
//   - Interfaces live in this package, named <Aggregate>Repository.
//   - GORM implementations are unexported (gormXxxRepository) and constructed
//     via New<Aggregate>Repository(db *gorm.DB) <Aggregate>Repository.
//   - Cross-aggregate transactional operations (e.g. creating an article with
//     its tags and revision) live on the "owning" repository and encapsulate
//     the transaction internally.
package repository

import "gorm.io/gorm"

// DB is the minimal persistence handle required by GORM-backed repositories.
// It is *gorm.DB in production and can be substituted in tests.
//
// Repositories accept *gorm.DB directly in their constructors; this alias is
// kept for documentation and for any future abstraction that needs to swap the
// concrete type (e.g. a shadow read replica).
type DB = *gorm.DB

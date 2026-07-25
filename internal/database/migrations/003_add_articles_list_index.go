package migrations

import (
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// 003 adds a composite index on articles(status, post_type, is_pinned,
// published_at, created_at) to eliminate the filesort / TEMP B-TREE that
// dominates article-list and GraphQL query latency on MySQL and SQLite.
//
// The default list ordering is:
//
//	ORDER BY is_pinned DESC, published_at DESC, created_at DESC
//
// optionally filtered by status and post_type. PostgreSQL already avoids a
// full sort via Incremental Sort, but MySQL falls back to filesort and SQLite
// to a TEMP B-TREE because no index covers the multi-column ORDER BY. This
// composite index lets those engines seek by the equality predicates and read
// the remaining columns in (reverse) index order.
//
// The index is created ascending for portability; every supported engine can
// scan an ascending index backward to satisfy the all-DESC ORDER BY. See
// reports/benchmarks/cross-db-comparison.md §4.4.
const articlesListSortIdx = "idx_articles_list_sort"

func init() {
	RegisterMigrations(
		database.Migration{
			Version:     3,
			Description: "Add composite index on articles(status, post_type, is_pinned, published_at, created_at)",
			Up: func(tx *gorm.DB) error {
				if tx.Migrator().HasIndex(&models.Article{}, articlesListSortIdx) {
					return nil
				}
				return tx.Exec(
					"CREATE INDEX " + articlesListSortIdx +
						" ON articles(status, post_type, is_pinned, published_at, created_at)",
				).Error
			},
			Down: func(tx *gorm.DB) error {
				if !tx.Migrator().HasIndex(&models.Article{}, articlesListSortIdx) {
					return nil
				}
				return tx.Migrator().DropIndex(&models.Article{}, articlesListSortIdx)
			},
		},
	)
}

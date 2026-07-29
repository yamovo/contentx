package migrations

import (
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// 006 creates the webhook_deliveries table backing the persistent webhook
// delivery queue (STATUS: Webhook 队列、并发限制和退避策略完善).
// Uses GORM CreateTable for the single new table so column types stay
// portable across SQLite, PostgreSQL and MySQL.
func init() {
	RegisterMigrations(
		database.Migration{
			Version:     6,
			Description: "Create webhook_deliveries table for persistent delivery queue",
			Up: func(tx *gorm.DB) error {
				if tx.Migrator().HasTable(&models.WebhookDelivery{}) {
					return nil
				}
				return tx.Migrator().CreateTable(&models.WebhookDelivery{})
			},
			Down: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable(&models.WebhookDelivery{}) {
					return nil
				}
				return tx.Migrator().DropTable(&models.WebhookDelivery{})
			},
		},
	)
}

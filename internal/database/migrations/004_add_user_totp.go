package migrations

import (
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// 004 creates the user_totps table for TOTP two-factor authentication.
// Uses GORM AutoMigrate for the single new table so column types stay
// portable across SQLite, PostgreSQL and MySQL.
func init() {
	RegisterMigrations(
		database.Migration{
			Version:     4,
			Description: "Create user_totps table for TOTP 2FA",
			Up: func(tx *gorm.DB) error {
				if tx.Migrator().HasTable(&models.UserTOTP{}) {
					return nil
				}
				return tx.Migrator().CreateTable(&models.UserTOTP{})
			},
			Down: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable(&models.UserTOTP{}) {
					return nil
				}
				return tx.Migrator().DropTable(&models.UserTOTP{})
			},
		},
	)
}

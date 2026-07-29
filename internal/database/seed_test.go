package database

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
)

func TestSeedAll_PopulatesInitialData(t *testing.T) {
	db := newTestDB(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	t.Setenv("ADMIN_PASSWORD", "SeedTestPass123!")

	if err := SeedAll(db); err != nil {
		t.Fatalf("SeedAll: %v", err)
	}

	var roleCount int64
	db.Model(&models.Role{}).Count(&roleCount)
	if roleCount != 4 {
		t.Errorf("roles = %d, want 4 (admin/editor/author/subscriber)", roleCount)
	}

	var permCount int64
	db.Model(&models.Permission{}).Count(&permCount)
	if permCount == 0 {
		t.Error("permissions should be seeded")
	}

	var admin models.User
	if err := db.Preload("Role").Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("admin user not seeded: %v", err)
	}
	if admin.Role.Slug != "admin" {
		t.Errorf("admin role slug = %q, want admin", admin.Role.Slug)
	}

	var settingCount int64
	db.Model(&models.SiteSetting{}).Count(&settingCount)
	if settingCount == 0 {
		t.Error("site settings should be seeded")
	}

	var catCount int64
	db.Model(&models.Category{}).Count(&catCount)
	if catCount != 4 {
		t.Errorf("categories = %d, want 4", catCount)
	}
}

func TestSeedAll_Idempotent(t *testing.T) {
	db := newTestDB(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	t.Setenv("ADMIN_PASSWORD", "SeedTestPass123!")

	if err := SeedAll(db); err != nil {
		t.Fatalf("first SeedAll: %v", err)
	}
	if err := SeedAll(db); err != nil {
		t.Fatalf("second SeedAll: %v", err)
	}

	// 重复执行不得产生重复数据。
	var roleCount, userCount int64
	db.Model(&models.Role{}).Count(&roleCount)
	db.Model(&models.User{}).Count(&userCount)
	if roleCount != 4 {
		t.Errorf("roles after reseed = %d, want 4", roleCount)
	}
	if userCount != 1 {
		t.Errorf("users after reseed = %d, want 1", userCount)
	}
}

func TestSeedAdminUser_ReleaseModeRequiresPassword(t *testing.T) {
	db := newTestDB(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	t.Setenv("ADMIN_PASSWORD", "")
	t.Setenv("SERVER_MODE", "release")

	// release 模式下缺 ADMIN_PASSWORD 必须失败（SEC-4 相关的启动安全约束）。
	err := SeedAll(db)
	if err == nil {
		t.Fatal("SeedAll should fail in release mode without ADMIN_PASSWORD")
	}
}

func TestGenerateRandomPasswordSeed(t *testing.T) {
	p1, err := generateRandomPasswordSeed(16)
	if err != nil {
		t.Fatalf("generateRandomPasswordSeed: %v", err)
	}
	if len(p1) != 16 {
		t.Errorf("password length = %d, want 16", len(p1))
	}
	p2, _ := generateRandomPasswordSeed(16)
	if p1 == p2 {
		t.Error("two generated passwords should differ")
	}
}

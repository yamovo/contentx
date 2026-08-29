package services

import (
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/permissions"
	"gorm.io/gorm"
)

// ─── TokenService Tests ─────────────────────────────────────────────────────

func createTokenTestTenant(t *testing.T, db *gorm.DB, slug, status string) *models.Tenant {
	t.Helper()
	tenant := &models.Tenant{Name: slug, Slug: slug, Status: status}
	if err := db.Create(tenant).Error; err != nil {
		t.Fatalf("create tenant %q: %v", slug, err)
	}
	return tenant
}

func ensureTokenDefaultTenant(t *testing.T, db *gorm.DB) *models.Tenant {
	t.Helper()
	var tenant models.Tenant
	err := db.First(&tenant, models.DefaultTenantID).Error
	if err == nil {
		return &tenant
	}
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("find default tenant: %v", err)
	}
	tenant = models.Tenant{
		BaseModel: models.BaseModel{ID: models.DefaultTenantID},
		Name:      "Default",
		Slug:      "default",
		Status:    models.TenantStatusActive,
	}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("create default tenant: %v", err)
	}
	return &tenant
}

func grantTokenMembership(t *testing.T, db *gorm.DB, userID, tenantID uint, role string) *models.TenantMembership {
	t.Helper()
	membership := &models.TenantMembership{TenantID: tenantID, UserID: userID, RoleSlug: role}
	if err := db.Create(membership).Error; err != nil {
		t.Fatalf("create membership: %v", err)
	}
	return membership
}

func prepareResolvableTokenUser(t *testing.T, db *gorm.DB, user *models.User, role string) {
	t.Helper()
	ensureTokenDefaultTenant(t, db)
	grantTokenMembership(t, db, user.ID, models.DefaultTenantID, role)
}

func createBoundToken(t *testing.T, svc *TokenService, userID, tenantID uint, permissions []string) *TokenCreatedResponse {
	t.Helper()
	resp, err := svc.Create(CreateTokenRequest{
		Name:        "test-token",
		Permissions: permissions,
	}, userID, tenantID)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	return resp
}

func TestTokenService_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "tokenuser", "admin")
	svc := NewTokenService(db)

	resp, err := svc.Create(CreateTokenRequest{
		Name:        "test-token",
		Permissions: []string{"articles.read", "articles.create"},
	}, user.ID, models.DefaultTenantID)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	if resp.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if resp.Name != "test-token" {
		t.Fatalf("expected name 'test-token', got '%s'", resp.Name)
	}
	if resp.Token == "" {
		t.Fatal("expected non-empty token string")
	}
	if len(resp.Permissions) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(resp.Permissions))
	}
}

func TestTokenService_Create_WithExpiry(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "expiryuser", "admin")
	svc := NewTokenService(db)

	expiry := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	resp, err := svc.Create(CreateTokenRequest{
		Name:      "expiring",
		ExpiresAt: expiry,
	}, user.ID, models.DefaultTenantID)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if resp.ExpiresAt == nil {
		t.Fatal("expected non-nil ExpiresAt")
	}
}

func TestTokenService_Create_InvalidExpiry(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "badexpiryuser", "admin")
	svc := NewTokenService(db)

	_, err := svc.Create(CreateTokenRequest{
		Name:      "bad",
		ExpiresAt: "not-a-date",
	}, user.ID, models.DefaultTenantID)
	if err == nil {
		t.Fatal("expected error for invalid date format")
	}
}

func TestTokenService_List(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "listuser", "admin")
	svc := NewTokenService(db)

	svc.Create(CreateTokenRequest{Name: "token1"}, user.ID, models.DefaultTenantID)
	svc.Create(CreateTokenRequest{Name: "token2"}, user.ID, models.DefaultTenantID)

	tokens, err := svc.List(models.DefaultTenantID)
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
}

func TestTokenService_List_IsTenantScoped(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "scoped-list-user", "admin")
	tenantA := ensureTokenDefaultTenant(t, db)
	tenantB := createTokenTestTenant(t, db, "scoped-list-b", models.TenantStatusActive)
	svc := NewTokenService(db)

	if _, err := svc.Create(CreateTokenRequest{Name: "tenant-a-token"}, user.ID, tenantA.ID); err != nil {
		t.Fatalf("create tenant A token: %v", err)
	}
	if _, err := svc.Create(CreateTokenRequest{Name: "tenant-b-token"}, user.ID, tenantB.ID); err != nil {
		t.Fatalf("create tenant B token: %v", err)
	}

	tokensA, err := svc.List(tenantA.ID)
	if err != nil {
		t.Fatalf("list tenant A: %v", err)
	}
	if len(tokensA) != 1 || tokensA[0].Name != "tenant-a-token" {
		t.Fatalf("tenant A tokens = %+v, want only tenant-a-token", tokensA)
	}
	tokensB, err := svc.List(tenantB.ID)
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(tokensB) != 1 || tokensB[0].Name != "tenant-b-token" {
		t.Fatalf("tenant B tokens = %+v, want only tenant-b-token", tokensB)
	}
}

func TestTokenService_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "deluser", "admin")
	svc := NewTokenService(db)

	resp, _ := svc.Create(CreateTokenRequest{Name: "to-delete"}, user.ID, models.DefaultTenantID)
	if err := svc.Delete(resp.ID, models.DefaultTenantID); err != nil {
		t.Fatalf("delete token: %v", err)
	}

	var count int64
	db.Model(&models.APIToken{}).Where("id = ?", resp.ID).Count(&count)
	if count != 0 {
		t.Fatal("token should be deleted")
	}
}

func TestTokenService_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewTokenService(db)

	err := svc.Delete(99999, models.DefaultTenantID)
	if err == nil {
		t.Fatal("expected error for non-existent token")
	}
}

func TestTokenService_Delete_RejectsCrossTenantID(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "scoped-delete-user", "admin")
	tenantA := ensureTokenDefaultTenant(t, db)
	tenantB := createTokenTestTenant(t, db, "scoped-delete-b", models.TenantStatusActive)
	svc := NewTokenService(db)
	token := createBoundToken(t, svc, user.ID, tenantA.ID, nil)

	if err := svc.Delete(token.ID, tenantB.ID); err == nil {
		t.Fatal("expected cross-tenant delete to return not found")
	}
	var count int64
	if err := db.Model(&models.APIToken{}).Where("id = ?", token.ID).Count(&count).Error; err != nil {
		t.Fatalf("count token after rejected delete: %v", err)
	}
	if count != 1 {
		t.Fatal("cross-tenant delete removed the token")
	}
	if err := svc.Delete(token.ID, tenantA.ID); err != nil {
		t.Fatalf("delete from owning tenant: %v", err)
	}
}

func TestTokenService_Validate_Success(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "validateuser", "admin")
	prepareResolvableTokenUser(t, db, user, models.TenantRoleAdmin)
	svc := NewTokenService(db)

	resp, _ := svc.Create(CreateTokenRequest{
		Name:        "valid-token",
		Permissions: []string{"articles.read"},
	}, user.ID, models.DefaultTenantID)

	valid, userID, err := svc.Validate(resp.Token, "articles.read")
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if !valid {
		t.Fatal("expected token to be valid")
	}
	if userID != user.ID {
		t.Fatalf("expected user ID %d, got %d", user.ID, userID)
	}
}

func TestTokenService_Validate_InvalidToken(t *testing.T) {
	db := setupTestDB(t)
	svc := NewTokenService(db)

	valid, _, err := svc.Validate("vc_live_invalid", "articles.read")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if valid {
		t.Fatal("expected invalid")
	}
}

func TestTokenService_Validate_ExpiredToken(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "expireduser", "admin")
	prepareResolvableTokenUser(t, db, user, models.TenantRoleAdmin)
	svc := NewTokenService(db)

	past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	resp, _ := svc.Create(CreateTokenRequest{
		Name:      "expired",
		ExpiresAt: past,
	}, user.ID, models.DefaultTenantID)

	valid, _, err := svc.Validate(resp.Token, "")
	if err == nil {
		t.Fatal("expected error for expired token")
	}
	if valid {
		t.Fatal("expected expired token to be invalid")
	}
}

func TestTokenService_Validate_WildcardPermission(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "wildcarduser", "admin")
	prepareResolvableTokenUser(t, db, user, models.TenantRoleAdmin)
	svc := NewTokenService(db)

	resp, _ := svc.Create(CreateTokenRequest{
		Name:        "wildcard",
		Permissions: []string{"*"},
	}, user.ID, models.DefaultTenantID)

	valid, _, err := svc.Validate(resp.Token, permissions.ArticlesPublish)
	if err != nil {
		t.Fatalf("validate wildcard: %v", err)
	}
	if !valid {
		t.Fatal("expected wildcard to grant articles.publish within the effective tenant ceiling")
	}
}

func TestTokenService_Resolve_RevalidatesPrincipalLifecycle(t *testing.T) {
	t.Run("inactive creator", func(t *testing.T) {
		db := setupTestDB(t)
		user := createTestUser(t, db, "inactive-token-user", "admin")
		prepareResolvableTokenUser(t, db, user, models.TenantRoleAdmin)
		svc := NewTokenService(db)
		token := createBoundToken(t, svc, user.ID, models.DefaultTenantID, []string{permissions.ArticlesRead})

		if err := db.Model(&models.User{}).Where("id = ?", user.ID).
			Update("status", models.UserStatusInactive).Error; err != nil {
			t.Fatalf("disable user: %v", err)
		}
		if _, err := svc.Resolve(token.Token); err == nil {
			t.Fatal("expected an inactive token creator to be rejected")
		}
	})

	t.Run("suspended tenant", func(t *testing.T) {
		db := setupTestDB(t)
		user := createTestUser(t, db, "suspended-tenant-token-user", "admin")
		prepareResolvableTokenUser(t, db, user, models.TenantRoleAdmin)
		svc := NewTokenService(db)
		token := createBoundToken(t, svc, user.ID, models.DefaultTenantID, []string{permissions.ArticlesRead})

		if err := db.Model(&models.Tenant{}).Where("id = ?", models.DefaultTenantID).
			Update("status", models.TenantStatusSuspended).Error; err != nil {
			t.Fatalf("suspend tenant: %v", err)
		}
		if _, err := svc.Resolve(token.Token); err == nil {
			t.Fatal("expected a token for a suspended tenant to be rejected")
		}
	})

	t.Run("removed membership", func(t *testing.T) {
		db := setupTestDB(t)
		user := createTestUser(t, db, "removed-membership-token-user", "admin")
		prepareResolvableTokenUser(t, db, user, models.TenantRoleAdmin)
		svc := NewTokenService(db)
		token := createBoundToken(t, svc, user.ID, models.DefaultTenantID, []string{permissions.ArticlesRead})

		if err := db.Where("tenant_id = ? AND user_id = ?", models.DefaultTenantID, user.ID).
			Delete(&models.TenantMembership{}).Error; err != nil {
			t.Fatalf("remove membership: %v", err)
		}
		if _, err := svc.Resolve(token.Token); err == nil {
			t.Fatal("expected a token without a current membership to be rejected")
		}
	})

	t.Run("unknown tenant role", func(t *testing.T) {
		db := setupTestDB(t)
		user := createTestUser(t, db, "unknown-membership-role-user", "admin")
		prepareResolvableTokenUser(t, db, user, models.TenantRoleAdmin)
		svc := NewTokenService(db)
		token := createBoundToken(t, svc, user.ID, models.DefaultTenantID, []string{permissions.ArticlesRead})

		if err := db.Model(&models.TenantMembership{}).
			Where("tenant_id = ? AND user_id = ?", models.DefaultTenantID, user.ID).
			Update("role_slug", "reviewer").Error; err != nil {
			t.Fatalf("corrupt membership role: %v", err)
		}
		if _, err := svc.Resolve(token.Token); err == nil {
			t.Fatal("expected an unknown tenant role to fail closed")
		}
	})

	t.Run("membership exists only in another tenant", func(t *testing.T) {
		db := setupTestDB(t)
		user := createTestUser(t, db, "wrong-membership-tenant-user", "admin")
		prepareResolvableTokenUser(t, db, user, models.TenantRoleAdmin)
		tenantB := createTokenTestTenant(t, db, "resolve-tenant-b", models.TenantStatusActive)
		svc := NewTokenService(db)
		token := createBoundToken(t, svc, user.ID, tenantB.ID, []string{permissions.ArticlesRead})

		if _, err := svc.Resolve(token.Token); err == nil {
			t.Fatal("expected a tenant B token without tenant B membership to be rejected")
		}
	})

	t.Run("legacy token without tenant binding", func(t *testing.T) {
		db := setupTestDB(t)
		user := createTestUser(t, db, "legacy-token-user", "admin")
		raw := "vc_live_legacy_without_tenant"
		legacy := &models.APIToken{
			Name:        "legacy",
			Token:       raw,
			CreatedByID: user.ID,
			IsActive:    true,
		}
		if err := db.Create(legacy).Error; err != nil {
			t.Fatalf("create legacy token: %v", err)
		}
		if _, err := NewTokenService(db).Resolve(raw); err == nil {
			t.Fatal("expected a token without an explicit tenant to be rejected")
		}
	})
}

func TestTokenService_Resolve_UsesEffectiveTenantPermissions(t *testing.T) {
	t.Run("global editor and tenant member are both ceilings", func(t *testing.T) {
		db := setupTestDB(t)
		user := createTestUser(t, db, "effective-editor-member", "editor")
		prepareResolvableTokenUser(t, db, user, models.TenantRoleMember)
		svc := NewTokenService(db)
		token := createBoundToken(t, svc, user.ID, models.DefaultTenantID, []string{permissions.Wildcard})

		principal, err := svc.Resolve(token.Token)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if principal.TenantID != models.DefaultTenantID || principal.UserID != user.ID {
			t.Fatalf("principal identity = user %d tenant %d", principal.UserID, principal.TenantID)
		}
		if principal.IsPlatformAdmin {
			t.Fatal("a global editor with tenant admin/member privileges must not become platform admin")
		}
		if !permissions.Grants(principal.Permissions, permissions.ArticlesRead) {
			t.Fatal("expected articles.read in the effective intersection")
		}
		if permissions.Grants(principal.Permissions, permissions.ArticlesPublish) {
			t.Fatal("tenant member ceiling must remove articles.publish")
		}
		if permissions.Grants(principal.Permissions, permissions.UsersDelete) {
			t.Fatal("platform permissions must not enter a tenant token principal")
		}
	})

	t.Run("platform admin remains explicit but tenant-bound", func(t *testing.T) {
		db := setupTestDB(t)
		user := createTestUser(t, db, "effective-platform-admin", "admin")
		prepareResolvableTokenUser(t, db, user, models.TenantRoleAdmin)
		svc := NewTokenService(db)
		token := createBoundToken(t, svc, user.ID, models.DefaultTenantID, []string{permissions.Wildcard})

		principal, err := svc.Resolve(token.Token)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if !principal.IsPlatformAdmin {
			t.Fatal("expected the global admin identity marker to remain explicit")
		}
		if !permissions.Grants(principal.Permissions, permissions.ArticlesPublish) {
			t.Fatal("expected an allowed tenant permission for tenant admin")
		}
		if permissions.Grants(principal.Permissions, permissions.UsersDelete) {
			t.Fatal("a platform admin API token must not implicitly carry platform-wide permissions")
		}
	})

	t.Run("tenant admin does not raise global author ceiling", func(t *testing.T) {
		db := setupTestDB(t)
		user := createTestUser(t, db, "effective-tenant-admin", "author")
		prepareResolvableTokenUser(t, db, user, models.TenantRoleAdmin)
		svc := NewTokenService(db)
		token := createBoundToken(t, svc, user.ID, models.DefaultTenantID, []string{permissions.Wildcard})

		principal, err := svc.Resolve(token.Token)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if principal.IsPlatformAdmin {
			t.Fatal("tenant admin membership must not mark an author as platform admin")
		}
		if permissions.Grants(principal.Permissions, permissions.ArticlesPublish) {
			t.Fatal("tenant admin membership must not exceed the author's global role")
		}
	})
}

func TestTokenService_Validate_InsufficientPermission(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "insufficientuser", "admin")
	prepareResolvableTokenUser(t, db, user, models.TenantRoleAdmin)
	svc := NewTokenService(db)

	resp, _ := svc.Create(CreateTokenRequest{
		Name:        "limited",
		Permissions: []string{"articles.read"},
	}, user.ID, models.DefaultTenantID)

	valid, _, err := svc.Validate(resp.Token, "articles.delete")
	if err == nil {
		t.Fatal("expected error for insufficient permission")
	}
	if valid {
		t.Fatal("expected invalid for insufficient permission")
	}
}

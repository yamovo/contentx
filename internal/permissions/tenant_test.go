package permissions

import (
	"reflect"
	"testing"

	"github.com/yamovo/contentx/internal/models"
)

func TestNormalizeTenantRole(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{models.TenantRoleAdmin, models.TenantRoleAdmin, true},
		{models.TenantRoleEditor, models.TenantRoleEditor, true},
		{models.TenantRoleMember, models.TenantRoleMember, true},
		{"author", models.TenantRoleMember, true},
		{"subscriber", models.TenantRoleMember, true},
		{"reviewer", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := NormalizeTenantRole(tt.input)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("NormalizeTenantRole(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestTenantRoleGrants_IsCeilingAndNeverPlatformGrant(t *testing.T) {
	if !TenantRoleGrants(models.TenantRoleMember, ArticlesCreate) {
		t.Fatal("member should inherit the author create ceiling")
	}
	if TenantRoleGrants(models.TenantRoleMember, ArticlesPublish) {
		t.Fatal("member must not publish")
	}
	if !TenantRoleGrants(models.TenantRoleEditor, ArticlesPublish) {
		t.Fatal("editor should be allowed to publish")
	}
	if !TenantRoleGrants(models.TenantRoleAdmin, SettingsUpdate) {
		t.Fatal("tenant admin should be allowed a tenant-scoped settings update")
	}
	if TenantRoleGrants(models.TenantRoleAdmin, UsersDelete) {
		t.Fatal("tenant admin must never receive platform user administration")
	}
	if TenantRoleGrants("unknown", ArticlesRead) {
		t.Fatal("unknown tenant roles must fail closed")
	}
}

func TestEffectiveForTenant_IntersectsTokenGlobalAndTenantPermissions(t *testing.T) {
	user := &models.User{Role: models.Role{
		Slug: "editor",
		Permissions: []models.Permission{
			{Slug: ArticlesRead},
			{Slug: ArticlesPublish},
			{Slug: UsersRead},
		},
	}}

	got, ok := EffectiveForTenant(user, []string{Wildcard}, models.TenantRoleMember)
	if !ok {
		t.Fatal("expected a recognized tenant role")
	}
	want := []string{ArticlesRead}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EffectiveForTenant() = %v, want %v", got, want)
	}

	if _, ok := EffectiveForTenant(user, []string{"unknown.permission"}, models.TenantRoleAdmin); ok {
		t.Fatal("unknown stored token permissions must invalidate the principal")
	}
	if _, ok := EffectiveForTenant(user, []string{ArticlesRead}, "reviewer"); ok {
		t.Fatal("unknown tenant roles must invalidate the principal")
	}
}

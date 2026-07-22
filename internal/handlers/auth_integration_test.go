package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/services"
)

// setupAuthTestRouter initializes a test gin engine with the auth routes registered.
func setupAuthTestRouter(t *testing.T) (*gin.Engine, *gorm.DB, *auth.JWTManager) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.Seed(db)

	cfg := &config.Config{}
	cfg.JWT.Secret = "test-jwt-secret-for-integration-testing-32chars"
	cfg.JWT.AccessTokenTTL = 3600000000000
	cfg.JWT.RefreshTokenTTL = 86400000000000
	cfg.JWT.Issuer = "contentx-test"
	cfg.Server.BaseURL = "http://localhost:8080"

	jwtMgr := auth.NewJWTManager(cfg.JWT)
	blacklist := auth.NewBlacklist()
	guard := auth.NewLoginGuard()

	authSvc := services.NewAuthService(db, jwtMgr, blacklist, guard)

	r := gin.New()
	api := r.Group("/api/v1")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/login", NewAuthHandler(authSvc).Login)
			authGroup.POST("/register", NewAuthHandler(authSvc).Register)
			authGroup.POST("/refresh", NewAuthHandler(authSvc).RefreshToken)
		}
		protected := api.Group("")
		protected.Use(mockAuthMiddleware(jwtMgr, db))
		{
			protected.POST("/auth/logout", NewAuthHandler(authSvc).Logout)
			protected.GET("/auth/me", NewAuthHandler(authSvc).Me)
		}
	}

	return r, db, jwtMgr
}

// mockAuthMiddleware is a simplified auth middleware for integration tests.
// It validates the JWT and loads the user from the DB so that getCurrentUser works.
func mockAuthMiddleware(jwtMgr *auth.JWTManager, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" || !strings.HasPrefix(token, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": -1, "message": "unauthorized"})
			return
		}
		claims, err := jwtMgr.ValidateToken(token[7:])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": -1, "message": "invalid token"})
			return
		}
		// Load user from DB so getCurrentUser works.
		var user models.User
		if err := db.Preload("Role").First(&user, claims.UserID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": -1, "message": "user not found"})
			return
		}
		c.Set("currentUser", &user)
		c.Set("claims", claims)
		c.Next()
	}
}

// generateTestJWT returns a valid JWT access token for a given user.
func generateTestJWT(t *testing.T, jwtMgr *auth.JWTManager, user models.User) string {
	t.Helper()
	pair, err := jwtMgr.GenerateTokenPair(user.ID, user.Username, user.Email, "admin", user.DisplayName)
	if err != nil {
		t.Fatalf("generate JWT: %v", err)
	}
	return pair.AccessToken
}

// createTestUserDB creates a user directly in the test DB.
func createTestUserDB(t *testing.T, db *gorm.DB, username, roleSlug string) *models.User {
	t.Helper()

	var role models.Role
	db.Where("slug = ?", roleSlug).First(&role)

	hash, err := auth.HashPassword("TestPass1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Username:    username,
		Email:       username + "@test.com",
		Password:    hash,
		DisplayName: username,
		RoleID:      role.ID,
		Status:      models.UserStatusActive,
	}
	db.Create(&user)
	db.Preload("Role").First(&user, user.ID)
	return &user
}

// ─── Login Tests ─────────────────────────────────────────────────────────────

func TestAuth_Login_Success(t *testing.T) {
	r, db, _ := setupAuthTestRouter(t)
	createTestUserDB(t, db, "testadmin", "admin")

	body := `{"username":"testadmin","password":"TestPass1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}
	data := resp.Data.(map[string]interface{})
	if data["token"] == nil {
		t.Fatal("expected token in response")
	}
}

func TestAuth_Login_WrongPassword(t *testing.T) {
	r, db, _ := setupAuthTestRouter(t)
	createTestUserDB(t, db, "testadmin", "admin")

	body := `{"username":"testadmin","password":"WrongPass999"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_Login_MissingFields(t *testing.T) {
	r, _, _ := setupAuthTestRouter(t)

	body := `{"username":"testadmin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ─── Register Tests ──────────────────────────────────────────────────────────

func TestAuth_Register_Success(t *testing.T) {
	r, _, _ := setupAuthTestRouter(t)

	body := `{"username":"newuser","email":"new@test.com","password":"SecurePass123","display_name":"New User"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	if data["token"] == nil {
		t.Fatal("expected token in register response")
	}
}

func TestAuth_Register_DuplicateUsername(t *testing.T) {
	r, db, _ := setupAuthTestRouter(t)
	createTestUserDB(t, db, "existing", "admin")

	body := `{"username":"existing","email":"another@test.com","password":"SecurePass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuth_Register_WeakPassword(t *testing.T) {
	r, _, _ := setupAuthTestRouter(t)

	body := `{"username":"weakpass","email":"weak@test.com","password":"123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for weak password, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── Auth Endpoint Tests ─────────────────────────────────────────────────────

func TestAuth_Me_Authenticated(t *testing.T) {
	r, db, jwtMgr := setupAuthTestRouter(t)
	user := createTestUserDB(t, db, "authed", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuth_Me_Unauthenticated(t *testing.T) {
	r, _, _ := setupAuthTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_RefreshToken_Success(t *testing.T) {
	r, db, jwtMgr := setupAuthTestRouter(t)
	user := createTestUserDB(t, db, "refreshuser", "admin")
	pair, _ := jwtMgr.GenerateTokenPair(user.ID, user.Username, user.Email, "admin", user.DisplayName)

	body := fmt.Sprintf(`{"refresh_token":"%s"}`, pair.RefreshToken)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for refresh, got %d: %s", w.Code, w.Body.String())
	}
}

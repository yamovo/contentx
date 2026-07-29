package services

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
)

// genTestTOTP replicates RFC 6238 (6 digits / 30s period, HMAC-SHA1) so tests
// can produce a currently-valid code for a known secret.
func genTestTOTP(t *testing.T, secret string) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	counter := uint64(time.Now().Unix()) / 30
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0F
	code := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7FFFFFFF
	return fmt.Sprintf("%06d", code%1000000)
}

// setupEnabledTOTP walks a user through setup+enable and returns the secret
// and the backup codes.
func setupEnabledTOTP(t *testing.T, svc *TOTPService, userID uint) (string, []string) {
	t.Helper()
	resp, err := svc.Setup(userID, "tester")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	codes, err := svc.Enable(userID, genTestTOTP(t, resp.Secret))
	if err != nil {
		t.Fatalf("Enable failed: %v", err)
	}
	return resp.Secret, codes
}

// ─── Setup / Enable / Status ────────────────────────────────────────────────

func TestTOTP_Setup_ReturnsSecretAndURI(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "totpuser", "author")
	svc := NewTOTPService(db)

	resp, err := svc.Setup(user.ID, user.Username)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	if resp.Secret == "" {
		t.Error("expected non-empty secret")
	}
	if !strings.HasPrefix(resp.OtpauthURI, "otpauth://totp/") {
		t.Errorf("unexpected otpauth URI: %s", resp.OtpauthURI)
	}
	// Setup alone must not enable TOTP.
	enabled, err := svc.Status(user.ID)
	if err != nil || enabled {
		t.Errorf("expected disabled after setup, got enabled=%v err=%v", enabled, err)
	}
}

func TestTOTP_Enable_WrongCodeRejected(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "totpuser", "author")
	svc := NewTOTPService(db)

	if _, err := svc.Setup(user.ID, user.Username); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	if _, err := svc.Enable(user.ID, "000000"); err != errs.ErrTOTPInvalid {
		t.Fatalf("expected ErrTOTPInvalid, got %v", err)
	}
}

func TestTOTP_Enable_ValidCodeActivatesAndReturnsBackupCodes(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "totpuser", "author")
	svc := NewTOTPService(db)

	_, codes := setupEnabledTOTP(t, svc, user.ID)
	if len(codes) != totpBackupCodeCount {
		t.Errorf("expected %d backup codes, got %d", totpBackupCodeCount, len(codes))
	}
	enabled, _ := svc.Status(user.ID)
	if !enabled {
		t.Error("expected TOTP enabled")
	}
}

func TestTOTP_Enable_WithoutSetupRejected(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "totpuser", "author")
	svc := NewTOTPService(db)

	if _, err := svc.Enable(user.ID, "123456"); err == nil {
		t.Fatal("expected error enabling without setup")
	}
}

func TestTOTP_Setup_WhileEnabledRejected(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "totpuser", "author")
	svc := NewTOTPService(db)

	setupEnabledTOTP(t, svc, user.ID)
	if _, err := svc.Setup(user.ID, user.Username); err == nil {
		t.Fatal("expected conflict when re-running setup while enabled")
	}
}

// ─── VerifyLogin ────────────────────────────────────────────────────────────

func TestTOTP_VerifyLogin_ValidCode(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "totpuser", "author")
	svc := NewTOTPService(db)

	secret, _ := setupEnabledTOTP(t, svc, user.ID)
	if err := svc.VerifyLogin(user.ID, genTestTOTP(t, secret)); err != nil {
		t.Fatalf("expected valid code to pass, got %v", err)
	}
}

func TestTOTP_VerifyLogin_WrongCode(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "totpuser", "author")
	svc := NewTOTPService(db)

	setupEnabledTOTP(t, svc, user.ID)
	if err := svc.VerifyLogin(user.ID, "000000"); err != errs.ErrTOTPInvalid {
		t.Fatalf("expected ErrTOTPInvalid, got %v", err)
	}
}

func TestTOTP_VerifyLogin_BackupCodeConsumedOnce(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "totpuser", "author")
	svc := NewTOTPService(db)

	_, codes := setupEnabledTOTP(t, svc, user.ID)
	if err := svc.VerifyLogin(user.ID, codes[0]); err != nil {
		t.Fatalf("backup code should pass once, got %v", err)
	}
	if err := svc.VerifyLogin(user.ID, codes[0]); err != errs.ErrTOTPInvalid {
		t.Fatalf("reused backup code should fail, got %v", err)
	}
	// A different, unused backup code still works.
	if err := svc.VerifyLogin(user.ID, codes[1]); err != nil {
		t.Fatalf("second backup code should pass, got %v", err)
	}
}

// ─── Disable ────────────────────────────────────────────────────────────────

func TestTOTP_Disable_WrongPasswordRejected(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "totpuser", "author")
	svc := NewTOTPService(db)

	setupEnabledTOTP(t, svc, user.ID)
	if err := svc.Disable(user.ID, "wrong-password"); err == nil {
		t.Fatal("expected error for wrong password")
	}
	enabled, _ := svc.Status(user.ID)
	if !enabled {
		t.Error("TOTP should remain enabled after failed disable")
	}
}

func TestTOTP_Disable_CorrectPassword(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "totpuser", "author")
	svc := NewTOTPService(db)

	setupEnabledTOTP(t, svc, user.ID)
	// createTestUser hashes the fixed password "TestPass1".
	if err := svc.Disable(user.ID, "TestPass1"); err != nil {
		t.Fatalf("Disable failed: %v", err)
	}
	enabled, _ := svc.Status(user.ID)
	if enabled {
		t.Error("expected TOTP disabled")
	}
}

// ─── Login integration (AuthService + TOTPService) ──────────────────────────

func TestTOTP_Login_RequiresCodeWhenEnabled(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "totplogin", "author")
	totpSvc := NewTOTPService(db)
	secret, codes := setupEnabledTOTP(t, totpSvc, user.ID)

	authSvc := NewAuthService(db, newTestJWTManager(), nil, nil)
	authSvc.SetTOTPVerifier(totpSvc)
	audit := &captureAuditLogger{}
	authSvc.SetAuditLogger(audit)

	// Missing code → TOTP_REQUIRED, no tokens issued.
	_, _, err := authSvc.LoginWithTOTP(user.Username, "TestPass1", "", "127.0.0.1", "test")
	if err != errs.ErrTOTPRequired {
		t.Fatalf("expected ErrTOTPRequired, got %v", err)
	}
	ev := audit.findAction("login.failed")
	if ev == nil {
		t.Fatalf("expected login.failed audit event, got: %+v", audit.Events())
	}
	details, ok := ev.Details.(map[string]any)
	if !ok || details["reason"] != "totp_required" {
		t.Fatalf("login.failed reason = %v, want totp_required", ev.Details)
	}
	if ev.UserID == nil || *ev.UserID != user.ID {
		t.Fatalf("login.failed UserID = %v, want %d", ev.UserID, user.ID)
	}

	// Wrong code → TOTP_INVALID.
	_, _, err = authSvc.LoginWithTOTP(user.Username, "TestPass1", "000000", "127.0.0.1", "test")
	if err != errs.ErrTOTPInvalid {
		t.Fatalf("expected ErrTOTPInvalid, got %v", err)
	}

	// Valid TOTP code → success.
	pair, safeUser, err := authSvc.LoginWithTOTP(user.Username, "TestPass1", genTestTOTP(t, secret), "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("login with valid code failed: %v", err)
	}
	if pair == nil || pair.AccessToken == "" || safeUser == nil {
		t.Fatal("expected token pair and user")
	}

	// Backup code also works for login.
	if _, _, err := authSvc.LoginWithTOTP(user.Username, "TestPass1", codes[0], "127.0.0.1", "test"); err != nil {
		t.Fatalf("login with backup code failed: %v", err)
	}
}

func TestTOTP_Login_UnaffectedWhenDisabled(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "plainlogin", "author")
	authSvc := NewAuthService(db, newTestJWTManager(), nil, nil)
	authSvc.SetTOTPVerifier(NewTOTPService(db))

	// No TOTP row at all → plain password login keeps working (degraded path).
	pair, _, err := authSvc.Login(user.Username, "TestPass1", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("plain login failed: %v", err)
	}
	if pair == nil || pair.AccessToken == "" {
		t.Fatal("expected token pair")
	}

	// Sanity: table exists and stays empty for this user.
	var count int64
	db.Model(&models.UserTOTP{}).Where("user_id = ?", user.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected no totp rows, got %d", count)
	}
}

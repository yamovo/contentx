package services

import (
	"strings"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

const totpBackupCodeCount = 10

// TOTPSetupResponse is returned by Setup: the secret and otpauth URI are shown
// once so the user can register the account in an authenticator app.
type TOTPSetupResponse struct {
	Secret     string `json:"secret"`
	OtpauthURI string `json:"otpauth_uri"`
}

// TOTPService manages per-user TOTP two-factor authentication state.
// It also implements the auth-time verification hook used by AuthService.
type TOTPService struct {
	db *gorm.DB
}

// NewTOTPService creates a TOTPService.
func NewTOTPService(db *gorm.DB) *TOTPService {
	return &TOTPService{db: db}
}

// Status reports whether TOTP is enabled for the user.
func (s *TOTPService) Status(userID uint) (bool, error) {
	var rec models.UserTOTP
	if err := s.db.Where("user_id = ?", userID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, errs.ErrInternal.Wrap(err)
	}
	return rec.Enabled, nil
}

// Setup generates a fresh secret for the user and stores it in the pending
// (disabled) state. Rejected while TOTP is already enabled — the user must
// disable first, so a session hijacker cannot silently rotate the secret.
func (s *TOTPService) Setup(userID uint, account string) (*TOTPSetupResponse, error) {
	var rec models.UserTOTP
	err := s.db.Where("user_id = ?", userID).First(&rec).Error
	switch {
	case err == nil:
		if rec.Enabled {
			return nil, errs.ErrConflict.WithMessage("two-factor authentication is already enabled")
		}
	case err != gorm.ErrRecordNotFound:
		return nil, errs.ErrInternal.Wrap(err)
	}

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}

	rec.UserID = userID
	rec.Secret = secret
	rec.Enabled = false
	rec.BackupCodes = ""
	if err := s.db.Save(&rec).Error; err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}

	return &TOTPSetupResponse{
		Secret:     secret,
		OtpauthURI: auth.GenerateTOTPURI(secret, account),
	}, nil
}

// Enable confirms the pending secret with a valid code, activates TOTP, and
// returns freshly generated one-time backup codes (shown only once).
func (s *TOTPService) Enable(userID uint, code string) ([]string, error) {
	var rec models.UserTOTP
	if err := s.db.Where("user_id = ?", userID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.ErrBadRequest.WithMessage("run setup before enabling two-factor authentication")
		}
		return nil, errs.ErrInternal.Wrap(err)
	}
	if rec.Enabled {
		return nil, errs.ErrConflict.WithMessage("two-factor authentication is already enabled")
	}
	if !auth.ValidateTOTP(rec.Secret, code) {
		return nil, errs.ErrTOTPInvalid
	}

	codes, err := auth.GenerateBackupCodes(totpBackupCodeCount)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	hashed := make([]string, len(codes))
	for i, c := range codes {
		hashed[i] = auth.HashBackupCode(c)
	}

	rec.Enabled = true
	rec.BackupCodes = strings.Join(hashed, ",")
	if err := s.db.Save(&rec).Error; err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return codes, nil
}

// Disable turns off TOTP after re-verifying the user's password, so a stolen
// session alone cannot remove the second factor.
func (s *TOTPService) Disable(userID uint, password string) error {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if err := auth.CheckPassword(user.Password, password); err != nil {
		return errs.ErrInvalidCreds.WithMessage("password verification failed")
	}
	if err := s.db.Where("user_id = ?", userID).Delete(&models.UserTOTP{}).Error; err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

// ─── Login-time verification (used by AuthService) ─────────────────────────

// Required reports whether the user must supply a second factor at login.
// Status errors are returned to the login service so authentication fails
// closed instead of silently treating an unreadable TOTP record as disabled.
func (s *TOTPService) Required(userID uint) (bool, error) {
	return s.Status(userID)
}

// VerifyLogin checks a TOTP code (or consumes a one-time backup code) during
// login. Returns errs.ErrTOTPInvalid when neither matches.
func (s *TOTPService) VerifyLogin(userID uint, code string) error {
	var rec models.UserTOTP
	if err := s.db.Where("user_id = ? AND enabled = ?", userID, true).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil // not enabled — nothing to verify
		}
		return errs.ErrInternal.Wrap(err)
	}

	if auth.ValidateTOTP(rec.Secret, code) {
		return nil
	}

	// Fall back to one-time backup codes; consume on success.
	if rec.BackupCodes != "" {
		hash := auth.HashBackupCode(code)
		hashes := strings.Split(rec.BackupCodes, ",")
		for i, h := range hashes {
			if h == hash {
				remaining := append(hashes[:i], hashes[i+1:]...)
				rec.BackupCodes = strings.Join(remaining, ",")
				if err := s.db.Save(&rec).Error; err != nil {
					return errs.ErrInternal.Wrap(err)
				}
				return nil
			}
		}
	}
	return errs.ErrTOTPInvalid
}

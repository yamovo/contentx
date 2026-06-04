package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/vortexcms/go-cms/internal/config"
)

var (
	ErrInvalidToken  = errors.New("invalid or expired token")
	ErrTokenExpired  = errors.New("token has expired")
	ErrTokenRevoked  = errors.New("token has been revoked")
)

// Claims represents JWT claims.
type Claims struct {
	UserID      uint   `json:"user_id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	RoleSlug    string `json:"role"`
	DisplayName string `json:"display_name"`
	jwt.RegisteredClaims
}

// TokenPair contains access and refresh tokens.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	ExpiresIn    int64     `json:"expires_in"` // seconds
}

// JWTManager handles JWT token operations.
type JWTManager struct {
	cfg config.JWTConfig
}

// NewJWTManager creates a new JWT manager.
func NewJWTManager(cfg config.JWTConfig) *JWTManager {
	return &JWTManager{cfg: cfg}
}

// GenerateTokenPair creates both access and refresh tokens.
func (m *JWTManager) GenerateTokenPair(userID uint, username, email, roleSlug, displayName string) (*TokenPair, error) {
	now := time.Now()

	// Access token.
	accessClaims := &Claims{
		UserID:      userID,
		Username:    username,
		Email:       email,
		RoleSlug:    roleSlug,
		DisplayName: displayName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.cfg.AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    m.cfg.Issuer,
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenStr, err := accessToken.SignedString([]byte(m.cfg.Secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Refresh token.
	refreshClaims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.cfg.RefreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    m.cfg.Issuer,
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenStr, err := refreshToken.SignedString([]byte(m.cfg.Secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenStr,
		RefreshToken: refreshTokenStr,
		TokenType:    "Bearer",
		ExpiresAt:    now.Add(m.cfg.AccessTokenTTL),
		ExpiresIn:    int64(m.cfg.AccessTokenTTL.Seconds()),
	}, nil
}

// ValidateToken validates and parses a JWT token string.
func (m *JWTManager) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.cfg.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// RefreshAccessToken creates a new access token from a valid refresh token.
func (m *JWTManager) RefreshAccessToken(refreshTokenStr string) (*TokenPair, error) {
	claims, err := m.ValidateToken(refreshTokenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// For refresh, we only have UserID; the new access token needs more info.
	// In practice, you'd look up the user here. For now we'll use what's in the token.
	return m.GenerateTokenPair(claims.UserID, claims.Username, claims.Email, claims.RoleSlug, claims.DisplayName)
}

// Blacklist is an in-memory token blacklist (replace with Redis in production).
type Blacklist struct {
	tokens map[string]time.Time
}

// NewBlacklist creates a new token blacklist.
func NewBlacklist() *Blacklist {
	return &Blacklist{
		tokens: make(map[string]time.Time),
	}
}

// Revoke adds a token to the blacklist.
func (b *Blacklist) Revoke(tokenStr string, expiresAt time.Time) {
	b.tokens[tokenStr] = expiresAt
}

// IsRevoked checks if a token has been revoked.
func (b *Blacklist) IsRevoked(tokenStr string) bool {
	exp, ok := b.tokens[tokenStr]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(b.tokens, tokenStr)
		return false
	}
	return true
}

// Cleanup removes expired tokens from the blacklist.
func (b *Blacklist) Cleanup() {
	now := time.Now()
	for token, exp := range b.tokens {
		if now.After(exp) {
			delete(b.tokens, token)
		}
	}
}

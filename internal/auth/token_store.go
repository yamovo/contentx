package auth

import (
	"errors"
	"time"
)

// ErrTokenStoreUnavailable is returned when a one-time token cannot be
// consumed safely because the shared revocation store is unavailable.
var ErrTokenStoreUnavailable = errors.New("token revocation store unavailable")

// TokenStore is the interface for token revocation storage.
// Implementations must be safe for concurrent use.
type TokenStore interface {
	// Revoke adds a token to the revocation set, expiring at the given time.
	Revoke(tokenStr string, expiresAt time.Time)
	// IsRevoked returns true if the token has been revoked and has not yet expired.
	IsRevoked(tokenStr string) bool
	// Consume atomically marks a one-time token as used. It returns false when
	// the token was already consumed/revoked. Implementations backed by a
	// shared store must return an error rather than fail open when atomicity
	// cannot be guaranteed.
	Consume(tokenStr string, expiresAt time.Time) (bool, error)
}

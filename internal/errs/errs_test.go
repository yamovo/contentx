package errs

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

// TestPredefinedErrors_CodeStatusMapping verifies every sentinel error keeps
// its stable err_code and the HTTP status the frontend relies on.
func TestPredefinedErrors_CodeStatusMapping(t *testing.T) {
	cases := []struct {
		err        *AppError
		wantCode   string
		wantStatus int
	}{
		{ErrBadRequest, "BAD_REQUEST", http.StatusBadRequest},
		{ErrValidation, "VALIDATION_ERROR", http.StatusUnprocessableEntity},
		{ErrUnauthorized, "UNAUTHORIZED", http.StatusUnauthorized},
		{ErrInvalidCreds, "INVALID_CREDENTIALS", http.StatusUnauthorized},
		{ErrTokenExpired, "TOKEN_EXPIRED", http.StatusUnauthorized},
		{ErrTokenRevoked, "TOKEN_REVOKED", http.StatusUnauthorized},
		{ErrForbidden, "FORBIDDEN", http.StatusForbidden},
		{ErrAccountLocked, "ACCOUNT_LOCKED", http.StatusForbidden},
		{ErrAccountDisabled, "ACCOUNT_DISABLED", http.StatusForbidden},
		{ErrNotFound, "NOT_FOUND", http.StatusNotFound},
		{ErrConflict, "CONFLICT", http.StatusConflict},
		{ErrDuplicateUser, "DUPLICATE_USER", http.StatusConflict},
		{ErrRateLimitExceeded, "RATE_LIMIT_EXCEEDED", http.StatusTooManyRequests},
		{ErrInternal, "INTERNAL_ERROR", http.StatusInternalServerError},
		{ErrServiceUnavailable, "SERVICE_UNAVAILABLE", http.StatusServiceUnavailable},
	}
	seen := make(map[string]bool)
	for _, tc := range cases {
		if tc.err.Code != tc.wantCode {
			t.Errorf("Code = %q, want %q", tc.err.Code, tc.wantCode)
		}
		if tc.err.StatusCode() != tc.wantStatus {
			t.Errorf("%s: StatusCode() = %d, want %d", tc.wantCode, tc.err.StatusCode(), tc.wantStatus)
		}
		if tc.err.Message == "" {
			t.Errorf("%s: Message should not be empty", tc.wantCode)
		}
		if seen[tc.wantCode] {
			t.Errorf("duplicate error code %q", tc.wantCode)
		}
		seen[tc.wantCode] = true
	}
}

func TestAppError_Error(t *testing.T) {
	plain := &AppError{Code: "X_CODE", Message: "msg", Status: 400}
	if plain.Error() != "X_CODE" {
		t.Errorf("Error() = %q, want %q", plain.Error(), "X_CODE")
	}

	wrapped := plain.Wrap(errors.New("db down"))
	if got, want := wrapped.Error(), "X_CODE: db down"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAppError_Wrap_CopiesAndPreservesOriginal(t *testing.T) {
	inner := errors.New("boom")
	wrapped := ErrInternal.Wrap(inner)

	if wrapped == ErrInternal {
		t.Fatal("Wrap must return a copy, not mutate the sentinel")
	}
	if ErrInternal.Err != nil {
		t.Fatal("sentinel ErrInternal must stay untouched")
	}
	if wrapped.Code != ErrInternal.Code || wrapped.Status != ErrInternal.Status {
		t.Error("Wrap must preserve Code and Status")
	}
	if !errors.Is(wrapped, inner) {
		t.Error("errors.Is should find the wrapped inner error via Unwrap")
	}
}

func TestAppError_WithMessage_CopiesAndPreservesOriginal(t *testing.T) {
	custom := ErrNotFound.WithMessage("article not found")

	if custom == ErrNotFound {
		t.Fatal("WithMessage must return a copy, not mutate the sentinel")
	}
	if ErrNotFound.Message != "Resource not found" {
		t.Fatalf("sentinel message mutated: %q", ErrNotFound.Message)
	}
	if custom.Message != "article not found" {
		t.Errorf("Message = %q, want %q", custom.Message, "article not found")
	}
	if custom.Code != ErrNotFound.Code || custom.Status != ErrNotFound.Status {
		t.Error("WithMessage must preserve Code and Status")
	}
}

func TestNew(t *testing.T) {
	e := New("ARTICLE_LOCKED", "article is being edited", http.StatusConflict)
	if e.Code != "ARTICLE_LOCKED" || e.Message != "article is being edited" || e.StatusCode() != http.StatusConflict {
		t.Errorf("New() built unexpected error: %+v", e)
	}
}

func TestIs_DirectAppError(t *testing.T) {
	var appErr *AppError
	if !Is(ErrForbidden, &appErr) {
		t.Fatal("Is should match a direct *AppError")
	}
	if appErr != ErrForbidden {
		t.Error("target should be assigned the matched error")
	}
}

func TestIs_SingleWrapChain(t *testing.T) {
	err := fmt.Errorf("outer: %w", fmt.Errorf("mid: %w", ErrTokenExpired))
	var appErr *AppError
	if !Is(err, &appErr) {
		t.Fatal("Is should unwrap fmt.Errorf %w chains")
	}
	if appErr.Code != "TOKEN_EXPIRED" {
		t.Errorf("Code = %q, want TOKEN_EXPIRED", appErr.Code)
	}
}

func TestIs_JoinedErrors(t *testing.T) {
	err := errors.Join(errors.New("plain"), ErrConflict, errors.New("other"))
	var appErr *AppError
	if !Is(err, &appErr) {
		t.Fatal("Is should search errors.Join groups")
	}
	if appErr.Code != "CONFLICT" {
		t.Errorf("Code = %q, want CONFLICT", appErr.Code)
	}
}

func TestIs_NoMatch(t *testing.T) {
	var appErr *AppError
	if Is(nil, &appErr) {
		t.Error("Is(nil) should be false")
	}
	if Is(errors.New("plain error"), &appErr) {
		t.Error("Is should not match a non-AppError")
	}
	if Is(fmt.Errorf("wrapped: %w", errors.New("still plain")), &appErr) {
		t.Error("Is should not match a chain without AppError")
	}
}

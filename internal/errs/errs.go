// Package errs 定义了应用统一的错误体系。
//
// # 错误码枚举
//
// 所有错误通过 *AppError 携带稳定的字符串错误码（Code 字段），前端通过
// APIResponse.err_code 字段获取，可做 switch(err_code) 精细化处理。
//
//	通用错误码（按 HTTP 状态分组）：
//
//	400 Bad Request:
//	  - BAD_REQUEST         通用请求错误
//	  - VALIDATION_ERROR    字段校验失败（422）
//
//	401 Unauthorized:
//	  - UNAUTHORIZED        未认证
//	  - INVALID_CREDENTIALS 用户名或密码错误
//	  - TOKEN_EXPIRED       令牌已过期
//	  - TOKEN_REVOKED       令牌已吊销
//
//	403 Forbidden:
//	  - FORBIDDEN           权限不足
//	  - ACCOUNT_LOCKED      账户锁定
//	  - ACCOUNT_DISABLED    账户已禁用
//
//	404 Not Found:
//	  - NOT_FOUND           资源不存在
//
//	409 Conflict:
//	  - CONFLICT            资源已存在
//	  - DUPLICATE_USER      用户名或邮箱重复
//
//	429 Too Many Requests:
//	  - RATE_LIMIT_EXCEEDED 频率超限
//
//	500 Internal Server Error:
//	  - INTERNAL_ERROR      内部错误
//
//	503 Service Unavailable:
//	  - SERVICE_UNAVAILABLE 服务不可用
//
// 业务自定义错误码（通过 errs.New 创建）：
//   - CREATE_TYPE_FAILED       创建内容类型失败
//   - CREATE_ENTRY_FAILED      创建内容条目失败
//   - UPDATE_ENTRY_FAILED      更新内容条目失败
//   - PUBLISH_ENTRY_FAILED     发布内容条目失败
//   - UNPUBLISH_ENTRY_FAILED   取消发布内容条目失败
//
// # 使用方式
//
//	service 层返回错误：
//	  return errs.ErrNotFound.WithMessage("article not found")
//	  return errs.New("ARTICLE_LOCKED", "article is being edited", http.StatusConflict)
//
//	handler 层转换（handleServiceError 自动处理）：
//	  if ok := errs.Is(err, &appErr); ok {
//	      Error(c, appErr.StatusCode(), appErr.Code, appErr.Message)
//	  }
package errs

import (
	"fmt"
	"net/http"
)

// AppError represents a structured application error with an error code.
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
	Err     error  `json:"-"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	return e.Code
}

// Unwrap supports errors.Is/As.
func (e *AppError) Unwrap() error {
	return e.Err
}

// StatusCode returns the HTTP status code for this error.
func (e *AppError) StatusCode() int {
	return e.Status
}

// Wrap wraps an underlying error into an AppError.
func (e *AppError) Wrap(err error) *AppError {
	return &AppError{
		Code:    e.Code,
		Message: e.Message,
		Status:  e.Status,
		Err:     err,
	}
}

// WithMessage returns a copy with a custom message.
func (e *AppError) WithMessage(msg string) *AppError {
	return &AppError{
		Code:    e.Code,
		Message: msg,
		Status:  e.Status,
		Err:     e.Err,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Pre-defined errors
// ──────────────────────────────────────────────────────────────────────────────

var (
	// 400 Bad Request
	ErrBadRequest = &AppError{Code: "BAD_REQUEST", Message: "Bad request", Status: http.StatusBadRequest}
	ErrValidation = &AppError{Code: "VALIDATION_ERROR", Message: "Validation failed", Status: http.StatusUnprocessableEntity}

	// 401 Unauthorized
	ErrUnauthorized = &AppError{Code: "UNAUTHORIZED", Message: "Authentication required", Status: http.StatusUnauthorized}
	ErrInvalidCreds = &AppError{Code: "INVALID_CREDENTIALS", Message: "Invalid username or password", Status: http.StatusUnauthorized}
	ErrTokenExpired = &AppError{Code: "TOKEN_EXPIRED", Message: "Token has expired", Status: http.StatusUnauthorized}
	ErrTokenRevoked = &AppError{Code: "TOKEN_REVOKED", Message: "Token has been revoked", Status: http.StatusUnauthorized}

	// 403 Forbidden
	ErrForbidden       = &AppError{Code: "FORBIDDEN", Message: "Insufficient permissions", Status: http.StatusForbidden}
	ErrAccountLocked   = &AppError{Code: "ACCOUNT_LOCKED", Message: "Account temporarily locked due to too many failed attempts", Status: http.StatusForbidden}
	ErrAccountDisabled = &AppError{Code: "ACCOUNT_DISABLED", Message: "Account is disabled", Status: http.StatusForbidden}

	// 404 Not Found
	ErrNotFound = &AppError{Code: "NOT_FOUND", Message: "Resource not found", Status: http.StatusNotFound}

	// 409 Conflict
	ErrConflict      = &AppError{Code: "CONFLICT", Message: "Resource already exists", Status: http.StatusConflict}
	ErrDuplicateUser = &AppError{Code: "DUPLICATE_USER", Message: "Username or email already exists", Status: http.StatusConflict}

	// 429 Too Many Requests
	ErrRateLimitExceeded = &AppError{Code: "RATE_LIMIT_EXCEEDED", Message: "Too many requests", Status: http.StatusTooManyRequests}

	// 500 Internal Server Error
	ErrInternal = &AppError{Code: "INTERNAL_ERROR", Message: "Internal server error", Status: http.StatusInternalServerError}

	// 503 Service Unavailable
	ErrServiceUnavailable = &AppError{Code: "SERVICE_UNAVAILABLE", Message: "Service temporarily unavailable", Status: http.StatusServiceUnavailable}
)

// New creates a new AppError with the given code and message.
func New(code string, message string, status int) *AppError {
	return &AppError{Code: code, Message: message, Status: status}
}

// Is checks if err is an *AppError and assigns it to target.
// It unwraps both single-error chains (fmt.Errorf %w) and multi-error
// groups (errors.Join).
// Usage: if ok := errs.Is(err, &appErr); ok { ... }
func Is(err error, target **AppError) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*AppError); ok {
		*target = e
		return true
	}
	// Multi-error (errors.Join): check each wrapped error.
	if u, ok := err.(interface{ Unwrap() []error }); ok {
		for _, sub := range u.Unwrap() {
			if Is(sub, target) {
				return true
			}
		}
		return false
	}
	// Single-error chain.
	if e, ok := err.(interface{ Unwrap() error }); ok {
		return Is(e.Unwrap(), target)
	}
	return false
}

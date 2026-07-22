package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/yamovo/contentx/internal/errs"
	"gorm.io/gorm"
)

// codeFromStatus maps an HTTP status code to the canonical string error code
// used by the legacy statusCoder branch. This ensures that even errors that
// don't use *errs.AppError still emit a stable err_code to the client.
func codeFromStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusTooManyRequests:
		return "RATE_LIMIT_EXCEEDED"
	case http.StatusInternalServerError:
		return "INTERNAL_ERROR"
	case http.StatusServiceUnavailable:
		return "SERVICE_UNAVAILABLE"
	default:
		return "ERROR"
	}
}

// handleServiceError maps service errors to HTTP responses using the unified APIResponse format.
func handleServiceError(c *gin.Context, err error) {
	// Check for AppError first.
	var appErr *errs.AppError
	if ok := errs.Is(err, &appErr); ok {
		Error(c, appErr.StatusCode(), appErr.Code, appErr.Message)
		return
	}

	// Check for statusCoder interface (legacy).
	type statusCoder interface {
		StatusCode() int
	}
	if sc, ok := err.(statusCoder); ok {
		Error(c, sc.StatusCode(), codeFromStatus(sc.StatusCode()), sanitizeMessage(err.Error()))
		return
	}

	// Check for common errors.
	if err == gorm.ErrRecordNotFound {
		NotFound(c, "Resource not found")
		return
	}

	// Log the unexpected error with context.
	slog.Error("unhandled service error",
		"error", err,
		"path", c.Request.URL.Path,
		"method", c.Request.Method,
	)

	InternalError(c)
}

// RegisterJSONTagNameFunc 配置 gin 的 validator，使其在 ValidationErrors 中
// 返回字段的 json tag 名（而非默认的结构体字段 PascalCase 名）。
// 调用时机：gin.New() 之后、注册路由之前。
func RegisterJSONTagNameFunc() {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
}

// sanitizeBindErr 将请求绑定/校验错误转为对前端友好的消息，
// 移除内部结构体名、反射细节、validator 内部格式等敏感信息。
//
// 返回示例：
//   - "请求体为空"（EOF）
//   - "JSON 格式错误"（SyntaxError）
//   - `字段 "title" 类型无效，期望 string`（UnmarshalTypeError）
//   - `字段 "title" 校验失败（规则: required）`（ValidationErrors）
//   - "请求体格式无效"（其他）
func sanitizeBindErr(err error) string {
	if err == nil {
		return "请求体格式无效"
	}

	// EOF: 请求体为空。
	if errors.Is(err, io.EOF) {
		return "请求体为空"
	}

	// JSON 语法错误。
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return "JSON 格式错误"
	}

	// JSON 类型不匹配。
	var unmarshalErr *json.UnmarshalTypeError
	if errors.As(err, &unmarshalErr) {
		if unmarshalErr.Field != "" {
			return fmt.Sprintf("字段 %q 类型无效，期望 %s", unmarshalErr.Field, unmarshalErr.Type.String())
		}
		return fmt.Sprintf("类型无效，期望 %s", unmarshalErr.Type.String())
	}

	// validator 校验错误：仅返回第一个错误的字段名 + 规则。
	var valErrs validator.ValidationErrors
	if errors.As(err, &valErrs) {
		if len(valErrs) == 0 {
			return "参数校验失败"
		}
		fe := valErrs[0]
		return fmt.Sprintf("字段 %q 校验失败（规则: %s）", fe.Field(), fe.Tag())
	}

	// 兜底：返回通用消息，不暴露内部错误细节。
	return "请求体格式无效"
}

// sanitizeMessage 对未分类的错误消息做基础脱敏，
// 移除可能包含文件路径、SQL 语句、内部类型名等敏感信息。
func sanitizeMessage(msg string) string {
	if msg == "" {
		return "Internal server error"
	}
	lower := strings.ToLower(msg)
	// 命中任一敏感模式时整体替换为通用消息。
	for _, pattern := range []string{
		"gorm:", "sql:", "driver:", "pq:", "mysql:", "sqlite:",
		"connection refused", "no such host", "i/o timeout",
		"panic:", "runtime error:", "goroutine",
		"/home/", "/users/", `c:\users`, "c:/users",
	} {
		if strings.Contains(lower, pattern) {
			return "Internal server error"
		}
	}
	return msg
}

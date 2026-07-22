package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/errs"
	"gorm.io/gorm"
)

func jsonTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	return c, w
}

func decodeResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, w.Body.String())
	}
	return body
}

// ---------- Response helpers ----------

func TestSuccess(t *testing.T) {
	c, w := jsonTestContext()
	Success(c, gin.H{"id": 1})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := decodeResponse(t, w)
	if body["code"].(float64) != 0 || body["message"] != "success" {
		t.Fatalf("bad body: %v", body)
	}
	if body["data"] == nil {
		t.Fatal("data missing")
	}
}

func TestSuccessWithMeta(t *testing.T) {
	c, w := jsonTestContext()
	SuccessWithMeta(c, []string{"a"}, &Meta{Page: 1, PageSize: 10, Total: 25, HasNext: true})
	body := decodeResponse(t, w)
	meta, ok := body["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("meta missing: %v", body)
	}
	if meta["total"].(float64) != 25 || meta["has_next"] != true {
		t.Fatalf("bad meta: %v", meta)
	}
}

func TestCreated(t *testing.T) {
	c, w := jsonTestContext()
	Created(c, gin.H{"id": 7})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if decodeResponse(t, w)["message"] != "created" {
		t.Fatal("message should be 'created'")
	}
}

func TestErrorHelpers(t *testing.T) {
	cases := []struct {
		name    string
		fn      func(*gin.Context)
		status  int
		errCode string
	}{
		{"BadRequest", func(c *gin.Context) { BadRequest(c, "bad") }, http.StatusBadRequest, "BAD_REQUEST"},
		{"Unauthorized", func(c *gin.Context) { Unauthorized(c, "no auth") }, http.StatusUnauthorized, "UNAUTHORIZED"},
		{"Forbidden", func(c *gin.Context) { Forbidden(c, "no perm") }, http.StatusForbidden, "FORBIDDEN"},
		{"NotFound", func(c *gin.Context) { NotFound(c, "gone") }, http.StatusNotFound, "NOT_FOUND"},
		{"Conflict", func(c *gin.Context) { Conflict(c, "dup") }, http.StatusConflict, "CONFLICT"},
		{"TooManyRequests", func(c *gin.Context) { TooManyRequests(c, "slow down") }, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED"},
		{"InternalError", func(c *gin.Context) { InternalError(c) }, http.StatusInternalServerError, "INTERNAL_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := jsonTestContext()
			tc.fn(c)
			if w.Code != tc.status {
				t.Fatalf("expected %d, got %d", tc.status, w.Code)
			}
			body := decodeResponse(t, w)
			if body["code"].(float64) != -1 {
				t.Fatalf("error code should be -1: %v", body)
			}
			if body["err_code"] != tc.errCode {
				t.Fatalf("expected err_code %q, got %v", tc.errCode, body["err_code"])
			}
			if body["message"] == "" {
				t.Fatal("message should not be empty")
			}
		})
	}
}

// ---------- handleServiceError ----------

// legacyStatusErr implements only StatusCode() (not AppError) to exercise the
// legacy statusCoder branch.
type legacyStatusErr struct{ code int }

func (e legacyStatusErr) Error() string   { return "legacy failure" }
func (e legacyStatusErr) StatusCode() int { return e.code }

func TestHandleServiceError_AppError(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not found", errs.ErrNotFound.WithMessage("entry not found"), http.StatusNotFound, "NOT_FOUND"},
		{"conflict", errs.ErrConflict.WithMessage("duplicate uid"), http.StatusConflict, "CONFLICT"},
		{"validation", errs.ErrValidation.WithMessage("bad field"), http.StatusUnprocessableEntity, "VALIDATION_ERROR"},
		{"custom", errs.New("CREATE_FAILED", "boom", http.StatusInternalServerError), http.StatusInternalServerError, "CREATE_FAILED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := jsonTestContext()
			handleServiceError(c, tc.err)
			if w.Code != tc.status {
				t.Fatalf("expected %d, got %d", tc.status, w.Code)
			}
			body := decodeResponse(t, w)
			if body["err_code"] != tc.code {
				t.Fatalf("expected err_code %q, got %v", tc.code, body["err_code"])
			}
			if body["message"] == "" {
				t.Fatal("message missing")
			}
		})
	}
}

func TestHandleServiceError_WrappedAppError(t *testing.T) {
	c, w := jsonTestContext()
	wrapped := errors.Join(errors.New("outer"), errs.ErrNotFound)
	handleServiceError(c, wrapped)
	if w.Code != http.StatusNotFound {
		t.Fatalf("wrapped AppError should map to 404, got %d", w.Code)
	}
}

func TestHandleServiceError_LegacyStatusCoder(t *testing.T) {
	c, w := jsonTestContext()
	handleServiceError(c, legacyStatusErr{code: http.StatusTeapot})
	if w.Code != http.StatusTeapot {
		t.Fatalf("expected 418, got %d", w.Code)
	}
	body := decodeResponse(t, w)
	if body["message"] != "legacy failure" {
		t.Fatal("should use err.Error() as message")
	}
	// 418 (Teapot) 不在 codeFromStatus 的映射表中，应回退为 "ERROR"。
	if body["err_code"] != "ERROR" {
		t.Fatalf("expected err_code ERROR for unmapped status, got %v", body["err_code"])
	}
}

func TestHandleServiceError_LegacyStatusCoder_MappedCode(t *testing.T) {
	cases := []struct {
		name   string
		status int
		code   string
	}{
		{"bad request", http.StatusBadRequest, "BAD_REQUEST"},
		{"forbidden", http.StatusForbidden, "FORBIDDEN"},
		{"not found", http.StatusNotFound, "NOT_FOUND"},
		{"conflict", http.StatusConflict, "CONFLICT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := jsonTestContext()
			handleServiceError(c, legacyStatusErr{code: tc.status})
			body := decodeResponse(t, w)
			if body["err_code"] != tc.code {
				t.Fatalf("expected err_code %q, got %v", tc.code, body["err_code"])
			}
		})
	}
}

func TestHandleServiceError_GormNotFound(t *testing.T) {
	c, w := jsonTestContext()
	handleServiceError(c, gorm.ErrRecordNotFound)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if decodeResponse(t, w)["err_code"] != "NOT_FOUND" {
		t.Fatal("gorm.ErrRecordNotFound should map to err_code NOT_FOUND")
	}
}

func TestHandleServiceError_UnknownIs500(t *testing.T) {
	c, w := jsonTestContext()
	handleServiceError(c, errors.New("something weird"))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	body := decodeResponse(t, w)
	if body["message"] != "Internal server error" {
		t.Fatalf("should not leak internal error details, got %q", body["message"])
	}
}

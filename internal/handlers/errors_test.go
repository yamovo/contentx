package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestSanitizeBindErr_Nil(t *testing.T) {
	if got := sanitizeBindErr(nil); got != "请求体格式无效" {
		t.Errorf("nil err: got %q", got)
	}
}

func TestSanitizeBindErr_EOF(t *testing.T) {
	if got := sanitizeBindErr(io.EOF); got != "请求体为空" {
		t.Errorf("EOF: got %q", got)
	}
}

func TestSanitizeBindErr_JSONSyntaxError(t *testing.T) {
	// 用真实 json.Unmarshal 触发 *json.SyntaxError（其 msg 字段不可导出）。
	var v interface{}
	err := json.Unmarshal([]byte("{invalid json"), &v)
	if err == nil {
		t.Fatal("expected SyntaxError from invalid JSON")
	}
	got := sanitizeBindErr(err)
	if got != "JSON 格式错误" {
		t.Errorf("SyntaxError: got %q", got)
	}
}

func TestSanitizeBindErr_UnmarshalTypeError(t *testing.T) {
	err := &json.UnmarshalTypeError{
		Field:  "age",
		Type:   reflect.TypeOf(""),
		Offset: 10,
	}
	got := sanitizeBindErr(err)
	if !strings.Contains(got, "age") || !strings.Contains(got, "string") {
		t.Errorf("UnmarshalTypeError: got %q, want field+type", got)
	}
}

func TestSanitizeBindErr_UnknownError(t *testing.T) {
	got := sanitizeBindErr(errors.New("some internal gorm: record not found"))
	if got != "请求体格式无效" {
		t.Errorf("unknown error should be sanitized, got %q", got)
	}
}

func TestSanitizeMessage_Empty(t *testing.T) {
	if got := sanitizeMessage(""); got != "Internal server error" {
		t.Errorf("empty: got %q", got)
	}
}

func TestSanitizeMessage_SensitivePatterns(t *testing.T) {
	cases := []string{
		"gorm: record not found",
		"sql: no rows in result set",
		"pq: password authentication failed",
		"connection refused (tcp 127.0.0.1:5432)",
		"runtime error: invalid memory address",
		"open /Users/admin/.env: no such file",
		"open C:/Users/admin/.env: no such file",
	}
	for _, in := range cases {
		if got := sanitizeMessage(in); got != "Internal server error" {
			t.Errorf("sensitive pattern %q not masked: got %q", in, got)
		}
	}
}

func TestSanitizeMessage_SafeMessage(t *testing.T) {
	if got := sanitizeMessage("article not found"); got != "article not found" {
		t.Errorf("safe message should pass through: got %q", got)
	}
}

package api

import (
	"encoding/json"
	"net/http"
)

// Response represents a standard API response
type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error represents a structured error response
// Follows the "code/what/why/fix" format from spec
type Error struct {
	Code string     `json:"code"`
	What string     `json:"what"`
	Why  string     `json:"why,omitempty"`
	Fix  *FixAction `json:"fix,omitempty"`
}

// FixAction provides a one-click fix suggestion
type FixAction struct {
	Label  string `json:"label"`
	Action string `json:"action"`
	Target string `json:"target,omitempty"`
}

// WriteJSON writes a JSON response
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Success: status >= 200 && status < 300,
		Data:    data,
	})
}

// WriteError writes an error response
func WriteError(w http.ResponseWriter, status int, err *Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Success: false,
		Error:   err,
	})
}

// Common error constructors
func ErrInvalidRequest(why string) *Error {
	return &Error{
		Code: "INVALID_REQUEST",
		What: "请求参数不正确",
		Why:  why,
	}
}

func ErrNotFound(resource string) *Error {
	return &Error{
		Code: "NOT_FOUND",
		What: resource + "不存在",
	}
}

func ErrInternal(why string) *Error {
	return &Error{
		Code: "INTERNAL_ERROR",
		What: "服务器内部错误",
		Why:  why,
	}
}

func ErrKernelCheckFailed(why string, target string) *Error {
	return &Error{
		Code: "KERNEL_CHECK_FAILED",
		What: "配置校验未通过",
		Why:  why,
		Fix: &FixAction{
			Label:  "编辑该节点",
			Action: "node.edit",
			Target: target,
		},
	}
}

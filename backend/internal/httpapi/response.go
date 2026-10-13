// Package httpapi assembles the Gin HTTP router, shared middleware, JSON
// envelopes, and error responses for the /api/v1 REST API.
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorCode enumerates the machine-readable error codes used in API responses.
type ErrorCode string

const (
	ErrBadRequest     ErrorCode = "bad_request"
	ErrUnauthorized   ErrorCode = "unauthorized"
	ErrForbidden      ErrorCode = "forbidden"
	ErrNotFound       ErrorCode = "not_found"
	ErrConflict       ErrorCode = "conflict"
	ErrValidation     ErrorCode = "validation_error"
	ErrRateLimited    ErrorCode = "rate_limited"
	ErrTimeout        ErrorCode = "timeout"
	ErrUpstream       ErrorCode = "upstream_error"
	ErrInternal       ErrorCode = "internal"
	ErrNotImplemented ErrorCode = "not_implemented"
	ErrNotConfigured  ErrorCode = "not_configured"
	ErrNetworkBlocked ErrorCode = "network_blocked"
)

type errorBody struct {
	Code    ErrorCode      `json:"code"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// Respond writes an application-level payload under {"data": ...}.
func Respond(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"data": data})
}

// RespondList writes a paginated collection under {"data": ..., "meta": ...}.
func RespondList(c *gin.Context, data []any, page, perPage, total int) {
	c.JSON(http.StatusOK, gin.H{
		"data": data,
		"meta": gin.H{"page": page, "perPage": perPage, "total": total},
	})
}

// Fail writes a consistent JSON error envelope.
func Fail(c *gin.Context, status int, code ErrorCode, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": errorBody{Code: code, Message: message}})
}

// FailFields writes a validation error envelope with per-field details.
func FailFields(c *gin.Context, message string, fields map[string]any) {
	c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": errorBody{Code: ErrValidation, Message: message, Fields: fields}})
}

// InternalError writes an internal server error without leaking details.
func InternalError(c *gin.Context) {
	Fail(c, http.StatusInternalServerError, ErrInternal, "An internal error occurred.")
}

package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse is the standard Cont API error response format
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// Standard error codes
const (
	ErrCodeBadRequest    = "BAD_REQUEST"
	ErrCodeUnauthorized  = "UNAUTHORIZED"
	ErrCodeForbidden     = "FORBIDDEN"
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeConflict      = "CONFLICT"
	ErrCodeInternal      = "INTERNAL_ERROR"
	ErrCodeValidation    = "VALIDATION_ERROR"
	ErrCodeBadGateway    = "BAD_GATEWAY"
	ErrCodeInvalidJSON   = "INVALID_JSON"
	ErrCodeMissingField  = "MISSING_FIELD"
	ErrCodeAlreadyExists = "ALREADY_EXISTS"
)

// badRequestMsg sends a 400 with a plain message string
func badRequestMsg(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, ErrorResponse{Code: ErrCodeBadRequest, Message: message})
}

// badRequestWithDetails sends a 400 with details field
func badRequestWithDetails(c *gin.Context, message string, details any) {
	c.JSON(http.StatusBadRequest, ErrorResponse{Code: ErrCodeBadRequest, Message: message, Details: details})
}

// badRequestValidation sends a 400 for validation errors
func badRequestValidation(c *gin.Context, message string, errors []string) {
	c.JSON(http.StatusBadRequest, gin.H{"code": ErrCodeValidation, "message": message, "errors": errors})
}

// unauthorized sends a 401 with standard error format
func unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, ErrorResponse{Code: ErrCodeUnauthorized, Message: message})
}

// forbidden sends a 403 with standard error format
func forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, ErrorResponse{Code: ErrCodeForbidden, Message: message})
}

// notFound sends a 404 with standard error format
func notFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, ErrorResponse{Code: ErrCodeNotFound, Message: message})
}

// conflict sends a 409 with standard error format
func conflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, ErrorResponse{Code: ErrCodeConflict, Message: message})
}

// badGateway sends a 502 with optional details
func badGateway(c *gin.Context, message string, details ...string) {
	resp := ErrorResponse{Code: ErrCodeBadGateway, Message: message}
	if len(details) > 0 {
		resp.Details = details[0]
	}
	c.JSON(http.StatusBadGateway, resp)
}

// invalidJSON sends a 400 for malformed JSON
func invalidJSON(c *gin.Context) {
	c.JSON(http.StatusBadRequest, ErrorResponse{Code: ErrCodeInvalidJSON, Message: "invalid JSON body"})
}

// missingField sends a 400 for missing required fields
func missingField(c *gin.Context, field string) {
	c.JSON(http.StatusBadRequest, ErrorResponse{Code: ErrCodeMissingField, Message: "missing required field: " + field})
}

// alreadyExists sends a 409 for duplicate resources
func alreadyExists(c *gin.Context, resource string) {
	c.JSON(http.StatusConflict, ErrorResponse{Code: ErrCodeAlreadyExists, Message: resource + " already exists"})
}

// internalError sends a 500 with standard error format (message hidden from client)
func internalError(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, ErrorResponse{Code: ErrCodeInternal, Message: "internal server error"})
}

// internalErrorWithLog sends a 500 and logs the actual error (for server-side errors)
func internalErrorWithLog(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, ErrorResponse{Code: ErrCodeInternal, Message: "internal server error"})
}

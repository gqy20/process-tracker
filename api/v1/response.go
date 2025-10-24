package v1

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// NewSuccessResponse creates a successful API response
func NewSuccessResponse(kind ResponseKind, data interface{}, metadata *ResponseMetadata) *APIResponse {
	return &APIResponse{
		Kind:       kind,
		APIVersion: APIVersion,
		Metadata:   metadata,
		Data:       data,
		Errors:     nil,
	}
}

// NewErrorResponse creates an error API response
func NewErrorResponse(errors []APIError) *APIResponse {
	return &APIResponse{
		Kind:       KindError,
		APIVersion: APIVersion,
		Metadata: &ResponseMetadata{
			GeneratedAt: time.Now(),
		},
		Data:   nil,
		Errors: errors,
	}
}

// NewPaginatedResponse creates a paginated response with metadata
func NewPaginatedResponse(kind ResponseKind, data interface{}, total int, params QueryParams) *APIResponse {
	metadata := &ResponseMetadata{
		Total:       total,
		Limit:       params.Limit,
		Offset:      params.Offset,
		Sort:        params.Sort,
		Filter:      params.Filter,
		GeneratedAt: time.Now(),
	}

	// Add pagination links
	metadata.Links = make(map[string]string)
	baseURL := ""

	if params.Offset > 0 {
		prevOffset := params.Offset - params.Limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		metadata.Links["prev"] = baseURL + "?limit=" + strconv.Itoa(params.Limit) + "&offset=" + strconv.Itoa(prevOffset)
	}

	if params.Offset+params.Limit < total {
		nextOffset := params.Offset + params.Limit
		metadata.Links["next"] = baseURL + "?limit=" + strconv.Itoa(params.Limit) + "&offset=" + strconv.Itoa(nextOffset)
	}

	return NewSuccessResponse(kind, data, metadata)
}

// SendSuccess sends a successful response
func SendSuccess(c *gin.Context, kind ResponseKind, data interface{}, metadata *ResponseMetadata) {
	response := NewSuccessResponse(kind, data, metadata)
	c.JSON(http.StatusOK, response)
}

// SendPaginated sends a paginated response
func SendPaginated(c *gin.Context, kind ResponseKind, data interface{}, total int, params QueryParams) {
	response := NewPaginatedResponse(kind, data, total, params)
	c.JSON(http.StatusOK, response)
}

// SendError sends an error response
func SendError(c *gin.Context, statusCode int, code string, message string, details map[string]interface{}) {
	error := APIError{
		Code:    code,
		Message: message,
		Details: details,
	}

	response := NewErrorResponse([]APIError{error})
	c.JSON(statusCode, response)
}

// SendValidationError sends validation error response
func SendValidationError(c *gin.Context, errors []string) {
	apiErrors := make([]APIError, len(errors))
	for i, err := range errors {
		apiErrors[i] = APIError{
			Code:    "VALIDATION_ERROR",
			Message: err,
		}
	}

	response := NewErrorResponse(apiErrors)
	c.JSON(http.StatusBadRequest, response)
}

// SendNotFoundError sends not found error response
func SendNotFoundError(c *gin.Context, resource string, id interface{}) {
	SendError(c, http.StatusNotFound, "NOT_FOUND",
		resource+" with id "+strconv.Itoa(int(id.(int)))+" not found", nil)
}

// SendInternalServerError sends internal server error response
func SendInternalServerError(c *gin.Context, err error) {
	details := map[string]interface{}{
		"error": err.Error(),
	}
	SendError(c, http.StatusInternalServerError, "INTERNAL_ERROR",
		"Internal server error occurred", details)
}

// SendBadRequest sends bad request error response
func SendBadRequest(c *gin.Context, message string) {
	SendError(c, http.StatusBadRequest, "BAD_REQUEST", message, nil)
}

// SendConflict sends conflict error response
func SendConflict(c *gin.Context, message string) {
	SendError(c, http.StatusConflict, "CONFLICT", message, nil)
}

// SendUnauthorized sends unauthorized error response
func SendUnauthorized(c *gin.Context, message string) {
	SendError(c, http.StatusUnauthorized, "UNAUTHORIZED", message, nil)
}

// SendForbidden sends forbidden error response
func SendForbidden(c *gin.Context, message string) {
	SendError(c, http.StatusForbidden, "FORBIDDEN", message, nil)
}

// SendCreated sends created response
func SendCreated(c *gin.Context, kind ResponseKind, data interface{}) {
	metadata := &ResponseMetadata{
		GeneratedAt: time.Now(),
	}
	response := NewSuccessResponse(kind, data, metadata)
	c.JSON(http.StatusCreated, response)
}

// SendNoContent sends no content response
func SendNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Helper functions for common error codes
const (
	ErrorCodeValidation      = "VALIDATION_ERROR"
	ErrorCodeNotFound       = "NOT_FOUND"
	ErrorCodeInternal       = "INTERNAL_ERROR"
	ErrorCodeBadRequest     = "BAD_REQUEST"
	ErrorCodeConflict       = "CONFLICT"
	ErrorCodeUnauthorized   = "UNAUTHORIZED"
	ErrorCodeForbidden      = "FORBIDDEN"
	ErrorCodeTaskNotFound   = "TASK_NOT_FOUND"
	ErrorCodeProcessNotFound = "PROCESS_NOT_FOUND"
	ErrorCodeTaskRunning   = "TASK_ALREADY_RUNNING"
	ErrorCodeTaskStopped   = "TASK_ALREADY_STOPPED"
)
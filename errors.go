package hevy

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

// APIError represents a non-2xx response from the Hevy API.
type APIError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *APIError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("hevy API: %s: %s", e.Status, e.Body)
	}
	return fmt.Sprintf("hevy API: %s", e.Status)
}

func newAPIError(resp *http.Response) *APIError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return &APIError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       string(body),
	}
}

// IsNotFound reports whether err is a 404 API error.
func IsNotFound(err error) bool {
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// IsRateLimited reports whether err is a 429 API error.
func IsRateLimited(err error) bool {
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.StatusCode == http.StatusTooManyRequests
	}
	return false
}

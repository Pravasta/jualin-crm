package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Sentinel errors. Handlers never set HTTP status codes themselves — they
// return one of these (or wrap one via errors.Is-compatible wrapping), and
// MapError translates it centrally. See docs/architecture/api.md for the
// full error code catalog as it grows with each phase.
var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation failed")
	ErrInternal   = errors.New("internal error")
)

// ErrorDetail describes a single field-level validation failure.
type ErrorDetail struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// ErrorBody is the wire shape of the "error" envelope.
//
// Code is stable and machine-readable — clients key behavior off it, never
// off Message. Message is for humans and may change without being
// considered a breaking change.
type ErrorBody struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

// ValidationError wraps ErrValidation with field-level details.
type ValidationError struct {
	Details []ErrorDetail
}

func (e *ValidationError) Error() string { return "validation failed" }
func (e *ValidationError) Unwrap() error { return ErrValidation }

// NewValidationError builds a ValidationError from field/code pairs.
func NewValidationError(details ...ErrorDetail) *ValidationError {
	return &ValidationError{Details: details}
}

// DomainError lets any domain package produce a specific status/code/
// message without internal/shared/httpx needing to import that domain
// package — importing domain packages here would invert the dependency
// direction every other shared package follows. A domain constructs
// &httpx.DomainError{...} and returns it; MapError recognizes the type
// generically via errors.As.
//
// This is the extension point referenced throughout
// .claude/skills/jualin-backend: "tambahkan sentinel baru + case di
// MapError saat domain butuh kode error baru" — for a one-off status/code
// pair, constructing a DomainError directly is enough; only reach for a
// dedicated sentinel + MapError case when multiple call sites need to
// recognize the same error via errors.Is/errors.As.
type DomainError struct {
	Status  int
	Code    string
	Message string
}

func (e *DomainError) Error() string { return e.Message }

// MapError translates a domain error into an HTTP status and response body.
//
// Resources belonging to another tenant must be returned as ErrNotFound
// (404), never as a 403 — a 403 confirms the resource exists, which is
// itself an information leak (Rule #6).
func MapError(err error) (int, ErrorBody) {
	var verr *ValidationError
	var derr *DomainError
	switch {
	case errors.As(err, &verr):
		return http.StatusBadRequest, ErrorBody{
			Code:    "validation_failed",
			Message: "Permintaan tidak valid.",
			Details: verr.Details,
		}
	case errors.As(err, &derr):
		return derr.Status, ErrorBody{Code: derr.Code, Message: derr.Message}
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound, ErrorBody{
			Code:    "not_found",
			Message: "Resource tidak ditemukan.",
		}
	case errors.Is(err, ErrValidation):
		return http.StatusBadRequest, ErrorBody{
			Code:    "validation_failed",
			Message: "Permintaan tidak valid.",
		}
	default:
		// Internal details never leak into the response — only into logs.
		return http.StatusInternalServerError, ErrorBody{
			Code:    "internal_error",
			Message: "Terjadi kesalahan internal.",
		}
	}
}

// WriteError maps err and writes the resulting error envelope.
func WriteError(c *gin.Context, err error) {
	status, body := MapError(err)
	c.JSON(status, gin.H{"error": body})
}

// RespondError writes an error envelope directly from a status/code/message
// triple, for the router-level cases (404 route not found, 405 method not
// allowed) that never produce a Go error to map.
func RespondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": ErrorBody{Code: code, Message: message}})
}

package httpx_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
)

func TestMapError_NotFound(t *testing.T) {
	status, body := httpx.MapError(httpx.ErrNotFound)

	if status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", status)
	}
	if body.Code != "not_found" {
		t.Errorf("expected code 'not_found', got %q", body.Code)
	}
}

func TestMapError_WrappedNotFound(t *testing.T) {
	wrapped := fmt.Errorf("lead 123: %w", httpx.ErrNotFound)

	status, body := httpx.MapError(wrapped)

	if status != http.StatusNotFound {
		t.Errorf("expected status 404 for wrapped ErrNotFound, got %d", status)
	}
	if body.Code != "not_found" {
		t.Errorf("expected code 'not_found', got %q", body.Code)
	}
}

func TestMapError_Validation(t *testing.T) {
	status, body := httpx.MapError(httpx.ErrValidation)

	if status != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", status)
	}
	if body.Code != "validation_failed" {
		t.Errorf("expected code 'validation_failed', got %q", body.Code)
	}
}

func TestMapError_ValidationWithDetails(t *testing.T) {
	err := httpx.NewValidationError(
		httpx.ErrorDetail{Field: "email", Code: "invalid_format"},
	)

	status, body := httpx.MapError(err)

	if status != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", status)
	}
	if len(body.Details) != 1 || body.Details[0].Field != "email" {
		t.Errorf("expected details to carry field 'email', got %+v", body.Details)
	}
	if !errors.Is(err, httpx.ErrValidation) {
		t.Error("expected ValidationError to unwrap to ErrValidation")
	}
}

func TestMapError_UnknownError_MapsToInternal(t *testing.T) {
	status, body := httpx.MapError(errors.New("something exploded"))

	if status != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", status)
	}
	if body.Code != "internal_error" {
		t.Errorf("expected code 'internal_error', got %q", body.Code)
	}
	if body.Message == "something exploded" {
		t.Error("internal error details must not leak into the response message")
	}
}

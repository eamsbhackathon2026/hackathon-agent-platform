package domain

import (
	"errors"
	"github.com/google/uuid"
)

// Stable error kinds shared by services and adapters.
var (
	ErrNotFound             = errors.New("not found")
	ErrForbidden            = errors.New("forbidden")
	ErrUnauthenticated      = errors.New("unauthenticated")
	ErrConflict             = errors.New("conflict")
	ErrValidation           = errors.New("validation failed")
	ErrRateLimited          = errors.New("rate limited")
	ErrNotImplemented       = errors.New("not implemented")
	ErrRunInProgress        = errors.New("run in progress")
	ErrIdempotencyKeyReused = errors.New("idempotency key reused")
)

// FieldError provides safe validation details for one input field.
type FieldError struct {
	Field   string
	Message string
}

// Error carries safe, user-facing details in addition to a stable error kind.
type Error struct {
	Kind          error
	Detail        string
	Fields        []FieldError
	RelatedAgents []ResourceReference
	RunID         *uuid.UUID
}

func (e *Error) Error() string { return e.Kind.Error() }
func (e *Error) Unwrap() error { return e.Kind }

// Invalid creates a field-specific validation error with safe details.
func Invalid(field, message string) error {
	return &Error{Kind: ErrValidation, Fields: []FieldError{{Field: field, Message: message}}}
}

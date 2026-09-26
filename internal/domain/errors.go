package domain

import "errors"

var (
	// ErrUnauthorized is returned when authentication fails or token is missing/invalid
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden is returned when a user attempts to access or modify resources owned by someone else
	ErrForbidden = errors.New("forbidden")

	// ErrNotFound is returned when a requested resource does not exist or has been soft-deleted
	ErrNotFound = errors.New("not found")

	// ErrValidation is returned when input parameters fail domain validation rules
	ErrValidation = errors.New("validation failed")

	// ErrRateLimited is returned when rate limits are exceeded
	ErrRateLimited = errors.New("rate limit exceeded")

	// ErrInternalServer is returned for unrecoverable server errors
	ErrInternalServer = errors.New("internal server error")
)

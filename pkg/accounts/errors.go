package accounts

import "errors"

var (
	// ErrNotFound indicates the target entity does not exist.
	ErrNotFound = errors.New("accounts: not found")
	// ErrConflict indicates a unique or ownership conflict.
	ErrConflict = errors.New("accounts: conflict")
	// ErrInvalidInput indicates the payload failed validation.
	ErrInvalidInput = errors.New("accounts: invalid input")
)

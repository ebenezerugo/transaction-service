package domain

import "errors"

var (
	ErrAccountNotFound        = errors.New("account not found")
	ErrInsufficientBalance    = errors.New("insufficient balance")
	ErrTransactionNotFound    = errors.New("transaction not found")
	ErrDuplicateTransaction   = errors.New("duplicate transaction: idempotency key already exists")
	ErrInvalidAmount          = errors.New("invalid amount: must be greater than zero")
	ErrAccountInactive        = errors.New("account is not active")
	ErrOptimisticLockFailed   = errors.New("optimistic lock failed: account was modified by another transaction")
	ErrHoldNotFound           = errors.New("hold not found")
	ErrInvalidTransactionType = errors.New("invalid transaction type")
	ErrMaxRetriesExceeded     = errors.New("maximum retry attempts exceeded")
)

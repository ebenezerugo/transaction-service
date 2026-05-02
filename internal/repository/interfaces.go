package repository

import (
	"context"

	"github.com/ebenezerugo/transaction-service/internal/domain"
)

type AccountRepository interface {
	GetByID(ctx context.Context, id string) (*domain.Account, error)
	GetByAccountNumber(ctx context.Context, accountNumber string) (*domain.Account, error)
	UpdateBalance(ctx context.Context, accountID string, balance float64, version int64) error
	UpdateHeldBalance(ctx context.Context, accountID string, heldBalance float64, version int64) error
	Create(ctx context.Context, account *domain.Account) error
	List(ctx context.Context, limit, offset int) ([]*domain.Account, error)
}

type TransactionRepository interface {
	Create(ctx context.Context, tx *domain.Transaction) error
	GetByID(ctx context.Context, id string) (*domain.Transaction, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error)
	UpdateStatus(ctx context.Context, id string, status domain.TransactionStatus, errMsg string) error
	ListByAccountID(ctx context.Context, accountID string, limit, offset int) ([]*domain.Transaction, error)
	CreateHold(ctx context.Context, hold *domain.Hold) error
	GetHoldByID(ctx context.Context, id string) (*domain.Hold, error)
	UpdateHoldStatus(ctx context.Context, id, status string) error
	ListActiveHoldsByAccountID(ctx context.Context, accountID string) ([]*domain.Hold, error)
}

type AuditRepository interface {
	Create(ctx context.Context, event *AuditLog) error
}

type AuditLog struct {
	ID            string
	TransactionID string
	AccountID     string
	EventType     string
	EventData     interface{}
	OperatorID    string
	IPAddress     string
}

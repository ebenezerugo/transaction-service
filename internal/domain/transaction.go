package domain

import "time"

type TransactionType string

const (
	TransactionTypeTransfer   TransactionType = "TRANSFER"
	TransactionTypeDeposit    TransactionType = "DEPOSIT"
	TransactionTypeWithdrawal TransactionType = "WITHDRAWAL"
	TransactionTypeRetrieve   TransactionType = "RETRIEVE"
	TransactionTypeUndo       TransactionType = "UNDO"
	TransactionTypeAdjust     TransactionType = "ADJUST"
	TransactionTypeHold       TransactionType = "HOLD"
	TransactionTypeRelease    TransactionType = "RELEASE"
)

type TransactionStatus string

const (
	StatusPending    TransactionStatus = "PENDING"
	StatusProcessing TransactionStatus = "PROCESSING"
	StatusCompleted  TransactionStatus = "COMPLETED"
	StatusFailed     TransactionStatus = "FAILED"
	StatusReversed   TransactionStatus = "REVERSED"
)

type TransferRoute string

const (
	RouteNIBSS    TransferRoute = "NIBSS"
	RouteFineract TransferRoute = "FINERACT"
)

type Transaction struct {
	ID             string
	IdempotencyKey string
	Type           TransactionType
	Status         TransactionStatus
	AccountID      string
	ToAccountID    string
	Amount         float64
	Currency       string
	Description    string
	Route          TransferRoute
	RetryCount     int
	ErrorMessage   string
	Metadata       map[string]interface{}
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

type Hold struct {
	ID            string
	AccountID     string
	TransactionID string
	Amount        float64
	Status        string
	ExpiresAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

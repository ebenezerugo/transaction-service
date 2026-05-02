package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ebenezerugo/transaction-service/internal/compliance"
	"github.com/ebenezerugo/transaction-service/internal/config"
	"github.com/ebenezerugo/transaction-service/internal/domain"
	"github.com/ebenezerugo/transaction-service/internal/kafka"
	"github.com/ebenezerugo/transaction-service/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type TransactionService interface {
	ProcessDeposit(ctx context.Context, req *DepositRequest) (*domain.Transaction, error)
	ProcessWithdrawal(ctx context.Context, req *WithdrawalRequest) (*domain.Transaction, error)
	ProcessTransfer(ctx context.Context, req *TransferRequest) (*domain.Transaction, error)
	ProcessUndo(ctx context.Context, req *UndoRequest) (*domain.Transaction, error)
	ProcessAdjust(ctx context.Context, req *AdjustRequest) (*domain.Transaction, error)
	GetTransaction(ctx context.Context, id string) (*domain.Transaction, error)
	GetTransactionsByAccount(ctx context.Context, accountID string, limit, offset int) ([]*domain.Transaction, error)
}

type DepositRequest struct {
	IdempotencyKey string
	AccountID      string
	Amount         float64
	Currency       string
	Description    string
	OperatorID     string
}

type WithdrawalRequest struct {
	IdempotencyKey string
	AccountID      string
	Amount         float64
	Currency       string
	Description    string
	OperatorID     string
}

type TransferRequest struct {
	IdempotencyKey string
	FromAccountID  string
	ToAccountID    string
	Amount         float64
	Currency       string
	Description    string
	OperatorID     string
}

type UndoRequest struct {
	IdempotencyKey string
	OriginalTxID   string
	AccountID      string
	OperatorID     string
}

type AdjustRequest struct {
	IdempotencyKey string
	AccountID      string
	Amount         float64
	Currency       string
	Description    string
	OperatorID     string
}

type transactionService struct {
	txRepo      repository.TransactionRepository
	accountRepo repository.AccountRepository
	auditLogger compliance.CBNAuditLogger
	producer    kafka.Producer
	cfg         *config.Config
	logger      *zap.Logger
}

func NewTransactionService(
	txRepo repository.TransactionRepository,
	accountRepo repository.AccountRepository,
	auditLogger compliance.CBNAuditLogger,
	producer kafka.Producer,
	cfg *config.Config,
	logger *zap.Logger,
) TransactionService {
	return &transactionService{
		txRepo:      txRepo,
		accountRepo: accountRepo,
		auditLogger: auditLogger,
		producer:    producer,
		cfg:         cfg,
		logger:      logger,
	}
}

func (s *transactionService) checkIdempotency(ctx context.Context, key string) (*domain.Transaction, error) {
	existing, err := s.txRepo.GetByIdempotencyKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	}
	if existing != nil {
		return existing, domain.ErrDuplicateTransaction
	}
	return nil, nil
}

func (s *transactionService) ProcessDeposit(ctx context.Context, req *DepositRequest) (*domain.Transaction, error) {
	if req.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	if existing, err := s.checkIdempotency(ctx, req.IdempotencyKey); existing != nil {
		return existing, nil
	} else if err != nil && err != domain.ErrDuplicateTransaction {
		return nil, err
	}

	account, err := s.accountRepo.GetByID(ctx, req.AccountID)
	if err != nil {
		return nil, err
	}
	if account.Status != "ACTIVE" {
		return nil, domain.ErrAccountInactive
	}

	tx := &domain.Transaction{
		ID:             uuid.New().String(),
		IdempotencyKey: req.IdempotencyKey,
		Type:           domain.TransactionTypeDeposit,
		Status:         domain.StatusProcessing,
		AccountID:      req.AccountID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Description:    req.Description,
	}

	if err := s.txRepo.Create(ctx, tx); err != nil {
		return nil, err
	}

	newBalance := account.Balance + req.Amount
	if err := s.accountRepo.UpdateBalance(ctx, account.ID, newBalance, account.Version); err != nil {
		_ = s.txRepo.UpdateStatus(ctx, tx.ID, domain.StatusFailed, err.Error())
		return nil, err
	}

	now := time.Now()
	tx.CompletedAt = &now
	tx.Status = domain.StatusCompleted
	_ = s.txRepo.UpdateStatus(ctx, tx.ID, domain.StatusCompleted, "")

	_ = s.auditLogger.LogTransactionEvent(ctx, tx.ID, "DEPOSIT_COMPLETED", map[string]interface{}{
		"account_id": req.AccountID,
		"amount":     req.Amount,
		"currency":   req.Currency,
	})

	if s.producer != nil {
		_ = s.producer.PublishTransaction(ctx, kafka.TopicTransactionEvents, req.AccountID, tx)
	}

	return tx, nil
}

func (s *transactionService) ProcessWithdrawal(ctx context.Context, req *WithdrawalRequest) (*domain.Transaction, error) {
	if req.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	if existing, err := s.checkIdempotency(ctx, req.IdempotencyKey); existing != nil {
		return existing, nil
	} else if err != nil && err != domain.ErrDuplicateTransaction {
		return nil, err
	}

	account, err := s.accountRepo.GetByID(ctx, req.AccountID)
	if err != nil {
		return nil, err
	}
	if account.Status != "ACTIVE" {
		return nil, domain.ErrAccountInactive
	}
	if account.AvailableBalance < req.Amount {
		return nil, domain.ErrInsufficientBalance
	}

	tx := &domain.Transaction{
		ID:             uuid.New().String(),
		IdempotencyKey: req.IdempotencyKey,
		Type:           domain.TransactionTypeWithdrawal,
		Status:         domain.StatusProcessing,
		AccountID:      req.AccountID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Description:    req.Description,
	}

	if err := s.txRepo.Create(ctx, tx); err != nil {
		return nil, err
	}

	newBalance := account.Balance - req.Amount
	if err := s.accountRepo.UpdateBalance(ctx, account.ID, newBalance, account.Version); err != nil {
		_ = s.txRepo.UpdateStatus(ctx, tx.ID, domain.StatusFailed, err.Error())
		return nil, err
	}

	now := time.Now()
	tx.CompletedAt = &now
	tx.Status = domain.StatusCompleted
	_ = s.txRepo.UpdateStatus(ctx, tx.ID, domain.StatusCompleted, "")

	_ = s.auditLogger.LogTransactionEvent(ctx, tx.ID, "WITHDRAWAL_COMPLETED", map[string]interface{}{
		"account_id": req.AccountID,
		"amount":     req.Amount,
	})

	if s.producer != nil {
		_ = s.producer.PublishTransaction(ctx, kafka.TopicTransactionEvents, req.AccountID, tx)
	}

	return tx, nil
}

func (s *transactionService) ProcessTransfer(ctx context.Context, req *TransferRequest) (*domain.Transaction, error) {
	if req.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	if existing, err := s.checkIdempotency(ctx, req.IdempotencyKey); existing != nil {
		return existing, nil
	} else if err != nil && err != domain.ErrDuplicateTransaction {
		return nil, err
	}

	fromAccount, err := s.accountRepo.GetByID(ctx, req.FromAccountID)
	if err != nil {
		return nil, err
	}
	if fromAccount.Status != "ACTIVE" {
		return nil, domain.ErrAccountInactive
	}
	if fromAccount.AvailableBalance < req.Amount {
		return nil, domain.ErrInsufficientBalance
	}

	toAccount, err := s.accountRepo.GetByID(ctx, req.ToAccountID)
	if err != nil {
		return nil, err
	}
	if toAccount.Status != "ACTIVE" {
		return nil, domain.ErrAccountInactive
	}

	route := domain.RouteFineract
	if fromAccount.BankCode != toAccount.BankCode {
		route = domain.RouteNIBSS
	}

	tx := &domain.Transaction{
		ID:             uuid.New().String(),
		IdempotencyKey: req.IdempotencyKey,
		Type:           domain.TransactionTypeTransfer,
		Status:         domain.StatusProcessing,
		AccountID:      req.FromAccountID,
		ToAccountID:    req.ToAccountID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Description:    req.Description,
		Route:          route,
	}

	if err := s.txRepo.Create(ctx, tx); err != nil {
		return nil, err
	}

	newFromBalance := fromAccount.Balance - req.Amount
	if err := s.accountRepo.UpdateBalance(ctx, fromAccount.ID, newFromBalance, fromAccount.Version); err != nil {
		_ = s.txRepo.UpdateStatus(ctx, tx.ID, domain.StatusFailed, err.Error())
		return nil, err
	}

	toAccount, err = s.accountRepo.GetByID(ctx, req.ToAccountID)
	if err != nil {
		_ = s.txRepo.UpdateStatus(ctx, tx.ID, domain.StatusFailed, err.Error())
		return nil, err
	}

	newToBalance := toAccount.Balance + req.Amount
	if err := s.accountRepo.UpdateBalance(ctx, toAccount.ID, newToBalance, toAccount.Version); err != nil {
		_ = s.txRepo.UpdateStatus(ctx, tx.ID, domain.StatusFailed, err.Error())
		return nil, err
	}

	now := time.Now()
	tx.CompletedAt = &now
	tx.Status = domain.StatusCompleted
	_ = s.txRepo.UpdateStatus(ctx, tx.ID, domain.StatusCompleted, "")

	_ = s.auditLogger.LogTransactionEvent(ctx, tx.ID, "TRANSFER_COMPLETED", map[string]interface{}{
		"from_account": req.FromAccountID,
		"to_account":   req.ToAccountID,
		"amount":       req.Amount,
		"route":        string(route),
	})

	if s.producer != nil {
		_ = s.producer.PublishTransaction(ctx, kafka.TopicTransactionEvents, req.FromAccountID, tx)
	}

	return tx, nil
}

func (s *transactionService) ProcessUndo(ctx context.Context, req *UndoRequest) (*domain.Transaction, error) {
	if existing, err := s.checkIdempotency(ctx, req.IdempotencyKey); existing != nil {
		return existing, nil
	} else if err != nil && err != domain.ErrDuplicateTransaction {
		return nil, err
	}

	origTx, err := s.txRepo.GetByID(ctx, req.OriginalTxID)
	if err != nil {
		return nil, err
	}

	account, err := s.accountRepo.GetByID(ctx, origTx.AccountID)
	if err != nil {
		return nil, err
	}

	undoTx := &domain.Transaction{
		ID:             uuid.New().String(),
		IdempotencyKey: req.IdempotencyKey,
		Type:           domain.TransactionTypeUndo,
		Status:         domain.StatusProcessing,
		AccountID:      origTx.AccountID,
		Amount:         origTx.Amount,
		Currency:       origTx.Currency,
		Description:    fmt.Sprintf("Undo of transaction %s", req.OriginalTxID),
	}

	if err := s.txRepo.Create(ctx, undoTx); err != nil {
		return nil, err
	}

	var newBalance float64
	switch origTx.Type {
	case domain.TransactionTypeDeposit:
		newBalance = account.Balance - origTx.Amount
	case domain.TransactionTypeWithdrawal:
		newBalance = account.Balance + origTx.Amount
	default:
		_ = s.txRepo.UpdateStatus(ctx, undoTx.ID, domain.StatusFailed, "unsupported undo type")
		return nil, domain.ErrInvalidTransactionType
	}

	if err := s.accountRepo.UpdateBalance(ctx, account.ID, newBalance, account.Version); err != nil {
		_ = s.txRepo.UpdateStatus(ctx, undoTx.ID, domain.StatusFailed, err.Error())
		return nil, err
	}

	_ = s.txRepo.UpdateStatus(ctx, origTx.ID, domain.StatusReversed, "")
	now := time.Now()
	undoTx.CompletedAt = &now
	undoTx.Status = domain.StatusCompleted
	_ = s.txRepo.UpdateStatus(ctx, undoTx.ID, domain.StatusCompleted, "")

	return undoTx, nil
}

func (s *transactionService) ProcessAdjust(ctx context.Context, req *AdjustRequest) (*domain.Transaction, error) {
	if existing, err := s.checkIdempotency(ctx, req.IdempotencyKey); existing != nil {
		return existing, nil
	} else if err != nil && err != domain.ErrDuplicateTransaction {
		return nil, err
	}

	account, err := s.accountRepo.GetByID(ctx, req.AccountID)
	if err != nil {
		return nil, err
	}

	tx := &domain.Transaction{
		ID:             uuid.New().String(),
		IdempotencyKey: req.IdempotencyKey,
		Type:           domain.TransactionTypeAdjust,
		Status:         domain.StatusProcessing,
		AccountID:      req.AccountID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Description:    req.Description,
	}

	if err := s.txRepo.Create(ctx, tx); err != nil {
		return nil, err
	}

	newBalance := account.Balance + req.Amount
	if newBalance < 0 {
		_ = s.txRepo.UpdateStatus(ctx, tx.ID, domain.StatusFailed, "adjustment would result in negative balance")
		return nil, domain.ErrInsufficientBalance
	}

	if err := s.accountRepo.UpdateBalance(ctx, account.ID, newBalance, account.Version); err != nil {
		_ = s.txRepo.UpdateStatus(ctx, tx.ID, domain.StatusFailed, err.Error())
		return nil, err
	}

	now := time.Now()
	tx.CompletedAt = &now
	tx.Status = domain.StatusCompleted
	_ = s.txRepo.UpdateStatus(ctx, tx.ID, domain.StatusCompleted, "")

	_ = s.auditLogger.LogTransactionEvent(ctx, tx.ID, "ADJUST_COMPLETED", map[string]interface{}{
		"account_id": req.AccountID,
		"amount":     req.Amount,
	})

	return tx, nil
}

func (s *transactionService) GetTransaction(ctx context.Context, id string) (*domain.Transaction, error) {
	return s.txRepo.GetByID(ctx, id)
}

func (s *transactionService) GetTransactionsByAccount(ctx context.Context, accountID string, limit, offset int) ([]*domain.Transaction, error) {
	return s.txRepo.ListByAccountID(ctx, accountID, limit, offset)
}

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ebenezerugo/transaction-service/internal/compliance"
	"github.com/ebenezerugo/transaction-service/internal/domain"
	"github.com/ebenezerugo/transaction-service/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type HoldService interface {
	PlaceHold(ctx context.Context, req *PlaceHoldRequest) (*domain.Hold, error)
	ReleaseHold(ctx context.Context, req *ReleaseHoldRequest) error
	GetHold(ctx context.Context, id string) (*domain.Hold, error)
}

type PlaceHoldRequest struct {
	AccountID     string
	TransactionID string
	Amount        float64
	ExpiresAt     *time.Time
	OperatorID    string
}

type ReleaseHoldRequest struct {
	HoldID     string
	OperatorID string
}

type holdService struct {
	txRepo      repository.TransactionRepository
	accountRepo repository.AccountRepository
	auditLogger compliance.CBNAuditLogger
	logger      *zap.Logger
}

func NewHoldService(
	txRepo repository.TransactionRepository,
	accountRepo repository.AccountRepository,
	auditLogger compliance.CBNAuditLogger,
	logger *zap.Logger,
) HoldService {
	return &holdService{
		txRepo:      txRepo,
		accountRepo: accountRepo,
		auditLogger: auditLogger,
		logger:      logger,
	}
}

func (s *holdService) PlaceHold(ctx context.Context, req *PlaceHoldRequest) (*domain.Hold, error) {
	if req.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
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

	hold := &domain.Hold{
		ID:            uuid.New().String(),
		AccountID:     req.AccountID,
		TransactionID: req.TransactionID,
		Amount:        req.Amount,
		Status:        "ACTIVE",
		ExpiresAt:     req.ExpiresAt,
	}

	if err := s.txRepo.CreateHold(ctx, hold); err != nil {
		return nil, fmt.Errorf("create hold: %w", err)
	}

	newHeld := account.HeldBalance + req.Amount
	if err := s.accountRepo.UpdateHeldBalance(ctx, account.ID, newHeld, account.Version); err != nil {
		return nil, err
	}

	_ = s.auditLogger.LogTransactionEvent(ctx, req.TransactionID, "HOLD_PLACED", map[string]interface{}{
		"account_id": req.AccountID,
		"amount":     req.Amount,
		"hold_id":    hold.ID,
	})

	return hold, nil
}

func (s *holdService) ReleaseHold(ctx context.Context, req *ReleaseHoldRequest) error {
	hold, err := s.txRepo.GetHoldByID(ctx, req.HoldID)
	if err != nil {
		return err
	}
	if hold.Status != "ACTIVE" {
		return fmt.Errorf("hold %s is not active", req.HoldID)
	}

	account, err := s.accountRepo.GetByID(ctx, hold.AccountID)
	if err != nil {
		return err
	}

	newHeld := account.HeldBalance - hold.Amount
	if newHeld < 0 {
		newHeld = 0
	}

	if err := s.accountRepo.UpdateHeldBalance(ctx, account.ID, newHeld, account.Version); err != nil {
		return err
	}

	if err := s.txRepo.UpdateHoldStatus(ctx, req.HoldID, "RELEASED"); err != nil {
		return err
	}

	_ = s.auditLogger.LogTransactionEvent(ctx, hold.TransactionID, "HOLD_RELEASED", map[string]interface{}{
		"account_id": hold.AccountID,
		"amount":     hold.Amount,
		"hold_id":    req.HoldID,
	})

	return nil
}

func (s *holdService) GetHold(ctx context.Context, id string) (*domain.Hold, error) {
	return s.txRepo.GetHoldByID(ctx, id)
}

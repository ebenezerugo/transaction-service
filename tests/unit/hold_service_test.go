package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ebenezerugo/transaction-service/internal/compliance"
	"github.com/ebenezerugo/transaction-service/internal/domain"
	"github.com/ebenezerugo/transaction-service/internal/repository"
	"github.com/ebenezerugo/transaction-service/internal/service"
	"go.uber.org/zap"
)

func setupHoldService() (service.HoldService, *mockTransactionRepo, *mockAccountRepo) {
	txRepo := newMockTransactionRepo()
	accountRepo := newMockAccountRepo()
	auditRepo := &mockAuditRepo{}
	logger := zap.NewNop()

	auditLogger := compliance.NewCBNAuditLogger(auditRepo, logger)
	svc := service.NewHoldService(txRepo, accountRepo, auditLogger, logger)
	return svc, txRepo, accountRepo
}

// Ensure mockAuditRepo implements repository.AuditRepository.
var _ repository.AuditRepository = (*mockAuditRepo)(nil)

func TestPlaceHold_Success(t *testing.T) {
	svc, _, accountRepo := setupHoldService()

	account := &domain.Account{
		ID:               "acc-hold-001",
		Balance:          1000.00,
		HeldBalance:      0,
		AvailableBalance: 1000.00,
		Status:           "ACTIVE",
		Version:          0,
	}
	accountRepo.accounts["acc-hold-001"] = account

	hold, err := svc.PlaceHold(context.Background(), &service.PlaceHoldRequest{
		AccountID: "acc-hold-001",
		Amount:    200.00,
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if hold == nil {
		t.Fatal("expected hold, got nil")
	}
	if hold.Status != "ACTIVE" {
		t.Errorf("expected ACTIVE, got %s", hold.Status)
	}
	if account.HeldBalance != 200.00 {
		t.Errorf("expected held balance 200, got %.2f", account.HeldBalance)
	}
}

func TestPlaceHold_InsufficientBalance(t *testing.T) {
	svc, _, accountRepo := setupHoldService()

	account := &domain.Account{
		ID:               "acc-hold-002",
		Balance:          100.00,
		HeldBalance:      0,
		AvailableBalance: 100.00,
		Status:           "ACTIVE",
		Version:          0,
	}
	accountRepo.accounts["acc-hold-002"] = account

	_, err := svc.PlaceHold(context.Background(), &service.PlaceHoldRequest{
		AccountID: "acc-hold-002",
		Amount:    500.00,
	})

	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Errorf("expected ErrInsufficientBalance, got: %v", err)
	}
}

func TestPlaceHold_InvalidAmount(t *testing.T) {
	svc, _, _ := setupHoldService()

	_, err := svc.PlaceHold(context.Background(), &service.PlaceHoldRequest{
		AccountID: "acc-hold-003",
		Amount:    -100.00,
	})

	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("expected ErrInvalidAmount, got: %v", err)
	}
}

func TestPlaceHold_InactiveAccount(t *testing.T) {
	svc, _, accountRepo := setupHoldService()

	account := &domain.Account{
		ID:               "acc-hold-004",
		Balance:          1000.00,
		AvailableBalance: 1000.00,
		Status:           "INACTIVE",
		Version:          0,
	}
	accountRepo.accounts["acc-hold-004"] = account

	_, err := svc.PlaceHold(context.Background(), &service.PlaceHoldRequest{
		AccountID: "acc-hold-004",
		Amount:    100.00,
	})

	if !errors.Is(err, domain.ErrAccountInactive) {
		t.Errorf("expected ErrAccountInactive, got: %v", err)
	}
}

func TestReleaseHold_Success(t *testing.T) {
	svc, txRepo, accountRepo := setupHoldService()

	account := &domain.Account{
		ID:               "acc-rel-001",
		Balance:          1000.00,
		HeldBalance:      200.00,
		AvailableBalance: 800.00,
		Status:           "ACTIVE",
		Version:          0,
	}
	accountRepo.accounts["acc-rel-001"] = account

	expires := time.Now().Add(24 * time.Hour)
	hold := &domain.Hold{
		ID:        "hold-001",
		AccountID: "acc-rel-001",
		Amount:    200.00,
		Status:    "ACTIVE",
		ExpiresAt: &expires,
	}
	txRepo.holds["hold-001"] = hold

	err := svc.ReleaseHold(context.Background(), &service.ReleaseHoldRequest{
		HoldID: "hold-001",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if hold.Status != "RELEASED" {
		t.Errorf("expected RELEASED, got %s", hold.Status)
	}
	if account.HeldBalance != 0 {
		t.Errorf("expected held balance 0, got %.2f", account.HeldBalance)
	}
}

func TestReleaseHold_NotFound(t *testing.T) {
	svc, _, _ := setupHoldService()

	err := svc.ReleaseHold(context.Background(), &service.ReleaseHoldRequest{
		HoldID: "non-existent-hold",
	})

	if !errors.Is(err, domain.ErrHoldNotFound) {
		t.Errorf("expected ErrHoldNotFound, got: %v", err)
	}
}

func TestReleaseHold_AlreadyReleased(t *testing.T) {
	svc, txRepo, accountRepo := setupHoldService()

	account := &domain.Account{
		ID:               "acc-rel-002",
		Balance:          1000.00,
		HeldBalance:      0,
		AvailableBalance: 1000.00,
		Status:           "ACTIVE",
		Version:          0,
	}
	accountRepo.accounts["acc-rel-002"] = account

	hold := &domain.Hold{
		ID:        "hold-002",
		AccountID: "acc-rel-002",
		Amount:    200.00,
		Status:    "RELEASED",
	}
	txRepo.holds["hold-002"] = hold

	err := svc.ReleaseHold(context.Background(), &service.ReleaseHoldRequest{
		HoldID: "hold-002",
	})

	if err == nil {
		t.Error("expected error for already released hold, got nil")
	}
}

func TestGetHold_Success(t *testing.T) {
	svc, txRepo, _ := setupHoldService()

	hold := &domain.Hold{
		ID:        "hold-get-001",
		AccountID: "acc-get-001",
		Amount:    150.00,
		Status:    "ACTIVE",
	}
	txRepo.holds["hold-get-001"] = hold

	result, err := svc.GetHold(context.Background(), "hold-get-001")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.ID != "hold-get-001" {
		t.Errorf("expected hold ID hold-get-001, got %s", result.ID)
	}
}

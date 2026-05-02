package service

import (
	"context"

	"github.com/ebenezerugo/transaction-service/internal/repository"
	"go.uber.org/zap"
)

type BalanceService interface {
	GetBalance(ctx context.Context, accountID string) (*BalanceResponse, error)
}

type BalanceResponse struct {
	AccountID        string
	Balance          float64
	HeldBalance      float64
	AvailableBalance float64
	Currency         string
}

type balanceService struct {
	accountRepo repository.AccountRepository
	logger      *zap.Logger
}

func NewBalanceService(accountRepo repository.AccountRepository, logger *zap.Logger) BalanceService {
	return &balanceService{accountRepo: accountRepo, logger: logger}
}

func (s *balanceService) GetBalance(ctx context.Context, accountID string) (*BalanceResponse, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return &BalanceResponse{
		AccountID:        account.ID,
		Balance:          account.Balance,
		HeldBalance:      account.HeldBalance,
		AvailableBalance: account.AvailableBalance,
		Currency:         account.Currency,
	}, nil
}

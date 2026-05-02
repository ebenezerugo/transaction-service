package service

import (
	"context"

	"github.com/ebenezerugo/transaction-service/internal/adapters/fineract"
	"github.com/ebenezerugo/transaction-service/internal/adapters/nibss"
	"github.com/ebenezerugo/transaction-service/internal/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type TransferService interface {
	RouteTransfer(ctx context.Context, tx *domain.Transaction, fromAccount, toAccount *domain.Account) error
}

type transferService struct {
	nibssClient    nibss.NIBSSClient
	fineractClient fineract.FineractClient
	logger         *zap.Logger
}

func NewTransferService(nibssClient nibss.NIBSSClient, fineractClient fineract.FineractClient, logger *zap.Logger) TransferService {
	return &transferService{
		nibssClient:    nibssClient,
		fineractClient: fineractClient,
		logger:         logger,
	}
}

func (s *transferService) RouteTransfer(ctx context.Context, tx *domain.Transaction, fromAccount, toAccount *domain.Account) error {
	if fromAccount.BankCode == toAccount.BankCode {
		return s.routeToFineract(ctx, tx, fromAccount, toAccount)
	}
	return s.routeToNIBSS(ctx, tx, fromAccount, toAccount)
}

func (s *transferService) routeToFineract(ctx context.Context, tx *domain.Transaction, fromAccount, toAccount *domain.Account) error {
	req := &fineract.TransferRequest{
		FromAccountID:   fromAccount.ID,
		FromAccountType: fromAccount.AccountType,
		ToAccountID:     toAccount.ID,
		ToAccountType:   toAccount.AccountType,
		Amount:          tx.Amount,
		Note:            tx.Description,
	}
	_, err := s.fineractClient.Transfer(ctx, req)
	if err != nil {
		s.logger.Error("fineract transfer failed", zap.Error(err), zap.String("tx_id", tx.ID))
		return err
	}
	return nil
}

func (s *transferService) routeToNIBSS(ctx context.Context, tx *domain.Transaction, fromAccount, toAccount *domain.Account) error {
	req := &nibss.TransferRequest{
		SessionID:     uuid.New().String(),
		SourceAccount: fromAccount.AccountNumber,
		DestAccount:   toAccount.AccountNumber,
		DestBankCode:  toAccount.BankCode,
		Amount:        tx.Amount,
		Currency:      tx.Currency,
		Narration:     tx.Description,
	}
	_, err := s.nibssClient.InitiateTransfer(ctx, req)
	if err != nil {
		s.logger.Error("NIBSS transfer failed", zap.Error(err), zap.String("tx_id", tx.ID))
		return err
	}
	return nil
}

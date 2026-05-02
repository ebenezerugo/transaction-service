package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ebenezerugo/transaction-service/internal/compliance"
	"github.com/ebenezerugo/transaction-service/internal/config"
	"github.com/ebenezerugo/transaction-service/internal/domain"
	"github.com/ebenezerugo/transaction-service/internal/repository"
	"github.com/ebenezerugo/transaction-service/internal/service"
	"go.uber.org/zap"
)

// mockTransactionRepo is a mock implementation of TransactionRepository.
type mockTransactionRepo struct {
	transactions map[string]*domain.Transaction
	holds        map[string]*domain.Hold
}

func newMockTransactionRepo() *mockTransactionRepo {
	return &mockTransactionRepo{
		transactions: make(map[string]*domain.Transaction),
		holds:        make(map[string]*domain.Hold),
	}
}

func (m *mockTransactionRepo) Create(ctx context.Context, tx *domain.Transaction) error {
	m.transactions[tx.ID] = tx
	return nil
}

func (m *mockTransactionRepo) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	tx, ok := m.transactions[id]
	if !ok {
		return nil, domain.ErrTransactionNotFound
	}
	return tx, nil
}

func (m *mockTransactionRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	for _, tx := range m.transactions {
		if tx.IdempotencyKey == key {
			return tx, nil
		}
	}
	return nil, nil
}

func (m *mockTransactionRepo) UpdateStatus(ctx context.Context, id string, status domain.TransactionStatus, errMsg string) error {
	tx, ok := m.transactions[id]
	if !ok {
		return domain.ErrTransactionNotFound
	}
	tx.Status = status
	tx.ErrorMessage = errMsg
	return nil
}

func (m *mockTransactionRepo) ListByAccountID(ctx context.Context, accountID string, limit, offset int) ([]*domain.Transaction, error) {
	var result []*domain.Transaction
	for _, tx := range m.transactions {
		if tx.AccountID == accountID {
			result = append(result, tx)
		}
	}
	return result, nil
}

func (m *mockTransactionRepo) CreateHold(ctx context.Context, hold *domain.Hold) error {
	m.holds[hold.ID] = hold
	return nil
}

func (m *mockTransactionRepo) GetHoldByID(ctx context.Context, id string) (*domain.Hold, error) {
	hold, ok := m.holds[id]
	if !ok {
		return nil, domain.ErrHoldNotFound
	}
	return hold, nil
}

func (m *mockTransactionRepo) UpdateHoldStatus(ctx context.Context, id, status string) error {
	hold, ok := m.holds[id]
	if !ok {
		return domain.ErrHoldNotFound
	}
	hold.Status = status
	return nil
}

func (m *mockTransactionRepo) ListActiveHoldsByAccountID(ctx context.Context, accountID string) ([]*domain.Hold, error) {
	var result []*domain.Hold
	for _, hold := range m.holds {
		if hold.AccountID == accountID && hold.Status == "ACTIVE" {
			result = append(result, hold)
		}
	}
	return result, nil
}

// mockAccountRepo is a mock implementation of AccountRepository.
type mockAccountRepo struct {
	accounts map[string]*domain.Account
}

func newMockAccountRepo() *mockAccountRepo {
	return &mockAccountRepo{
		accounts: make(map[string]*domain.Account),
	}
}

func (m *mockAccountRepo) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	account, ok := m.accounts[id]
	if !ok {
		return nil, domain.ErrAccountNotFound
	}
	return account, nil
}

func (m *mockAccountRepo) GetByAccountNumber(ctx context.Context, accountNumber string) (*domain.Account, error) {
	for _, a := range m.accounts {
		if a.AccountNumber == accountNumber {
			return a, nil
		}
	}
	return nil, domain.ErrAccountNotFound
}

func (m *mockAccountRepo) UpdateBalance(ctx context.Context, accountID string, balance float64, version int64) error {
	account, ok := m.accounts[accountID]
	if !ok {
		return domain.ErrAccountNotFound
	}
	if account.Version != version {
		return domain.ErrOptimisticLockFailed
	}
	account.Balance = balance
	account.AvailableBalance = balance - account.HeldBalance
	account.Version++
	return nil
}

func (m *mockAccountRepo) UpdateHeldBalance(ctx context.Context, accountID string, heldBalance float64, version int64) error {
	account, ok := m.accounts[accountID]
	if !ok {
		return domain.ErrAccountNotFound
	}
	if account.Version != version {
		return domain.ErrOptimisticLockFailed
	}
	account.HeldBalance = heldBalance
	account.AvailableBalance = account.Balance - heldBalance
	account.Version++
	return nil
}

func (m *mockAccountRepo) Create(ctx context.Context, account *domain.Account) error {
	m.accounts[account.ID] = account
	return nil
}

func (m *mockAccountRepo) List(ctx context.Context, limit, offset int) ([]*domain.Account, error) {
	var result []*domain.Account
	for _, a := range m.accounts {
		result = append(result, a)
	}
	return result, nil
}

// mockAuditRepo is a mock implementation of AuditRepository.
type mockAuditRepo struct{}

func (m *mockAuditRepo) Create(ctx context.Context, event *repository.AuditLog) error {
	return nil
}

// mockProducer is a mock Kafka producer.
type mockProducer struct {
	published []publishedMsg
}

type publishedMsg struct {
	topic     string
	accountID string
	msg       interface{}
}

func (m *mockProducer) PublishTransaction(ctx context.Context, topic string, accountID string, msg interface{}) error {
	m.published = append(m.published, publishedMsg{topic: topic, accountID: accountID, msg: msg})
	return nil
}

func (m *mockProducer) Close() error { return nil }

func setupTransactionService() (service.TransactionService, *mockTransactionRepo, *mockAccountRepo, *mockProducer) {
	txRepo := newMockTransactionRepo()
	accountRepo := newMockAccountRepo()
	auditRepo := &mockAuditRepo{}
	producer := &mockProducer{}
	logger := zap.NewNop()
	cfg := &config.Config{}

	auditLogger := compliance.NewCBNAuditLogger(auditRepo, logger)
	svc := service.NewTransactionService(txRepo, accountRepo, auditLogger, producer, cfg, logger)
	return svc, txRepo, accountRepo, producer
}

func TestProcessDeposit_Success(t *testing.T) {
	svc, _, accountRepo, producer := setupTransactionService()

	account := &domain.Account{
		ID:               "acc-001",
		AccountNumber:    "0001234567",
		Balance:          1000.00,
		HeldBalance:      0,
		AvailableBalance: 1000.00,
		Currency:         "NGN",
		Status:           "ACTIVE",
		Version:          0,
	}
	accountRepo.accounts["acc-001"] = account

	tx, err := svc.ProcessDeposit(context.Background(), &service.DepositRequest{
		IdempotencyKey: "idem-001",
		AccountID:      "acc-001",
		Amount:         500.00,
		Currency:       "NGN",
		Description:    "Test deposit",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if tx == nil {
		t.Fatal("expected transaction, got nil")
	}
	if tx.Status != domain.StatusCompleted {
		t.Errorf("expected status COMPLETED, got %s", tx.Status)
	}
	if account.Balance != 1500.00 {
		t.Errorf("expected balance 1500, got %.2f", account.Balance)
	}
	if len(producer.published) != 1 {
		t.Errorf("expected 1 published message, got %d", len(producer.published))
	}
}

func TestProcessDeposit_InvalidAmount(t *testing.T) {
	svc, _, _, _ := setupTransactionService()

	_, err := svc.ProcessDeposit(context.Background(), &service.DepositRequest{
		IdempotencyKey: "idem-002",
		AccountID:      "acc-001",
		Amount:         -100.00,
		Currency:       "NGN",
	})

	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("expected ErrInvalidAmount, got: %v", err)
	}
}

func TestProcessDeposit_AccountNotFound(t *testing.T) {
	svc, _, _, _ := setupTransactionService()

	_, err := svc.ProcessDeposit(context.Background(), &service.DepositRequest{
		IdempotencyKey: "idem-003",
		AccountID:      "non-existent",
		Amount:         100.00,
		Currency:       "NGN",
	})

	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("expected ErrAccountNotFound, got: %v", err)
	}
}

func TestProcessDeposit_Idempotency(t *testing.T) {
	svc, _, accountRepo, _ := setupTransactionService()

	account := &domain.Account{
		ID:               "acc-002",
		Balance:          1000.00,
		AvailableBalance: 1000.00,
		Status:           "ACTIVE",
		Version:          0,
	}
	accountRepo.accounts["acc-002"] = account

	req := &service.DepositRequest{
		IdempotencyKey: "idem-idem",
		AccountID:      "acc-002",
		Amount:         200.00,
		Currency:       "NGN",
	}

	tx1, err1 := svc.ProcessDeposit(context.Background(), req)
	tx2, err2 := svc.ProcessDeposit(context.Background(), req)

	if err1 != nil {
		t.Fatalf("first call: expected no error, got: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("second call: expected no error, got: %v", err2)
	}
	if tx1.ID != tx2.ID {
		t.Error("idempotency failed: got different transaction IDs")
	}
}

func TestProcessWithdrawal_InsufficientBalance(t *testing.T) {
	svc, _, accountRepo, _ := setupTransactionService()

	account := &domain.Account{
		ID:               "acc-003",
		Balance:          100.00,
		AvailableBalance: 100.00,
		Status:           "ACTIVE",
		Version:          0,
	}
	accountRepo.accounts["acc-003"] = account

	_, err := svc.ProcessWithdrawal(context.Background(), &service.WithdrawalRequest{
		IdempotencyKey: "idem-004",
		AccountID:      "acc-003",
		Amount:         500.00,
		Currency:       "NGN",
	})

	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Errorf("expected ErrInsufficientBalance, got: %v", err)
	}
}

func TestProcessTransfer_Success(t *testing.T) {
	svc, _, accountRepo, _ := setupTransactionService()

	fromAccount := &domain.Account{
		ID:               "acc-from",
		AccountNumber:    "1111111111",
		BankCode:         "000001",
		Balance:          2000.00,
		AvailableBalance: 2000.00,
		Status:           "ACTIVE",
		Version:          0,
	}
	toAccount := &domain.Account{
		ID:               "acc-to",
		AccountNumber:    "2222222222",
		BankCode:         "000001",
		Balance:          500.00,
		AvailableBalance: 500.00,
		Status:           "ACTIVE",
		Version:          0,
	}
	accountRepo.accounts["acc-from"] = fromAccount
	accountRepo.accounts["acc-to"] = toAccount

	tx, err := svc.ProcessTransfer(context.Background(), &service.TransferRequest{
		IdempotencyKey: "idem-transfer-001",
		FromAccountID:  "acc-from",
		ToAccountID:    "acc-to",
		Amount:         300.00,
		Currency:       "NGN",
		Description:    "Test transfer",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if tx.Status != domain.StatusCompleted {
		t.Errorf("expected COMPLETED, got %s", tx.Status)
	}
	if fromAccount.Balance != 1700.00 {
		t.Errorf("expected from balance 1700, got %.2f", fromAccount.Balance)
	}
	if toAccount.Balance != 800.00 {
		t.Errorf("expected to balance 800, got %.2f", toAccount.Balance)
	}
}

func TestProcessUndo_Deposit(t *testing.T) {
	svc, txRepo, accountRepo, _ := setupTransactionService()

	account := &domain.Account{
		ID:               "acc-undo",
		Balance:          1500.00,
		AvailableBalance: 1500.00,
		Status:           "ACTIVE",
		Version:          0,
	}
	accountRepo.accounts["acc-undo"] = account

	origTx := &domain.Transaction{
		ID:        "tx-orig-001",
		Type:      domain.TransactionTypeDeposit,
		Status:    domain.StatusCompleted,
		AccountID: "acc-undo",
		Amount:    500.00,
		Currency:  "NGN",
		CreatedAt: time.Now(),
	}
	txRepo.transactions["tx-orig-001"] = origTx

	undoTx, err := svc.ProcessUndo(context.Background(), &service.UndoRequest{
		IdempotencyKey: "idem-undo-001",
		OriginalTxID:   "tx-orig-001",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if undoTx.Status != domain.StatusCompleted {
		t.Errorf("expected COMPLETED, got %s", undoTx.Status)
	}
	if account.Balance != 1000.00 {
		t.Errorf("expected balance 1000 after undo, got %.2f", account.Balance)
	}
}

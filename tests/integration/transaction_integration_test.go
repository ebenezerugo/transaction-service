package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ebenezerugo/transaction-service/internal/api"
	"github.com/ebenezerugo/transaction-service/internal/compliance"
	"github.com/ebenezerugo/transaction-service/internal/config"
	"github.com/ebenezerugo/transaction-service/internal/domain"
	"github.com/ebenezerugo/transaction-service/internal/repository"
	"github.com/ebenezerugo/transaction-service/internal/service"
	"go.uber.org/zap"
)

// Integration tests use in-memory mocks for speed; real DB tests require a running PostgreSQL.

type integrationAccountRepo struct {
	accounts map[string]*domain.Account
}

func newIntegrationAccountRepo() *integrationAccountRepo {
	return &integrationAccountRepo{accounts: make(map[string]*domain.Account)}
}

func (r *integrationAccountRepo) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	a, ok := r.accounts[id]
	if !ok {
		return nil, domain.ErrAccountNotFound
	}
	return a, nil
}

func (r *integrationAccountRepo) GetByAccountNumber(ctx context.Context, accountNumber string) (*domain.Account, error) {
	for _, a := range r.accounts {
		if a.AccountNumber == accountNumber {
			return a, nil
		}
	}
	return nil, domain.ErrAccountNotFound
}

func (r *integrationAccountRepo) UpdateBalance(ctx context.Context, accountID string, balance float64, version int64) error {
	a, ok := r.accounts[accountID]
	if !ok {
		return domain.ErrAccountNotFound
	}
	a.Balance = balance
	a.AvailableBalance = balance - a.HeldBalance
	a.Version++
	return nil
}

func (r *integrationAccountRepo) UpdateHeldBalance(ctx context.Context, accountID string, heldBalance float64, version int64) error {
	a, ok := r.accounts[accountID]
	if !ok {
		return domain.ErrAccountNotFound
	}
	a.HeldBalance = heldBalance
	a.AvailableBalance = a.Balance - heldBalance
	a.Version++
	return nil
}

func (r *integrationAccountRepo) Create(ctx context.Context, account *domain.Account) error {
	r.accounts[account.ID] = account
	return nil
}

func (r *integrationAccountRepo) List(ctx context.Context, limit, offset int) ([]*domain.Account, error) {
	var result []*domain.Account
	for _, a := range r.accounts {
		result = append(result, a)
	}
	return result, nil
}

type integrationTxRepo struct {
	transactions map[string]*domain.Transaction
	holds        map[string]*domain.Hold
}

func newIntegrationTxRepo() *integrationTxRepo {
	return &integrationTxRepo{
		transactions: make(map[string]*domain.Transaction),
		holds:        make(map[string]*domain.Hold),
	}
}

func (r *integrationTxRepo) Create(ctx context.Context, tx *domain.Transaction) error {
	r.transactions[tx.ID] = tx
	return nil
}

func (r *integrationTxRepo) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	tx, ok := r.transactions[id]
	if !ok {
		return nil, domain.ErrTransactionNotFound
	}
	return tx, nil
}

func (r *integrationTxRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	for _, tx := range r.transactions {
		if tx.IdempotencyKey == key {
			return tx, nil
		}
	}
	return nil, nil
}

func (r *integrationTxRepo) UpdateStatus(ctx context.Context, id string, status domain.TransactionStatus, errMsg string) error {
	tx, ok := r.transactions[id]
	if !ok {
		return domain.ErrTransactionNotFound
	}
	tx.Status = status
	tx.ErrorMessage = errMsg
	return nil
}

func (r *integrationTxRepo) ListByAccountID(ctx context.Context, accountID string, limit, offset int) ([]*domain.Transaction, error) {
	var result []*domain.Transaction
	for _, tx := range r.transactions {
		if tx.AccountID == accountID {
			result = append(result, tx)
		}
	}
	return result, nil
}

func (r *integrationTxRepo) CreateHold(ctx context.Context, hold *domain.Hold) error {
	r.holds[hold.ID] = hold
	return nil
}

func (r *integrationTxRepo) GetHoldByID(ctx context.Context, id string) (*domain.Hold, error) {
	hold, ok := r.holds[id]
	if !ok {
		return nil, domain.ErrHoldNotFound
	}
	return hold, nil
}

func (r *integrationTxRepo) UpdateHoldStatus(ctx context.Context, id, status string) error {
	hold, ok := r.holds[id]
	if !ok {
		return domain.ErrHoldNotFound
	}
	hold.Status = status
	return nil
}

func (r *integrationTxRepo) ListActiveHoldsByAccountID(ctx context.Context, accountID string) ([]*domain.Hold, error) {
	var result []*domain.Hold
	for _, hold := range r.holds {
		if hold.AccountID == accountID && hold.Status == "ACTIVE" {
			result = append(result, hold)
		}
	}
	return result, nil
}

type integrationAuditRepo struct{}

func (r *integrationAuditRepo) Create(ctx context.Context, event *repository.AuditLog) error {
	return nil
}

type integrationProducer struct{}

func (p *integrationProducer) PublishTransaction(ctx context.Context, topic string, accountID string, msg interface{}) error {
	return nil
}

func (p *integrationProducer) Close() error { return nil }

func setupTestRouter() (*httptest.Server, *integrationAccountRepo, *integrationTxRepo) {
	accountRepo := newIntegrationAccountRepo()
	txRepo := newIntegrationTxRepo()
	auditRepo := &integrationAuditRepo{}
	producer := &integrationProducer{}
	logger := zap.NewNop()
	cfg := &config.Config{}

	auditLogger := compliance.NewCBNAuditLogger(auditRepo, logger)
	txService := service.NewTransactionService(txRepo, accountRepo, auditLogger, producer, cfg, logger)
	holdService := service.NewHoldService(txRepo, accountRepo, auditLogger, logger)
	balanceService := service.NewBalanceService(accountRepo, logger)

	router := api.NewRouter(txService, holdService, balanceService, logger)
	ts := httptest.NewServer(router)
	return ts, accountRepo, txRepo
}

func TestIntegration_DepositEndpoint(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test; set INTEGRATION_TEST=1 to run")
	}

	ts, accountRepo, _ := setupTestRouter()
	defer ts.Close()

	accountRepo.accounts["integ-acc-001"] = &domain.Account{
		ID:               "integ-acc-001",
		AccountNumber:    "9999999999",
		Balance:          500.00,
		AvailableBalance: 500.00,
		Status:           "ACTIVE",
		Version:          0,
	}

	body := map[string]interface{}{
		"idempotency_key": "integ-idem-001",
		"account_id":      "integ-acc-001",
		"amount":          100.00,
		"currency":        "NGN",
		"description":     "Integration test deposit",
	}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/transactions/deposit", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != string(domain.StatusCompleted) {
		t.Errorf("expected COMPLETED, got %v", result["status"])
	}
}

func TestIntegration_TransferEndpoint(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test; set INTEGRATION_TEST=1 to run")
	}

	ts, accountRepo, _ := setupTestRouter()
	defer ts.Close()

	accountRepo.accounts["integ-from"] = &domain.Account{
		ID:               "integ-from",
		BankCode:         "000001",
		Balance:          1000.00,
		AvailableBalance: 1000.00,
		Status:           "ACTIVE",
		Version:          0,
	}
	accountRepo.accounts["integ-to"] = &domain.Account{
		ID:               "integ-to",
		BankCode:         "000001",
		Balance:          0,
		AvailableBalance: 0,
		Status:           "ACTIVE",
		Version:          0,
	}

	body := map[string]interface{}{
		"idempotency_key": "integ-transfer-001",
		"from_account_id": "integ-from",
		"to_account_id":   "integ-to",
		"amount":          250.00,
		"currency":        "NGN",
	}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/transactions/transfer", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
}

func TestIntegration_HealthEndpoint(t *testing.T) {
	ts, _, _ := setupTestRouter()
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ebenezerugo/transaction-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transactionRepository struct {
	pool *pgxpool.Pool
}

func NewTransactionRepository(pool *pgxpool.Pool) *transactionRepository {
	return &transactionRepository{pool: pool}
}

func (r *transactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	if tx.ID == "" {
		tx.ID = uuid.New().String()
	}
	metaJSON, _ := json.Marshal(tx.Metadata)
	query := `INSERT INTO transactions (id, idempotency_key, type, status, account_id, to_account_id, amount, currency, description, route, retry_count, metadata) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`
	var toAccID interface{} = tx.ToAccountID
	if tx.ToAccountID == "" {
		toAccID = nil
	}
	_, err := r.pool.Exec(ctx, query, tx.ID, tx.IdempotencyKey, string(tx.Type), string(tx.Status),
		tx.AccountID, toAccID, tx.Amount, tx.Currency, tx.Description, string(tx.Route),
		tx.RetryCount, metaJSON)
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}
	return nil
}

func (r *transactionRepository) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	query := `SELECT id, idempotency_key, type, status, account_id, COALESCE(to_account_id::text,''), amount, currency, COALESCE(description,''), COALESCE(route,''), retry_count, COALESCE(error_message,''), metadata, created_at, updated_at, completed_at FROM transactions WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	return scanTransaction(row)
}

func (r *transactionRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	query := `SELECT id, idempotency_key, type, status, account_id, COALESCE(to_account_id::text,''), amount, currency, COALESCE(description,''), COALESCE(route,''), retry_count, COALESCE(error_message,''), metadata, created_at, updated_at, completed_at FROM transactions WHERE idempotency_key = $1`
	row := r.pool.QueryRow(ctx, query, key)
	tx, err := scanTransaction(row)
	if err != nil {
		if errors.Is(err, domain.ErrTransactionNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return tx, nil
}

func (r *transactionRepository) UpdateStatus(ctx context.Context, id string, status domain.TransactionStatus, errMsg string) error {
	query := `UPDATE transactions SET status = $1, error_message = $2, updated_at = NOW(), completed_at = CASE WHEN $1 IN ('COMPLETED','FAILED','REVERSED') THEN NOW() ELSE completed_at END WHERE id = $3`
	_, err := r.pool.Exec(ctx, query, string(status), errMsg, id)
	if err != nil {
		return fmt.Errorf("update transaction status: %w", err)
	}
	return nil
}

func (r *transactionRepository) ListByAccountID(ctx context.Context, accountID string, limit, offset int) ([]*domain.Transaction, error) {
	query := `SELECT id, idempotency_key, type, status, account_id, COALESCE(to_account_id::text,''), amount, currency, COALESCE(description,''), COALESCE(route,''), retry_count, COALESCE(error_message,''), metadata, created_at, updated_at, completed_at FROM transactions WHERE account_id = $1 OR to_account_id::text = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, query, accountID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var txs []*domain.Transaction
	for rows.Next() {
		tx, err := scanTransactionRow(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

func (r *transactionRepository) CreateHold(ctx context.Context, hold *domain.Hold) error {
	if hold.ID == "" {
		hold.ID = uuid.New().String()
	}
	query := `INSERT INTO holds (id, account_id, transaction_id, amount, status, expires_at) VALUES ($1,$2,$3,$4,$5,$6)`
	var txID interface{} = hold.TransactionID
	if hold.TransactionID == "" {
		txID = nil
	}
	_, err := r.pool.Exec(ctx, query, hold.ID, hold.AccountID, txID, hold.Amount, hold.Status, hold.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create hold: %w", err)
	}
	return nil
}

func (r *transactionRepository) GetHoldByID(ctx context.Context, id string) (*domain.Hold, error) {
	query := `SELECT id, account_id, COALESCE(transaction_id::text,''), amount, status, expires_at, created_at, updated_at FROM holds WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	hold := &domain.Hold{}
	err := row.Scan(&hold.ID, &hold.AccountID, &hold.TransactionID, &hold.Amount, &hold.Status, &hold.ExpiresAt, &hold.CreatedAt, &hold.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrHoldNotFound
		}
		return nil, fmt.Errorf("get hold: %w", err)
	}
	return hold, nil
}

func (r *transactionRepository) UpdateHoldStatus(ctx context.Context, id, status string) error {
	query := `UPDATE holds SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update hold status: %w", err)
	}
	return nil
}

func (r *transactionRepository) ListActiveHoldsByAccountID(ctx context.Context, accountID string) ([]*domain.Hold, error) {
	query := `SELECT id, account_id, COALESCE(transaction_id::text,''), amount, status, expires_at, created_at, updated_at FROM holds WHERE account_id = $1 AND status = 'ACTIVE'`
	rows, err := r.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("list holds: %w", err)
	}
	defer rows.Close()

	var holds []*domain.Hold
	for rows.Next() {
		hold := &domain.Hold{}
		if err := rows.Scan(&hold.ID, &hold.AccountID, &hold.TransactionID, &hold.Amount, &hold.Status, &hold.ExpiresAt, &hold.CreatedAt, &hold.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan hold: %w", err)
		}
		holds = append(holds, hold)
	}
	return holds, nil
}

func scanTransaction(row pgx.Row) (*domain.Transaction, error) {
	tx := &domain.Transaction{}
	var metaJSON []byte
	var txType, txStatus, route string
	err := row.Scan(&tx.ID, &tx.IdempotencyKey, &txType, &txStatus, &tx.AccountID, &tx.ToAccountID,
		&tx.Amount, &tx.Currency, &tx.Description, &route, &tx.RetryCount, &tx.ErrorMessage,
		&metaJSON, &tx.CreatedAt, &tx.UpdatedAt, &tx.CompletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("scan transaction: %w", err)
	}
	tx.Type = domain.TransactionType(txType)
	tx.Status = domain.TransactionStatus(txStatus)
	tx.Route = domain.TransferRoute(route)
	if metaJSON != nil {
		_ = json.Unmarshal(metaJSON, &tx.Metadata)
	}
	return tx, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTransactionRow(row rowScanner) (*domain.Transaction, error) {
	tx := &domain.Transaction{}
	var metaJSON []byte
	var txType, txStatus, route string
	err := row.Scan(&tx.ID, &tx.IdempotencyKey, &txType, &txStatus, &tx.AccountID, &tx.ToAccountID,
		&tx.Amount, &tx.Currency, &tx.Description, &route, &tx.RetryCount, &tx.ErrorMessage,
		&metaJSON, &tx.CreatedAt, &tx.UpdatedAt, &tx.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("scan transaction row: %w", err)
	}
	tx.Type = domain.TransactionType(txType)
	tx.Status = domain.TransactionStatus(txStatus)
	tx.Route = domain.TransferRoute(route)
	if metaJSON != nil {
		_ = json.Unmarshal(metaJSON, &tx.Metadata)
	}
	return tx, nil
}

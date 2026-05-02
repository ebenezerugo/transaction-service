package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/ebenezerugo/transaction-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type accountRepository struct {
	pool *pgxpool.Pool
}

func NewAccountRepository(pool *pgxpool.Pool) *accountRepository {
	return &accountRepository{pool: pool}
}

func (r *accountRepository) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	query := `SELECT id, account_number, account_type, bank_code, balance, held_balance, available_balance, currency, status, version, created_at, updated_at FROM accounts WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	return scanAccount(row)
}

func (r *accountRepository) GetByAccountNumber(ctx context.Context, accountNumber string) (*domain.Account, error) {
	query := `SELECT id, account_number, account_type, bank_code, balance, held_balance, available_balance, currency, status, version, created_at, updated_at FROM accounts WHERE account_number = $1`
	row := r.pool.QueryRow(ctx, query, accountNumber)
	return scanAccount(row)
}

func (r *accountRepository) UpdateBalance(ctx context.Context, accountID string, balance float64, version int64) error {
	query := `UPDATE accounts SET balance = $1, version = version + 1, updated_at = NOW() WHERE id = $2 AND version = $3`
	result, err := r.pool.Exec(ctx, query, balance, accountID, version)
	if err != nil {
		return fmt.Errorf("update balance: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrOptimisticLockFailed
	}
	return nil
}

func (r *accountRepository) UpdateHeldBalance(ctx context.Context, accountID string, heldBalance float64, version int64) error {
	query := `UPDATE accounts SET held_balance = $1, version = version + 1, updated_at = NOW() WHERE id = $2 AND version = $3`
	result, err := r.pool.Exec(ctx, query, heldBalance, accountID, version)
	if err != nil {
		return fmt.Errorf("update held balance: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrOptimisticLockFailed
	}
	return nil
}

func (r *accountRepository) Create(ctx context.Context, account *domain.Account) error {
	if account.ID == "" {
		account.ID = uuid.New().String()
	}
	query := `INSERT INTO accounts (id, account_number, account_type, bank_code, balance, held_balance, currency, status, version) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0)`
	_, err := r.pool.Exec(ctx, query, account.ID, account.AccountNumber, account.AccountType, account.BankCode, account.Balance, account.HeldBalance, account.Currency, account.Status)
	if err != nil {
		return fmt.Errorf("create account: %w", err)
	}
	return nil
}

func (r *accountRepository) List(ctx context.Context, limit, offset int) ([]*domain.Account, error) {
	query := `SELECT id, account_number, account_type, bank_code, balance, held_balance, available_balance, currency, status, version, created_at, updated_at FROM accounts ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*domain.Account
	for rows.Next() {
		account := &domain.Account{}
		if err := rows.Scan(&account.ID, &account.AccountNumber, &account.AccountType, &account.BankCode,
			&account.Balance, &account.HeldBalance, &account.AvailableBalance, &account.Currency,
			&account.Status, &account.Version, &account.CreatedAt, &account.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func scanAccount(row pgx.Row) (*domain.Account, error) {
	account := &domain.Account{}
	err := row.Scan(&account.ID, &account.AccountNumber, &account.AccountType, &account.BankCode,
		&account.Balance, &account.HeldBalance, &account.AvailableBalance, &account.Currency,
		&account.Status, &account.Version, &account.CreatedAt, &account.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, fmt.Errorf("scan account: %w", err)
	}
	return account, nil
}

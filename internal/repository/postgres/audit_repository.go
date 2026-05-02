package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ebenezerugo/transaction-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type auditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *auditRepository {
	return &auditRepository{pool: pool}
}

func (r *auditRepository) Create(ctx context.Context, event *repository.AuditLog) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	dataJSON, _ := json.Marshal(event.EventData)

	var txID interface{} = event.TransactionID
	if event.TransactionID == "" {
		txID = nil
	}
	var accID interface{} = event.AccountID
	if event.AccountID == "" {
		accID = nil
	}

	query := `INSERT INTO audit_logs (id, transaction_id, account_id, event_type, event_data, operator_id, ip_address) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := r.pool.Exec(ctx, query, event.ID, txID, accID, event.EventType, dataJSON, event.OperatorID, event.IPAddress)
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

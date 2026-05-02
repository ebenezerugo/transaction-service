package compliance

import (
	"context"
	"time"

	"github.com/ebenezerugo/transaction-service/internal/repository"
	"go.uber.org/zap"
)

type CBNAuditLogger interface {
	LogTransaction(ctx context.Context, event AuditEvent) error
	LogTransactionEvent(ctx context.Context, txID string, eventType string, data interface{}) error
}

type AuditEvent struct {
	TransactionID string
	AccountID     string
	EventType     string
	EventData     interface{}
	OperatorID    string
	IPAddress     string
	Timestamp     time.Time
}

type cbnAuditLogger struct {
	auditRepo repository.AuditRepository
	logger    *zap.Logger
}

func NewCBNAuditLogger(auditRepo repository.AuditRepository, logger *zap.Logger) CBNAuditLogger {
	return &cbnAuditLogger{auditRepo: auditRepo, logger: logger}
}

func (a *cbnAuditLogger) LogTransaction(ctx context.Context, event AuditEvent) error {
	log := &repository.AuditLog{
		TransactionID: event.TransactionID,
		AccountID:     event.AccountID,
		EventType:     event.EventType,
		EventData:     event.EventData,
		OperatorID:    event.OperatorID,
		IPAddress:     event.IPAddress,
	}
	if err := a.auditRepo.Create(ctx, log); err != nil {
		a.logger.Error("failed to create audit log", zap.Error(err), zap.String("event_type", event.EventType))
		return err
	}
	return nil
}

func (a *cbnAuditLogger) LogTransactionEvent(ctx context.Context, txID string, eventType string, data interface{}) error {
	return a.LogTransaction(ctx, AuditEvent{
		TransactionID: txID,
		EventType:     eventType,
		EventData:     data,
		Timestamp:     time.Now(),
	})
}

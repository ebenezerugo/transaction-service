package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ebenezerugo/transaction-service/internal/api"
	"github.com/ebenezerugo/transaction-service/internal/compliance"
	"github.com/ebenezerugo/transaction-service/internal/config"
	"github.com/ebenezerugo/transaction-service/internal/kafka"
	"github.com/ebenezerugo/transaction-service/internal/repository/postgres"
	"github.com/ebenezerugo/transaction-service/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func Run() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := config.Load()

	pool, err := pgxpool.New(context.Background(), cfg.Database.DSN)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	accountRepo := postgres.NewAccountRepository(pool)
	txRepo := postgres.NewTransactionRepository(pool)
	auditRepo := postgres.NewAuditRepository(pool)

	producer, err := kafka.NewProducer(cfg.Kafka.Brokers)
	if err != nil {
		logger.Warn("failed to create kafka producer", zap.Error(err))
	}
	if producer != nil {
		defer producer.Close()
	}

	auditLogger := compliance.NewCBNAuditLogger(auditRepo, logger)

	txService := service.NewTransactionService(txRepo, accountRepo, auditLogger, producer, cfg, logger)
	holdService := service.NewHoldService(txRepo, accountRepo, auditLogger, logger)
	balanceService := service.NewBalanceService(accountRepo, logger)

	router := api.NewRouter(txService, holdService, balanceService, logger)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Server.Port),
		Handler: router,
	}

	go func() {
		logger.Info("starting server", zap.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	logger.Info("server stopped")
}

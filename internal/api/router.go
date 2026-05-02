package api

import (
	"github.com/ebenezerugo/transaction-service/internal/api/handlers"
	"github.com/ebenezerugo/transaction-service/internal/api/middleware"
	"github.com/ebenezerugo/transaction-service/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(
	txService service.TransactionService,
	holdService service.HoldService,
	balanceService service.BalanceService,
	logger *zap.Logger,
) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(logger))
	router.Use(middleware.RequestID())

	txHandler := handlers.NewTransactionHandler(txService)
	wsHandler := handlers.NewWebSocketHandler(logger)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	router.GET("/ws", wsHandler.HandleConnection)

	v1 := router.Group("/api/v1")
	v1.Use(middleware.Auth())
	v1.Use(middleware.ValidateContentType())

	txRoutes := v1.Group("/transactions")
	{
		txRoutes.POST("/deposit", txHandler.Deposit)
		txRoutes.POST("/withdrawal", txHandler.Withdrawal)
		txRoutes.POST("/transfer", txHandler.Transfer)
		txRoutes.POST("/undo", txHandler.Undo)
		txRoutes.POST("/adjust", txHandler.Adjust)
		txRoutes.GET("/:id", txHandler.GetTransaction)
		txRoutes.GET("/account/:account_id", txHandler.ListTransactionsByAccount)
	}

	return router
}

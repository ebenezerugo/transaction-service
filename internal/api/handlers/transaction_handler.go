package handlers

import (
	"net/http"
	"strconv"

	"github.com/ebenezerugo/transaction-service/internal/domain"
	"github.com/ebenezerugo/transaction-service/internal/service"
	"github.com/gin-gonic/gin"
)

type TransactionHandler struct {
	txService service.TransactionService
}

func NewTransactionHandler(txService service.TransactionService) *TransactionHandler {
	return &TransactionHandler{txService: txService}
}

type depositRequest struct {
	IdempotencyKey string  `json:"idempotency_key" binding:"required"`
	AccountID      string  `json:"account_id" binding:"required"`
	Amount         float64 `json:"amount" binding:"required,gt=0"`
	Currency       string  `json:"currency" binding:"required"`
	Description    string  `json:"description"`
}

type withdrawalRequest struct {
	IdempotencyKey string  `json:"idempotency_key" binding:"required"`
	AccountID      string  `json:"account_id" binding:"required"`
	Amount         float64 `json:"amount" binding:"required,gt=0"`
	Currency       string  `json:"currency" binding:"required"`
	Description    string  `json:"description"`
}

type transferRequest struct {
	IdempotencyKey string  `json:"idempotency_key" binding:"required"`
	FromAccountID  string  `json:"from_account_id" binding:"required"`
	ToAccountID    string  `json:"to_account_id" binding:"required"`
	Amount         float64 `json:"amount" binding:"required,gt=0"`
	Currency       string  `json:"currency" binding:"required"`
	Description    string  `json:"description"`
}

type undoRequest struct {
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
	OriginalTxID   string `json:"original_tx_id" binding:"required"`
	AccountID      string `json:"account_id"`
}

type adjustRequest struct {
	IdempotencyKey string  `json:"idempotency_key" binding:"required"`
	AccountID      string  `json:"account_id" binding:"required"`
	Amount         float64 `json:"amount" binding:"required"`
	Currency       string  `json:"currency" binding:"required"`
	Description    string  `json:"description"`
}

func (h *TransactionHandler) Deposit(c *gin.Context) {
	var req depositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	operatorID, _ := c.Get("operator_id")
	tx, err := h.txService.ProcessDeposit(c.Request.Context(), &service.DepositRequest{
		IdempotencyKey: req.IdempotencyKey,
		AccountID:      req.AccountID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Description:    req.Description,
		OperatorID:     operatorID.(string),
	})
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, tx)
}

func (h *TransactionHandler) Withdrawal(c *gin.Context) {
	var req withdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	operatorID, _ := c.Get("operator_id")
	tx, err := h.txService.ProcessWithdrawal(c.Request.Context(), &service.WithdrawalRequest{
		IdempotencyKey: req.IdempotencyKey,
		AccountID:      req.AccountID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Description:    req.Description,
		OperatorID:     operatorID.(string),
	})
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, tx)
}

func (h *TransactionHandler) Transfer(c *gin.Context) {
	var req transferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	operatorID, _ := c.Get("operator_id")
	tx, err := h.txService.ProcessTransfer(c.Request.Context(), &service.TransferRequest{
		IdempotencyKey: req.IdempotencyKey,
		FromAccountID:  req.FromAccountID,
		ToAccountID:    req.ToAccountID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Description:    req.Description,
		OperatorID:     operatorID.(string),
	})
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, tx)
}

func (h *TransactionHandler) Undo(c *gin.Context) {
	var req undoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	operatorID, _ := c.Get("operator_id")
	tx, err := h.txService.ProcessUndo(c.Request.Context(), &service.UndoRequest{
		IdempotencyKey: req.IdempotencyKey,
		OriginalTxID:   req.OriginalTxID,
		AccountID:      req.AccountID,
		OperatorID:     operatorID.(string),
	})
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, tx)
}

func (h *TransactionHandler) Adjust(c *gin.Context) {
	var req adjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	operatorID, _ := c.Get("operator_id")
	tx, err := h.txService.ProcessAdjust(c.Request.Context(), &service.AdjustRequest{
		IdempotencyKey: req.IdempotencyKey,
		AccountID:      req.AccountID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Description:    req.Description,
		OperatorID:     operatorID.(string),
	})
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, tx)
}

func (h *TransactionHandler) GetTransaction(c *gin.Context) {
	id := c.Param("id")
	tx, err := h.txService.GetTransaction(c.Request.Context(), id)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, tx)
}

func (h *TransactionHandler) ListTransactionsByAccount(c *gin.Context) {
	accountID := c.Param("account_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	txs, err := h.txService.GetTransactionsByAccount(c.Request.Context(), accountID, limit, offset)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"transactions": txs, "limit": limit, "offset": offset})
}

func handleServiceError(c *gin.Context, err error) {
	switch err {
	case domain.ErrAccountNotFound, domain.ErrTransactionNotFound, domain.ErrHoldNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case domain.ErrInsufficientBalance, domain.ErrAccountInactive, domain.ErrInvalidAmount, domain.ErrInvalidTransactionType:
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case domain.ErrDuplicateTransaction:
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case domain.ErrOptimisticLockFailed:
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

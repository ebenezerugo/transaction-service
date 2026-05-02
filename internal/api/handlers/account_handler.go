package handlers

import (
	"net/http"
	"strconv"

	"github.com/ebenezerugo/transaction-service/internal/domain"
	"github.com/ebenezerugo/transaction-service/internal/repository"
	"github.com/ebenezerugo/transaction-service/internal/service"
	"github.com/gin-gonic/gin"
)

type AccountHandler struct {
	accountRepo repository.AccountRepository
	balanceSvc  service.BalanceService
	holdSvc     service.HoldService
}

func NewAccountHandler(accountRepo repository.AccountRepository, balanceSvc service.BalanceService, holdSvc service.HoldService) *AccountHandler {
	return &AccountHandler{
		accountRepo: accountRepo,
		balanceSvc:  balanceSvc,
		holdSvc:     holdSvc,
	}
}

type createAccountRequest struct {
	AccountNumber string  `json:"account_number" binding:"required"`
	AccountType   string  `json:"account_type" binding:"required"`
	BankCode      string  `json:"bank_code" binding:"required"`
	Currency      string  `json:"currency" binding:"required"`
	InitialBalance float64 `json:"initial_balance"`
}

type placeHoldRequest struct {
	AccountID     string  `json:"account_id" binding:"required"`
	TransactionID string  `json:"transaction_id"`
	Amount        float64 `json:"amount" binding:"required,gt=0"`
}

func (h *AccountHandler) CreateAccount(c *gin.Context) {
	var req createAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	account := &domain.Account{
		AccountNumber: req.AccountNumber,
		AccountType:   req.AccountType,
		BankCode:      req.BankCode,
		Currency:      req.Currency,
		Balance:       req.InitialBalance,
		Status:        "ACTIVE",
	}

	if err := h.accountRepo.Create(c.Request.Context(), account); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create account"})
		return
	}
	c.JSON(http.StatusCreated, account)
}

func (h *AccountHandler) GetAccount(c *gin.Context) {
	id := c.Param("id")
	account, err := h.accountRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, account)
}

func (h *AccountHandler) ListAccounts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	accounts, err := h.accountRepo.List(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list accounts"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accounts": accounts, "limit": limit, "offset": offset})
}

func (h *AccountHandler) GetBalance(c *gin.Context) {
	id := c.Param("id")
	balance, err := h.balanceSvc.GetBalance(c.Request.Context(), id)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, balance)
}

func (h *AccountHandler) PlaceHold(c *gin.Context) {
	var req placeHoldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	operatorID, _ := c.Get("operator_id")
	hold, err := h.holdSvc.PlaceHold(c.Request.Context(), &service.PlaceHoldRequest{
		AccountID:     req.AccountID,
		TransactionID: req.TransactionID,
		Amount:        req.Amount,
		OperatorID:    operatorID.(string),
	})
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, hold)
}

func (h *AccountHandler) ReleaseHold(c *gin.Context) {
	holdID := c.Param("hold_id")
	operatorID, _ := c.Get("operator_id")

	if err := h.holdSvc.ReleaseHold(c.Request.Context(), &service.ReleaseHoldRequest{
		HoldID:     holdID,
		OperatorID: operatorID.(string),
	}); err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "hold released"})
}

func (h *AccountHandler) GetHold(c *gin.Context) {
	holdID := c.Param("hold_id")
	hold, err := h.holdSvc.GetHold(c.Request.Context(), holdID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, hold)
}

package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// WalletHandler handles wallet and withdrawal requests
type WalletHandler struct {
	walletRepo      *database.LoungeOwnerWalletRepository
	loungeOwnerRepo *database.LoungeOwnerRepository
	bankRepo        *database.LoungeOwnerBankDetailsRepository
	bankLinkRepo    *database.LoungeOwnerBankLinkRepository
}

// NewWalletHandler creates a new WalletHandler
func NewWalletHandler(
	walletRepo *database.LoungeOwnerWalletRepository,
	loungeOwnerRepo *database.LoungeOwnerRepository,
	bankRepo *database.LoungeOwnerBankDetailsRepository,
	bankLinkRepo *database.LoungeOwnerBankLinkRepository,
) *WalletHandler {
	return &WalletHandler{
		walletRepo:      walletRepo,
		loungeOwnerRepo: loungeOwnerRepo,
		bankRepo:        bankRepo,
		bankLinkRepo:    bankLinkRepo,
	}
}

func (h *WalletHandler) getOwnerIDFromUser(c *gin.Context) (uuid.UUID, bool) {
	userCtx, ok := middleware.GetUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "User context not found"})
		return uuid.Nil, false
	}

	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: failed to get lounge owner by user id %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database_error", Message: "Failed to resolve lounge owner"})
		return uuid.Nil, false
	}

	if owner == nil {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "not_lounge_owner", Message: "Lounge owner account not found"})
		return uuid.Nil, false
	}

	return owner.ID, true
}

func (h *WalletHandler) getAdminID(c *gin.Context) uuid.UUID {
	if userCtx, ok := middleware.GetUserContext(c); ok {
		return userCtx.UserID
	}
	return uuid.Nil
}

// ============================================================================
// LOUNGE OWNER WALLET ENDPOINTS
// ============================================================================

// GetWalletStatus handles GET /api/v1/lounge-owner/wallet/status
func (h *WalletHandler) GetWalletStatus(c *gin.Context) {
	ownerID, ok := h.getOwnerIDFromUser(c)
	if !ok {
		return
	}

	wallet, err := h.walletRepo.GetOrCreateWallet(ownerID)
	if err != nil {
		log.Printf("ERROR: get or create wallet for owner %s: %v", ownerID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database_error", Message: "Failed to fetch wallet"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"wallet_balance": wallet.Balance,
		"owner_id":       wallet.OwnerID.String(),
		"wallet":         wallet,
	})
}

// RequestWithdrawal handles POST /api/v1/lounge-owner/wallet/withdraw
func (h *WalletHandler) RequestWithdrawal(c *gin.Context) {
	ownerID, ok := h.getOwnerIDFromUser(c)
	if !ok {
		return
	}

	var req models.OwnerWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: err.Error()})
		return
	}

	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "Withdrawal amount must be greater than 0"})
		return
	}

	bankDetailsUUID, err := uuid.Parse(req.BankDetailsID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "Invalid bank_details_id"})
		return
	}

	// Verify bank details belongs to owner via bank links
	links, err := h.bankLinkRepo.ListByOwner(ownerID)
	if err != nil {
		log.Printf("ERROR: list bank links for owner %s: %v", ownerID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database_error", Message: "Failed to verify bank details"})
		return
	}

	bankFound := false
	for _, l := range links {
		if l.BankDetailsID == bankDetailsUUID {
			bankFound = true
			break
		}
	}

	if !bankFound {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid_bank", Message: "The specified bank account is not linked to your lounge account"})
		return
	}

	tx, err := h.walletRepo.DebitWallet(ownerID, req.Amount, req.Description, bankDetailsUUID)
	if err != nil {
		log.Printf("ERROR: debit wallet for owner %s: %v", ownerID, err)
		if strings.Contains(err.Error(), "insufficient balance") {
			c.JSON(http.StatusUnprocessableEntity, ErrorResponse{
				Error:   "insufficient_balance",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "transaction_failed", Message: err.Error()})
		return
	}

	wallet, _ := h.walletRepo.GetWalletByOwnerID(ownerID)
	balance := 0.0
	if wallet != nil {
		balance = wallet.Balance
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Withdrawal request submitted successfully",
		"transaction":    tx,
		"wallet_balance": balance,
	})
}

// ListMyTransactions handles GET /api/v1/lounge-owner/wallet/transactions
func (h *WalletHandler) ListMyTransactions(c *gin.Context) {
	ownerID, ok := h.getOwnerIDFromUser(c)
	if !ok {
		return
	}

	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}
	if o := c.Query("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	txs, err := h.walletRepo.ListTransactions(ownerID, limit, offset)
	if err != nil {
		log.Printf("ERROR: list transactions for owner %s: %v", ownerID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database_error", Message: "Failed to fetch transactions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": txs,
		"count":        len(txs),
	})
}

// ============================================================================
// ADMIN WALLET ENDPOINTS
// ============================================================================

// AdminSettleOwner handles POST /api/v1/admin/wallet/settle-owner
func (h *WalletHandler) AdminSettleOwner(c *gin.Context) {
	var req models.AdminSettlementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: err.Error()})
		return
	}

	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "Settlement amount must be greater than 0"})
		return
	}

	ownerUUID, err := uuid.Parse(req.OwnerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "Invalid owner_id"})
		return
	}

	// Validate owner exists
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByID(ownerUUID)
	if err != nil {
		log.Printf("ERROR: get lounge owner %s: %v", ownerUUID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database_error", Message: "Failed to verify lounge owner"})
		return
	}
	if owner == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Lounge owner not found"})
		return
	}

	// Parse 14-day period dates
	startDate, err := time.Parse("2006-01-02", req.PeriodStart)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "Invalid period_start format (expected YYYY-MM-DD)"})
		return
	}

	endDate, err := time.Parse("2006-01-02", req.PeriodEnd)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "Invalid period_end format (expected YYYY-MM-DD)"})
		return
	}

	if endDate.Before(startDate) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "period_end must be after period_start"})
		return
	}

	// Ensure wallet exists
	if _, err := h.walletRepo.GetOrCreateWallet(ownerUUID); err != nil {
		log.Printf("ERROR: get or create wallet for owner %s: %v", ownerUUID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database_error", Message: "Failed to initialize wallet"})
		return
	}

	adminID := h.getAdminID(c)

	desc := req.Description
	if desc == "" {
		desc = "14-day earnings settlement (" + req.PeriodStart + " to " + req.PeriodEnd + ")"
	}

	tx, err := h.walletRepo.CreditWallet(ownerUUID, req.Amount, desc, startDate, endDate, adminID)
	if err != nil {
		log.Printf("ERROR: credit wallet for owner %s: %v", ownerUUID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "transaction_failed", Message: err.Error()})
		return
	}

	wallet, _ := h.walletRepo.GetWalletByOwnerID(ownerUUID)
	balance := 0.0
	if wallet != nil {
		balance = wallet.Balance
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Settlement processed successfully",
		"transaction":    tx,
		"wallet_balance": balance,
	})
}

// AdminListAllTransactions handles GET /api/v1/admin/wallet/transactions
func (h *WalletHandler) AdminListAllTransactions(c *gin.Context) {
	limit := 50
	offset := 0
	if l := c.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}
	if o := c.Query("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	txs, err := h.walletRepo.ListAllTransactionsForAdmin(limit, offset)
	if err != nil {
		log.Printf("ERROR: list all transactions for admin: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database_error", Message: "Failed to fetch transactions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": txs,
		"count":        len(txs),
	})
}

// AdminGetOwnerWallet handles GET /api/v1/admin/wallet/owner/:owner_id
func (h *WalletHandler) AdminGetOwnerWallet(c *gin.Context) {
	ownerIDStr := c.Param("owner_id")
	ownerUUID, err := uuid.Parse(ownerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "Invalid owner_id"})
		return
	}

	wallet, err := h.walletRepo.GetOrCreateWallet(ownerUUID)
	if err != nil {
		log.Printf("ERROR: get or create wallet for owner %s: %v", ownerUUID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database_error", Message: "Failed to fetch wallet"})
		return
	}

	txs, err := h.walletRepo.ListTransactions(ownerUUID, 20, 0)
	if err != nil {
		log.Printf("WARN: failed to fetch recent transactions for owner %s: %v", ownerUUID, err)
		txs = []models.LoungeOwnerWalletTransaction{}
	}

	c.JSON(http.StatusOK, gin.H{
		"wallet":              wallet,
		"recent_transactions": txs,
	})
}

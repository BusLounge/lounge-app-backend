package handlers

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// StaffCardHandler handles payment card operations saved to staff_bank_details
type StaffCardHandler struct {
	repo *database.StaffBankDetailsRepository
}

// NewStaffCardHandler creates a new handler
func NewStaffCardHandler(repo *database.StaffBankDetailsRepository) *StaffCardHandler {
	return &StaffCardHandler{repo: repo}
}

func detectCardBrand(num string) string {
	clean := strings.ReplaceAll(num, " ", "")
	clean = strings.ReplaceAll(clean, "-", "")
	if strings.HasPrefix(clean, "4") {
		return "Visa"
	}
	if strings.HasPrefix(clean, "51") || strings.HasPrefix(clean, "52") ||
		strings.HasPrefix(clean, "53") || strings.HasPrefix(clean, "54") || strings.HasPrefix(clean, "55") {
		return "Mastercard"
	}
	if strings.HasPrefix(clean, "34") || strings.HasPrefix(clean, "37") {
		return "American Express"
	}
	if strings.HasPrefix(clean, "6011") || strings.HasPrefix(clean, "65") {
		return "Discover"
	}
	return "Card"
}

// AddCard handles POST /api/v1/lounge-owner/cards
func (h *StaffCardHandler) AddCard(c *gin.Context) {
	userCtx, ok := middleware.GetUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "User context not found"})
		return
	}

	var req models.AddCardPaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: err.Error()})
		return
	}

	cardBrand := req.CardBrand
	if cardBrand == "" {
		cardBrand = detectCardBrand(req.CardNumber)
	}

	brandCode := strings.ToUpper(cardBrand)
	expDate := req.ExpirationDate
	branch := req.CountryOrRegion
	if req.AddressLine1 != "" {
		branch = req.CountryOrRegion + " - " + req.AddressLine1
	}

	cardType := req.CardType
	if cardType == "" {
		cardType = "card"
	}

	detail := &models.StaffBankDetail{
		UserID:            userCtx.UserID,
		BankName:          cardBrand,
		BankCode:          &brandCode,
		BranchName:        branch,
		BranchCode:        &expDate,
		AccountNumber:     req.CardNumber,
		AccountHolderName: req.FullName,
		AccountType:       &cardType,
		IsDefault:         true,
	}

	saved, err := h.repo.Create(detail)
	if err != nil {
		log.Printf("ERROR: failed to save card to staff_bank_details: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database_error", Message: "Failed to save payment card"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Payment method saved successfully",
		"card":    saved,
	})
}

// ListCards handles GET /api/v1/lounge-owner/cards
func (h *StaffCardHandler) ListCards(c *gin.Context) {
	userCtx, ok := middleware.GetUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "User context not found"})
		return
	}

	cards, err := h.repo.ListByUserID(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: failed to list cards for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database_error", Message: "Failed to list cards"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cards": cards,
	})
}

// DeleteCard handles DELETE /api/v1/lounge-owner/cards/:id
func (h *StaffCardHandler) DeleteCard(c *gin.Context) {
	userCtx, ok := middleware.GetUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "User context not found"})
		return
	}

	idStr := c.Param("id")
	cardID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "Invalid card ID"})
		return
	}

	if err := h.repo.DeleteByID(cardID, userCtx.UserID); err != nil {
		log.Printf("ERROR: failed to delete card: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "database_error", Message: "Failed to delete card"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Card deleted successfully"})
}

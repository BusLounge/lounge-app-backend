package handlers

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/services"
)

// InventoryHandler handles lounge inventory-related HTTP requests
type InventoryHandler struct {
	inventoryService *services.InventoryService
	loungeOwnerRepo  *database.LoungeOwnerRepository
	loungeRepo       *database.LoungeRepository
}

// NewInventoryHandler creates a new inventory handler
func NewInventoryHandler(
	inventoryService *services.InventoryService,
	loungeOwnerRepo *database.LoungeOwnerRepository,
	loungeRepo *database.LoungeRepository,
) *InventoryHandler {
	return &InventoryHandler{
		inventoryService: inventoryService,
		loungeOwnerRepo:  loungeOwnerRepo,
		loungeRepo:       loungeRepo,
	}
}

// GetAvailableCatalog handles GET /api/v1/lounges/:id/inventory/catalog
func (h *InventoryHandler) GetAvailableCatalog(c *gin.Context) {
	// Authenticate the Lounge Owner and resolve the lounge
	loungeIDStr := c.Param("id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Verify ownership (or staff access) — handle each error independently
	owner, ownerErr := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if ownerErr != nil {
		log.Printf("ERROR: Failed to get lounge owner for user %s: %v", userCtx.UserID, ownerErr)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to verify ownership",
		})
		return
	}

	lounge, loungeErr := h.loungeRepo.GetLoungeByID(loungeID)
	if loungeErr != nil {
		log.Printf("ERROR: Failed to get lounge %s: %v", loungeID, loungeErr)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounge",
		})
		return
	}
	if lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}

	isAuthorized := owner != nil && lounge.LoungeOwnerID == owner.ID
	if !isAuthorized {
		log.Printf("WARN: Catalog access denied — userID=%s, ownerID=%v, loungeOwnerID=%s",
			userCtx.UserID, owner, lounge.LoungeOwnerID)
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Not authorized to access this lounge",
		})
		return
	}

	search := c.Query("search")
	categoryID := c.Query("category_id")

	items, err := h.inventoryService.GetAvailableCatalog(loungeID, search, categoryID)
	if err != nil {
		log.Printf("ERROR: Failed to get available catalog: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve catalog",
		})
		return
	}

	log.Printf("INFO: Catalog for lounge %s returned %d items", loungeID, len(items))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    items,
	})
}

// AddInventoryItem handles POST /api/v1/lounges/:id/inventory
func (h *InventoryHandler) AddInventoryItem(c *gin.Context) {
	loungeIDStr := c.Param("id")
	loungeID, err := uuid.Parse(loungeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid lounge ID format",
		})
		return
	}

	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Verify ownership — handle each error independently
	owner, ownerErr := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if ownerErr != nil {
		log.Printf("ERROR: Failed to get lounge owner for user %s: %v", userCtx.UserID, ownerErr)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to verify ownership",
		})
		return
	}

	lounge, loungeErr := h.loungeRepo.GetLoungeByID(loungeID)
	if loungeErr != nil {
		log.Printf("ERROR: Failed to get lounge %s: %v", loungeID, loungeErr)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve lounge",
		})
		return
	}
	if lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}

	if owner == nil || lounge.LoungeOwnerID != owner.ID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Not authorized to access this lounge",
		})
		return
	}

	var req services.EnrollInventoryItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "Invalid request body",
		})
		return
	}

	product, err := h.inventoryService.EnrollInventoryItem(&req, loungeID, userCtx.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "already been added") {
			c.JSON(http.StatusConflict, ErrorResponse{
				Error:   "conflict",
				Message: err.Error(),
			})
			return
		}
		if strings.Contains(err.Error(), "not found or inactive") {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
			return
		}
		if strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "cannot be") || strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "after start_at") {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: err.Error(),
			})
			return
		}

		log.Printf("ERROR: Failed to enroll inventory item: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to enroll inventory item",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Item added to inventory successfully",
		"data":    product,
	})
}

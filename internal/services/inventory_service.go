package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// InventoryService handles business logic for lounge inventory
type InventoryService struct {
	inventoryRepo *database.InventoryRepository
}

// NewInventoryService creates a new inventory service
func NewInventoryService(inventoryRepo *database.InventoryRepository) *InventoryService {
	return &InventoryService{
		inventoryRepo: inventoryRepo,
	}
}

// GetAvailableCatalog retrieves master inventory items available for a lounge to add
func (s *InventoryService) GetAvailableCatalog(loungeID uuid.UUID, search string, categoryID string) ([]models.MasterInventoryItem, error) {
	return s.inventoryRepo.GetAvailableInventoryItems(loungeID, search, categoryID)
}

// EnrollInventoryItemRequest contains data for adding a new item to lounge inventory
type EnrollInventoryItemRequest struct {
	InventoryItemID string   `json:"inventory_item_id"`
	SellingPrice    float64  `json:"selling_price"`
	InitialStock    int      `json:"initial_stock"`
	ReorderLevel    int      `json:"reorder_level"`
	IsAvailable     bool     `json:"is_available"`
	Discount        *DiscountRequest `json:"discount,omitempty"`
}

// DiscountRequest contains data for an optional discount
type DiscountRequest struct {
	DiscountName  string    `json:"discount_name"`
	DiscountType  string    `json:"discount_type"` // PERCENTAGE or FIXED_AMOUNT
	DiscountValue float64   `json:"discount_value"`
	StartAt       time.Time `json:"start_at"`
	EndAt         time.Time `json:"end_at"`
	IsActive      bool      `json:"is_active"`
}

// EnrollInventoryItem enrolls a master item into the lounge's inventory
func (s *InventoryService) EnrollInventoryItem(req *EnrollInventoryItemRequest, loungeID uuid.UUID, userID uuid.UUID) (*models.LoungeProduct, error) {
	// 1. Validation
	masterID, err := uuid.Parse(req.InventoryItemID)
	if err != nil {
		return nil, errors.New("invalid inventory_item_id format")
	}

	if req.SellingPrice <= 0 {
		return nil, errors.New("selling_price must be greater than 0")
	}

	if req.InitialStock < 0 {
		return nil, errors.New("initial_stock cannot be negative")
	}

	if req.ReorderLevel < 0 {
		return nil, errors.New("reorder_level cannot be negative")
	}

	if req.Discount != nil {
		if req.Discount.DiscountName == "" {
			return nil, errors.New("discount_name is required when discount is provided")
		}
		if req.Discount.DiscountType != string(models.DiscountTypePercentage) && req.Discount.DiscountType != string(models.DiscountTypeFixedAmount) {
			return nil, errors.New("discount_type must be PERCENTAGE or FIXED_AMOUNT")
		}
		if req.Discount.DiscountValue <= 0 {
			return nil, errors.New("discount_value must be greater than 0")
		}
		if req.Discount.DiscountType == string(models.DiscountTypePercentage) && req.Discount.DiscountValue > 100 {
			return nil, errors.New("percentage discount_value cannot exceed 100")
		}
		if req.Discount.EndAt.Before(req.Discount.StartAt) || req.Discount.EndAt.Equal(req.Discount.StartAt) {
			return nil, errors.New("end_at must be after start_at")
		}
	}

	// 2. Verify Master Item
	masterItem, err := s.inventoryRepo.GetInventoryMasterItemByID(masterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get master item: %w", err)
	}
	if masterItem == nil {
		return nil, errors.New("master inventory item not found or inactive")
	}

	// 3. Check for Duplicates
	exists, err := s.inventoryRepo.CheckLoungeInventoryItemExists(loungeID, masterID)
	if err != nil {
		return nil, fmt.Errorf("failed to check for duplicate item: %w", err)
	}
	if exists {
		return nil, errors.New("this item has already been added to your inventory")
	}

	// 4. Prepare Models for Insert
	now := time.Now().UTC()
	loungeProductID := uuid.New()

	// Calculate discounted price if applicable
	var discountedPrice *string
	if req.Discount != nil && req.Discount.IsActive && req.Discount.StartAt.Before(now) && req.Discount.EndAt.After(now) {
		var finalPrice float64
		if req.Discount.DiscountType == string(models.DiscountTypePercentage) {
			finalPrice = req.SellingPrice - (req.SellingPrice * req.Discount.DiscountValue / 100)
		} else {
			finalPrice = req.SellingPrice - req.Discount.DiscountValue
		}
		
		if finalPrice < 0 {
			finalPrice = 0
		}
		
		formattedPrice := fmt.Sprintf("%.2f", finalPrice)
		discountedPrice = &formattedPrice
	}

	// Determine stock status based on initial stock
	stockStatus := models.LoungeProductStockStatusInStock
	if req.InitialStock == 0 {
		stockStatus = models.LoungeProductStockStatusOutOfStock
	}

	loungeProduct := &models.LoungeProduct{
		ID:              loungeProductID,
		LoungeID:        loungeID,
		CategoryID:      masterItem.CategoryID,
		Name:            masterItem.Name,
		Description:     masterItem.Description,
		ProductType:     models.LoungeProductTypeProduct,
		Price:           fmt.Sprintf("%.2f", req.SellingPrice),
		DiscountedPrice: discountedPrice,
		ImageURL:        masterItem.ImageURL,
		InventoryItemID: &masterItem.ID,
		StockStatus:     stockStatus,
		StockQuantity:   &req.InitialStock,
		ReorderLevel:    &req.ReorderLevel,
		IsAvailable:     req.IsAvailable,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
		PriceRateType:   "fixed_rate",
	}

	// Transaction logic
	var inventoryTx *models.InventoryTransaction
	if req.InitialStock > 0 {
		reason := "Initial stock"
		inventoryTx = &models.InventoryTransaction{
			ID:              uuid.New(),
			LoungeID:        loungeID,
			LoungeProductID: loungeProductID,
			TransactionType: models.TransactionTypeStockIn,
			Quantity:        req.InitialStock,
			StockBefore:     0,
			StockAfter:      req.InitialStock,
			Reason:          &reason,
			CreatedByUserID: userID,
			CreatedAt:       now,
		}
	}

	var discount *models.InventoryDiscount
	if req.Discount != nil {
		discount = &models.InventoryDiscount{
			ID:              uuid.New(),
			LoungeProductID: loungeProductID,
			DiscountName:    req.Discount.DiscountName,
			DiscountType:    models.DiscountType(req.Discount.DiscountType),
			DiscountValue:   fmt.Sprintf("%.2f", req.Discount.DiscountValue),
			StartAt:         req.Discount.StartAt,
			EndAt:           req.Discount.EndAt,
			IsActive:        req.Discount.IsActive,
			CreatedByUserID: userID,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
	}

	// 5. Execute DB Transaction
	err = s.inventoryRepo.CreateLoungeProductTx(loungeProduct, inventoryTx, discount)
	if err != nil {
		return nil, fmt.Errorf("failed to create inventory item: %w", err)
	}
	
	// Populate extra fields for response
	loungeProduct.CategoryName = masterItem.CategoryName

	return loungeProduct, nil
}

// UpdateProductRequest contains data for editing a lounge product
type UpdateProductRequest struct {
	SellingPrice float64 `json:"selling_price"`
	ReorderLevel int     `json:"reorder_level"`
	IsAvailable  bool    `json:"is_available"`
}

// UpdateStockRequest contains data for a stock update
type UpdateStockRequest struct {
	NewQuantity int    `json:"new_quantity"`
	Reason      string `json:"reason"`
}

// DeleteLoungeProduct soft-deletes a product from a lounge's inventory
func (s *InventoryService) DeleteLoungeProduct(loungeID uuid.UUID, productID uuid.UUID) error {
	return s.inventoryRepo.DeleteLoungeProduct(loungeID, productID)
}

// UpdateLoungeProduct updates a lounge product's editable fields
func (s *InventoryService) UpdateLoungeProduct(req *UpdateProductRequest, loungeID uuid.UUID, productID uuid.UUID) error {
	if req.SellingPrice <= 0 {
		return errors.New("selling_price must be greater than 0")
	}
	if req.ReorderLevel < 0 {
		return errors.New("reorder_level cannot be negative")
	}
	priceStr := fmt.Sprintf("%.2f", req.SellingPrice)
	return s.inventoryRepo.UpdateLoungeProductFields(loungeID, productID, priceStr, nil, req.ReorderLevel, req.IsAvailable)
}

// UpdateProductStock updates the stock quantity for a product and inserts an audit transaction
func (s *InventoryService) UpdateProductStock(req *UpdateStockRequest, loungeID uuid.UUID, productID uuid.UUID, userID uuid.UUID) error {
	if req.NewQuantity < 0 {
		return errors.New("new_quantity cannot be negative")
	}

	// Fetch current product to know current stock
	product, err := s.inventoryRepo.GetLoungeProductByID(loungeID, productID)
	if err != nil {
		return fmt.Errorf("failed to fetch product: %w", err)
	}
	if product == nil {
		return errors.New("product not found")
	}

	currentStock := 0
	if product.StockQuantity != nil {
		currentStock = *product.StockQuantity
	}

	delta := req.NewQuantity - currentStock
	txType := models.TransactionTypeStockIn
	if delta < 0 {
		txType = models.TransactionTypeStockOut
		delta = -delta
	} else if delta == 0 {
		// No change — record as adjustment
		txType = models.TransactionType("ADJUSTMENT")
		delta = req.NewQuantity
	}

	newStockStatus := "in_stock"
	if req.NewQuantity == 0 {
		newStockStatus = "out_of_stock"
	} else {
		reorderLevel := 0
		if product.ReorderLevel != nil {
			reorderLevel = *product.ReorderLevel
		}
		if req.NewQuantity <= reorderLevel {
			newStockStatus = "low_stock"
		}
	}

	reason := req.Reason
	if reason == "" {
		reason = "Manual stock update"
	}

	transaction := &models.InventoryTransaction{
		ID:              uuid.New(),
		LoungeID:        loungeID,
		LoungeProductID: productID,
		TransactionType: txType,
		Quantity:        delta,
		StockBefore:     currentStock,
		StockAfter:      req.NewQuantity,
		Reason:          &reason,
		CreatedByUserID: userID,
		CreatedAt:       time.Now().UTC(),
	}

	return s.inventoryRepo.UpdateLoungeProductStock(transaction, req.NewQuantity, newStockStatus)
}

package models

import (
	"time"

	"github.com/google/uuid"
)

// MasterInventoryItem represents an item in the admin's master inventory catalog
type MasterInventoryItem struct {
	ID                uuid.UUID  `db:"id" json:"id"`
	ItemCode          string     `db:"item_code" json:"item_code"`
	Name              string     `db:"name" json:"name"`
	Description       *string    `db:"description" json:"description,omitempty"`
	CategoryID        uuid.UUID  `db:"category_id" json:"category_id"`
	Unit              string     `db:"unit" json:"unit"`
	ImageURL          *string    `db:"image_url" json:"image_url,omitempty"`
	IsActive          bool       `db:"is_active" json:"is_active"`
	CreatedByAdminID  uuid.UUID  `db:"created_by_admin_id" json:"created_by_admin_id"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
	
	// Populated via JOIN
	CategoryName      string     `db:"category_name" json:"category_name,omitempty"`
}

// TransactionType represents the type of inventory transaction
type TransactionType string

const (
	TransactionTypeStockIn  TransactionType = "STOCK_IN"
	TransactionTypeStockOut TransactionType = "STOCK_OUT"
	TransactionTypeSale     TransactionType = "SALE"
	TransactionTypeReturn   TransactionType = "RETURN"
)

// InventoryTransaction represents a stock change for a lounge product
type InventoryTransaction struct {
	ID               uuid.UUID       `db:"id" json:"id"`
	LoungeID         uuid.UUID       `db:"lounge_id" json:"lounge_id"`
	LoungeProductID  uuid.UUID       `db:"lounge_product_id" json:"lounge_product_id"`
	TransactionType  TransactionType `db:"transaction_type" json:"transaction_type"`
	Quantity         int             `db:"quantity" json:"quantity"`
	StockBefore      int             `db:"stock_before" json:"stock_before"`
	StockAfter       int             `db:"stock_after" json:"stock_after"`
	ReferenceID      *uuid.UUID      `db:"reference_id" json:"reference_id,omitempty"`
	OrderItemID      *uuid.UUID      `db:"order_item_id" json:"order_item_id,omitempty"`
	Reason           *string         `db:"reason" json:"reason,omitempty"`
	CreatedByUserID  uuid.UUID       `db:"created_by_user_id" json:"created_by_user_id"`
	CreatedAt        time.Time       `db:"created_at" json:"created_at"`
}

// DiscountType represents the type of discount
type DiscountType string

const (
	DiscountTypePercentage  DiscountType = "PERCENTAGE"
	DiscountTypeFixedAmount DiscountType = "FIXED_AMOUNT"
)

// InventoryDiscount represents a discount applied to a lounge product
type InventoryDiscount struct {
	ID               uuid.UUID    `db:"id" json:"id"`
	LoungeProductID  uuid.UUID    `db:"lounge_product_id" json:"lounge_product_id"`
	DiscountName     string       `db:"discount_name" json:"discount_name"`
	DiscountType     DiscountType `db:"discount_type" json:"discount_type"`
	DiscountValue    string       `db:"discount_value" json:"discount_value"` // DECIMAL(10,2)
	StartAt          time.Time    `db:"start_at" json:"start_at"`
	EndAt            time.Time    `db:"end_at" json:"end_at"`
	IsActive         bool         `db:"is_active" json:"is_active"`
	CreatedByUserID  uuid.UUID    `db:"created_by_user_id" json:"created_by_user_id"`
	CreatedAt        time.Time    `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time    `db:"updated_at" json:"updated_at"`
}

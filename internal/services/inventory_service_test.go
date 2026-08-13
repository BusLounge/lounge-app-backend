package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// A simple test to verify discount validation logic in EnrollInventoryItem
func TestEnrollInventoryItem_Validation(t *testing.T) {
	s := NewInventoryService(nil) // we only test initial validation which doesn't hit DB immediately

	loungeID := uuid.New()
	userID := uuid.New()

	tests := []struct {
		name        string
		req         *EnrollInventoryItemRequest
		expectedErr string
	}{
		{
			name: "Invalid master ID",
			req: &EnrollInventoryItemRequest{
				InventoryItemID: "invalid-uuid",
			},
			expectedErr: "invalid inventory_item_id format",
		},
		{
			name: "Negative selling price",
			req: &EnrollInventoryItemRequest{
				InventoryItemID: uuid.New().String(),
				SellingPrice:    -100.0,
			},
			expectedErr: "selling_price must be greater than 0",
		},
		{
			name: "Negative initial stock",
			req: &EnrollInventoryItemRequest{
				InventoryItemID: uuid.New().String(),
				SellingPrice:    100.0,
				InitialStock:    -1,
			},
			expectedErr: "initial_stock cannot be negative",
		},
		{
			name: "Invalid discount type",
			req: &EnrollInventoryItemRequest{
				InventoryItemID: uuid.New().String(),
				SellingPrice:    100.0,
				InitialStock:    10,
				Discount: &DiscountRequest{
					DiscountName:  "Weekend",
					DiscountType:  "INVALID",
					DiscountValue: 10,
					StartAt:       time.Now(),
					EndAt:         time.Now().Add(time.Hour),
				},
			},
			expectedErr: "discount_type must be PERCENTAGE or FIXED_AMOUNT",
		},
		{
			name: "Percentage > 100",
			req: &EnrollInventoryItemRequest{
				InventoryItemID: uuid.New().String(),
				SellingPrice:    100.0,
				InitialStock:    10,
				Discount: &DiscountRequest{
					DiscountName:  "Weekend",
					DiscountType:  "PERCENTAGE",
					DiscountValue: 110,
					StartAt:       time.Now(),
					EndAt:         time.Now().Add(time.Hour),
				},
			},
			expectedErr: "percentage discount_value cannot exceed 100",
		},
		{
			name: "Invalid dates",
			req: &EnrollInventoryItemRequest{
				InventoryItemID: uuid.New().String(),
				SellingPrice:    100.0,
				InitialStock:    10,
				Discount: &DiscountRequest{
					DiscountName:  "Weekend",
					DiscountType:  "PERCENTAGE",
					DiscountValue: 10,
					StartAt:       time.Now().Add(time.Hour),
					EndAt:         time.Now(), // End before start
				},
			},
			expectedErr: "end_at must be after start_at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.EnrollInventoryItem(tt.req, loungeID, userID)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

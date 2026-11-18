package database

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// InventoryRepository handles database operations for lounge inventory
type InventoryRepository struct {
	db *sqlx.DB
}

// NewInventoryRepository creates a new inventory repository
func NewInventoryRepository(db *sqlx.DB) *InventoryRepository {
	return &InventoryRepository{db: db}
}

// GetAvailableInventoryItems retrieves all active master inventory items from the catalog.
// The duplicate-enrollment check is enforced at insert time (EnrollInventoryItem), so
// this query intentionally returns every active item regardless of whether it has
// already been added to this lounge's lounge_products table.
func (r *InventoryRepository) GetAvailableInventoryItems(loungeID uuid.UUID, search string, categoryID string) ([]models.MasterInventoryItem, error) {
	query := `
		SELECT 
			li.id, 
			li.item_code, 
			li.name, 
			li.description, 
			li.category_id, 
			li.unit, 
			li.image_url, 
			li.is_active, 
			li.created_by_admin_id, 
			li.created_at, 
			li.updated_at,
			c.name as category_name
		FROM lounge_inventory_items li
		LEFT JOIN lounge_marketplace_categories c ON li.category_id = c.id
		WHERE li.is_active = true
	`
	
	args := []interface{}{}
	paramCount := 0

	if search != "" {
		paramCount++
		query += fmt.Sprintf(" AND (li.name ILIKE $%d OR li.item_code ILIKE $%d)", paramCount, paramCount)
		args = append(args, "%"+search+"%")
	}

	if categoryID != "" {
		paramCount++
		query += fmt.Sprintf(" AND li.category_id = $%d", paramCount)
		args = append(args, categoryID)
	}

	query += " ORDER BY li.name ASC"

	var items []models.MasterInventoryItem
	err := r.db.Select(&items, query, args...)
	if err != nil {
		return nil, err
	}

	return items, nil
}

// GetInventoryMasterItemByID retrieves a specific active master inventory item
func (r *InventoryRepository) GetInventoryMasterItemByID(id uuid.UUID) (*models.MasterInventoryItem, error) {
	query := `
		SELECT 
			li.id, 
			li.item_code, 
			li.name, 
			li.description, 
			li.category_id, 
			li.unit, 
			li.image_url, 
			li.is_active, 
			li.created_by_admin_id, 
			li.created_at, 
			li.updated_at
		FROM lounge_inventory_items li
		WHERE li.id = $1 AND li.is_active = true
	`

	var item models.MasterInventoryItem
	err := r.db.Get(&item, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

// CheckLoungeInventoryItemExists checks if a master item is already enrolled by the lounge
func (r *InventoryRepository) CheckLoungeInventoryItemExists(loungeID uuid.UUID, masterID uuid.UUID) (bool, error) {
	query := `
		SELECT 1
		FROM lounge_products
		WHERE lounge_id = $1 AND inventory_item_id = $2
		LIMIT 1
	`
	
	var exists int
	err := r.db.Get(&exists, query, loungeID, masterID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	
	return true, nil
}

// CreateLoungeProductTx creates a lounge product, initial stock transaction, and optional discount in a single transaction
func (r *InventoryRepository) CreateLoungeProductTx(
	product *models.LoungeProduct,
	transaction *models.InventoryTransaction,
	discount *models.InventoryDiscount,
) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Insert Lounge Product
	insertProductQuery := `
		INSERT INTO lounge_products (
			id, lounge_id, category_id, name, description, product_type,
			price, discounted_price, image_url, thumbnail_url, inventory_item_id,
			stock_status, stock_quantity, reorder_level, is_available, is_pre_orderable,
			available_from, available_until, available_days, service_duration_minutes,
			is_vegetarian, is_vegan, is_halal, allergens, calories, display_order,
			is_featured, tags, average_rating, total_reviews, is_active,
			created_at, updated_at, price_rate_type
		) VALUES (
			:id, :lounge_id, :category_id, :name, :description, :product_type,
			:price, :discounted_price, :image_url, :thumbnail_url, :inventory_item_id,
			:stock_status, :stock_quantity, :reorder_level, :is_available, :is_pre_orderable,
			:available_from, :available_until, :available_days, :service_duration_minutes,
			:is_vegetarian, :is_vegan, :is_halal, :allergens, :calories, :display_order,
			:is_featured, :tags, :average_rating, :total_reviews, :is_active,
			:created_at, :updated_at, :price_rate_type
		)
	`
	_, err = tx.NamedExec(insertProductQuery, product)
	if err != nil {
		return fmt.Errorf("failed to insert lounge product: %w", err)
	}

	// 2. Insert Stock Transaction (if applicable)
	if transaction != nil {
		insertTxQuery := `
			INSERT INTO lounge_inventory_transactions (
				id, lounge_id, lounge_product_id, transaction_type, quantity,
				stock_before, stock_after, reason, created_by_user_id, created_at
			) VALUES (
				:id, :lounge_id, :lounge_product_id, :transaction_type, :quantity,
				:stock_before, :stock_after, :reason, :created_by_user_id, :created_at
			)
		`
		_, err = tx.NamedExec(insertTxQuery, transaction)
		if err != nil {
			return fmt.Errorf("failed to insert inventory transaction: %w", err)
		}
	}

	// 3. Insert Discount (if applicable)
	if discount != nil {
		insertDiscountQuery := `
			INSERT INTO lounge_inventory_discounts (
				id, lounge_product_id, discount_name, discount_type, discount_value,
				start_at, end_at, is_active, created_by_user_id, created_at, updated_at
			) VALUES (
				:id, :lounge_product_id, :discount_name, :discount_type, :discount_value,
				:start_at, :end_at, :is_active, :created_by_user_id, :created_at, :updated_at
			)
		`
		_, err = tx.NamedExec(insertDiscountQuery, discount)
		if err != nil {
			return fmt.Errorf("failed to insert inventory discount: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteLoungeProduct soft-deletes a lounge product by setting is_active = false
func (r *InventoryRepository) DeleteLoungeProduct(loungeID uuid.UUID, productID uuid.UUID) error {
	query := `
		UPDATE lounge_products
		SET is_active = false, is_available = false, updated_at = NOW()
		WHERE id = $1 AND lounge_id = $2 AND is_active = true
	`
	result, err := r.db.Exec(query, productID, loungeID)
	if err != nil {
		return fmt.Errorf("failed to delete lounge product: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("product not found or already deleted")
	}
	return nil
}

// UpdateLoungeProductFields updates editable fields on a lounge product
func (r *InventoryRepository) UpdateLoungeProductFields(loungeID uuid.UUID, productID uuid.UUID, price string, discountedPrice *string, reorderLevel int, isAvailable bool) error {
	query := `
		UPDATE lounge_products
		SET price = $1,
		    discounted_price = $2,
		    reorder_level = $3,
		    is_available = $4,
		    updated_at = NOW()
		WHERE id = $5 AND lounge_id = $6 AND is_active = true
	`
	result, err := r.db.Exec(query, price, discountedPrice, reorderLevel, isAvailable, productID, loungeID)
	if err != nil {
		return fmt.Errorf("failed to update lounge product: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("product not found")
	}
	return nil
}

// UpdateLoungeProductStock updates the stock quantity and inserts an audit transaction atomically
func (r *InventoryRepository) UpdateLoungeProductStock(transaction *models.InventoryTransaction, newQty int, newStockStatus string) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Update stock quantity and status
	updateQuery := `
		UPDATE lounge_products
		SET stock_quantity = $1,
		    stock_status = $2,
		    updated_at = NOW()
		WHERE id = $3 AND lounge_id = $4 AND is_active = true
	`
	result, err := tx.Exec(updateQuery, newQty, newStockStatus, transaction.LoungeProductID, transaction.LoungeID)
	if err != nil {
		return fmt.Errorf("failed to update product stock: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("product not found")
	}

	// 2. Insert audit transaction
	insertTxQuery := `
		INSERT INTO lounge_inventory_transactions (
			id, lounge_id, lounge_product_id, transaction_type, quantity,
			stock_before, stock_after, reason, created_by_user_id, created_at
		) VALUES (
			:id, :lounge_id, :lounge_product_id, :transaction_type, :quantity,
			:stock_before, :stock_after, :reason, :created_by_user_id, :created_at
		)
	`
	_, err = tx.NamedExec(insertTxQuery, transaction)
	if err != nil {
		return fmt.Errorf("failed to insert stock transaction: %w", err)
	}

	return tx.Commit()
}

// GetLoungeProductByID retrieves a single active lounge product by ID
func (r *InventoryRepository) GetLoungeProductByID(loungeID uuid.UUID, productID uuid.UUID) (*models.LoungeProduct, error) {
	query := `
		SELECT id, lounge_id, category_id, name, description, product_type,
		       price, discounted_price, image_url, thumbnail_url, inventory_item_id,
		       stock_status, stock_quantity, reorder_level, is_available, is_active,
		       created_at, updated_at, price_rate_type
		FROM lounge_products
		WHERE id = $1 AND lounge_id = $2 AND is_active = true
	`
	var product models.LoungeProduct
	err := r.db.Get(&product, query, productID, loungeID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return &product, nil
}

// UpsertProductDiscount creates or updates the discount for a lounge product
func (r *InventoryRepository) UpsertProductDiscount(discount *models.InventoryDiscount) error {
	var count int
	err := r.db.Get(&count, "SELECT COUNT(*) FROM lounge_inventory_discounts WHERE lounge_product_id = $1", discount.LoungeProductID)
	if err != nil {
		return fmt.Errorf("failed to check existing discount: %w", err)
	}

	if count > 0 {
		query := `
			UPDATE lounge_inventory_discounts
			SET discount_name = $1,
			    discount_type = $2,
			    discount_value = $3,
			    start_at = $4,
			    end_at = $5,
			    is_active = $6,
			    updated_at = NOW()
			WHERE lounge_product_id = $7
		`
		_, err = r.db.Exec(query,
			discount.DiscountName,
			discount.DiscountType,
			discount.DiscountValue,
			discount.StartAt,
			discount.EndAt,
			discount.IsActive,
			discount.LoungeProductID,
		)
		if err != nil {
			return fmt.Errorf("failed to update discount: %w", err)
		}
	} else {
		query := `
			INSERT INTO lounge_inventory_discounts (
				id, lounge_product_id, discount_name, discount_type, discount_value,
				start_at, end_at, is_active, created_by_user_id, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()
			)
		`
		_, err = r.db.Exec(query,
			discount.ID,
			discount.LoungeProductID,
			discount.DiscountName,
			discount.DiscountType,
			discount.DiscountValue,
			discount.StartAt,
			discount.EndAt,
			discount.IsActive,
			discount.CreatedByUserID,
		)
		if err != nil {
			return fmt.Errorf("failed to insert discount: %w", err)
		}
	}
	return nil
}

// DeactivateProductDiscounts marks all discounts for a product as inactive
func (r *InventoryRepository) DeactivateProductDiscounts(productID uuid.UUID) error {
	query := `
		UPDATE lounge_inventory_discounts
		SET is_active = false, updated_at = NOW()
		WHERE lounge_product_id = $1
	`
	_, err := r.db.Exec(query, productID)
	return err
}

// GetDiscountByProductID retrieves the latest discount (active or inactive) for a lounge product
func (r *InventoryRepository) GetDiscountByProductID(productID uuid.UUID) (*models.InventoryDiscount, error) {
	query := `
		SELECT id, lounge_product_id, discount_name, discount_type, discount_value,
		       start_at, end_at, is_active, created_by_user_id, created_at, updated_at
		FROM lounge_inventory_discounts
		WHERE lounge_product_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var discount models.InventoryDiscount
	err := r.db.Get(&discount, query, productID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return &discount, nil
}

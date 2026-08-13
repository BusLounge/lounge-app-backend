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

// GetAvailableInventoryItems retrieves master inventory items that are active and not yet enrolled by the specified lounge
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
		AND NOT EXISTS (
			SELECT 1
			FROM lounge_products lp
			WHERE lp.lounge_id = $1
			  AND lp.inventory_item_id = li.id
		)
	`
	
	args := []interface{}{loungeID}
	paramCount := 1

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

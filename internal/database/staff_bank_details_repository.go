package database

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// StaffBankDetailsRepository provides database access for staff_bank_details
type StaffBankDetailsRepository struct {
	db *sqlx.DB
}

// NewStaffBankDetailsRepository creates a new repository
func NewStaffBankDetailsRepository(db *sqlx.DB) *StaffBankDetailsRepository {
	return &StaffBankDetailsRepository{db: db}
}

// Create inserts a new card / bank detail into staff_bank_details
func (r *StaffBankDetailsRepository) Create(d *models.StaffBankDetail) (*models.StaffBankDetail, error) {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	now := time.Now().UTC()
	d.CreatedAt = now
	d.UpdatedAt = now

	// If is_default is true, unmark previous default entries for this user
	if d.IsDefault {
		_, _ = r.db.Exec(`UPDATE staff_bank_details SET is_default = false WHERE user_id = $1`, d.UserID)
	}

	query := `
		INSERT INTO staff_bank_details (
			id, user_id, staff_id, bank_name, bank_code, branch_name, branch_code,
			account_number, account_holder_name, account_type, is_default, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
		RETURNING id, user_id, staff_id, bank_name, bank_code, branch_name, branch_code,
		          account_number, account_holder_name, account_type, is_default, created_at, updated_at
	`

	var res models.StaffBankDetail
	err := r.db.QueryRow(
		query,
		d.ID, d.UserID, d.StaffID, d.BankName, d.BankCode, d.BranchName, d.BranchCode,
		d.AccountNumber, d.AccountHolderName, d.AccountType, d.IsDefault, d.CreatedAt, d.UpdatedAt,
	).Scan(
		&res.ID, &res.UserID, &res.StaffID, &res.BankName, &res.BankCode, &res.BranchName, &res.BranchCode,
		&res.AccountNumber, &res.AccountHolderName, &res.AccountType, &res.IsDefault, &res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert staff_bank_details: %w", err)
	}
	return &res, nil
}

// ListByUserID returns all saved cards/bank details for a user
func (r *StaffBankDetailsRepository) ListByUserID(userID uuid.UUID) ([]models.StaffBankDetail, error) {
	query := `
		SELECT id, user_id, staff_id, bank_name, bank_code, branch_name, branch_code,
		       account_number, account_holder_name, account_type, is_default, created_at, updated_at
		FROM staff_bank_details
		WHERE user_id = $1
		ORDER BY is_default DESC, created_at DESC
	`
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query staff_bank_details: %w", err)
	}
	defer rows.Close()

	var list []models.StaffBankDetail
	for rows.Next() {
		var d models.StaffBankDetail
		err := rows.Scan(
			&d.ID, &d.UserID, &d.StaffID, &d.BankName, &d.BankCode, &d.BranchName, &d.BranchCode,
			&d.AccountNumber, &d.AccountHolderName, &d.AccountType, &d.IsDefault, &d.CreatedAt, &d.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan staff_bank_details: %w", err)
		}
		list = append(list, d)
	}
	return list, nil
}

// DeleteByID deletes a card belonging to the specified user
func (r *StaffBankDetailsRepository) DeleteByID(id uuid.UUID, userID uuid.UUID) error {
	query := `DELETE FROM staff_bank_details WHERE id = $1 AND user_id = $2`
	_, err := r.db.Exec(query, id, userID)
	return err
}

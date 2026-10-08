package models

import (
	"time"

	"github.com/google/uuid"
)

// StaffBankDetail represents a row in staff_bank_details table
type StaffBankDetail struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	UserID            uuid.UUID  `json:"user_id" db:"user_id"`
	StaffID           *uuid.UUID `json:"staff_id,omitempty" db:"staff_id"`
	BankName          string     `json:"bank_name" db:"bank_name"`
	BankCode          *string    `json:"bank_code,omitempty" db:"bank_code"`
	BranchName        string     `json:"branch_name" db:"branch_name"`
	BranchCode        *string    `json:"branch_code,omitempty" db:"branch_code"`
	AccountNumber     string     `json:"account_number" db:"account_number"`
	AccountHolderName string     `json:"account_holder_name" db:"account_holder_name"`
	AccountType       *string    `json:"account_type,omitempty" db:"account_type"`
	IsDefault         bool       `json:"is_default" db:"is_default"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}

// AddCardPaymentMethodRequest represents the payload from the mobile Add Payment Method form
type AddCardPaymentMethodRequest struct {
	CardNumber      string `json:"card_number" binding:"required"`
	ExpirationDate  string `json:"expiration_date" binding:"required"`
	SecurityCode    string `json:"security_code" binding:"required"`
	FullName        string `json:"full_name" binding:"required"`
	CountryOrRegion string `json:"country_or_region" binding:"required"`
	AddressLine1    string `json:"address_line_1" binding:"required"`
	CardType        string `json:"card_type"`
	CardBrand       string `json:"card_brand"`
}

package models

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// LOUNGE OWNER WALLET
// ============================================================================

// LoungeOwnerWallet represents the wallet balance for a lounge owner.
// One wallet per lounge owner (UNIQUE on owner_id).
type LoungeOwnerWallet struct {
	ID        uuid.UUID `json:"id" db:"id"`
	OwnerID   uuid.UUID `json:"owner_id" db:"owner_id"`
	Balance   float64   `json:"balance" db:"balance"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ============================================================================
// WALLET TRANSACTIONS
// ============================================================================

// WalletTxType is the direction of the transaction.
type WalletTxType string

const (
	WalletTxCredit WalletTxType = "credit" // money added to wallet
	WalletTxDebit  WalletTxType = "debit"  // money removed from wallet
)

// WalletTxRefType categorises the source/destination of the transaction.
type WalletTxRefType string

const (
	// Credit types
	WalletTxRefTypeSettlement WalletTxRefType = "settlement" // admin pays owner after 14-day period
	// Debit types
	WalletTxRefTypeWithdrawal WalletTxRefType = "withdrawal" // owner withdraws to bank
)

// WalletTxStatus tracks the lifecycle of a transaction.
type WalletTxStatus string

const (
	WalletTxStatusPending   WalletTxStatus = "pending"   // created, not yet processed
	WalletTxStatusCompleted WalletTxStatus = "completed" // successfully processed
	WalletTxStatusFailed    WalletTxStatus = "failed"    // processing failed
)

// LoungeOwnerWalletTransaction records every credit and debit against
// a lounge owner wallet.
type LoungeOwnerWalletTransaction struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	WalletID    uuid.UUID       `json:"wallet_id" db:"wallet_id"`
	OwnerID     uuid.UUID       `json:"owner_id" db:"owner_id"`
	Type        WalletTxType    `json:"type" db:"type"`
	RefType     WalletTxRefType `json:"ref_type" db:"ref_type"`
	Amount      float64         `json:"amount" db:"amount"`
	Status      WalletTxStatus  `json:"status" db:"status"`
	Description string          `json:"description" db:"description"`
	// For settlement credits – which 14-day period this covers
	PeriodStart *time.Time `json:"period_start,omitempty" db:"period_start"`
	PeriodEnd   *time.Time `json:"period_end,omitempty" db:"period_end"`
	// For withdrawal debits – bank details snapshot ID
	BankDetailsID *uuid.UUID `json:"bank_details_id,omitempty" db:"bank_details_id"`
	// Admin who initiated a settlement credit (nullable for withdrawals)
	AdminID   *uuid.UUID `json:"admin_id,omitempty" db:"admin_id"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

// ============================================================================
// REQUEST / RESPONSE TYPES
// ============================================================================

// AdminSettlementRequest is the request body for
// POST /api/v1/admin/wallet/settle-owner
// Admin sends earnings to a lounge owner wallet after the 14-day period.
type AdminSettlementRequest struct {
	OwnerID     string  `json:"owner_id" binding:"required"`
	Amount      float64 `json:"amount" binding:"required,gt=0"`
	Description string  `json:"description"`
	// ISO date strings (YYYY-MM-DD) for the 14-day period being settled
	PeriodStart string `json:"period_start" binding:"required"`
	PeriodEnd   string `json:"period_end" binding:"required"`
}

// OwnerWithdrawalRequest is the request body for
// POST /api/v1/lounge-owner/wallet/withdraw
// Lounge owner requests a bank transfer from their wallet.
type OwnerWithdrawalRequest struct {
	Amount        float64 `json:"amount" binding:"required,gt=0"`
	BankDetailsID string  `json:"bank_details_id" binding:"required"`
	Description   string  `json:"description"`
}

// WalletStatusResponse is returned by GET /api/v1/lounge-owner/wallet/status
type WalletStatusResponse struct {
	WalletBalance float64 `json:"wallet_balance"`
	OwnerID       string  `json:"owner_id"`
}

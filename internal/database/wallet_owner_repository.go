package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// LoungeOwnerWalletRepository handles all wallet and wallet transaction DB ops.
type LoungeOwnerWalletRepository struct {
	db *sqlx.DB
}

// NewLoungeOwnerWalletRepository creates the repository.
func NewLoungeOwnerWalletRepository(db *sqlx.DB) *LoungeOwnerWalletRepository {
	return &LoungeOwnerWalletRepository{db: db}
}

// ============================================================================
// WALLET DDL HELPERS (called once at startup to auto-create tables)
// ============================================================================

// EnsureTablesExist creates the wallet tables if they do not already exist.
// Call this from main.go after the DB connection is established.
func (r *LoungeOwnerWalletRepository) EnsureTablesExist() error {
	walletTable := `
	CREATE TABLE IF NOT EXISTS lounge_owner_wallets (
		id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		owner_id   UUID NOT NULL UNIQUE REFERENCES lounge_owners(id) ON DELETE CASCADE,
		balance    NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (balance >= 0),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`

	txTable := `
	CREATE TABLE IF NOT EXISTS lounge_owner_wallet_transactions (
		id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		wallet_id       UUID NOT NULL REFERENCES lounge_owner_wallets(id) ON DELETE CASCADE,
		owner_id        UUID NOT NULL,
		type            TEXT NOT NULL CHECK (type IN ('credit','debit')),
		ref_type        TEXT NOT NULL CHECK (ref_type IN ('settlement','withdrawal')),
		amount          NUMERIC(14,2) NOT NULL CHECK (amount > 0),
		status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','completed','failed')),
		description     TEXT NOT NULL DEFAULT '',
		period_start    DATE,
		period_end      DATE,
		bank_details_id UUID,
		admin_id        UUID,
		created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`

	if _, err := r.db.Exec(walletTable); err != nil {
		return fmt.Errorf("failed to create lounge_owner_wallets table: %w", err)
	}
	if _, err := r.db.Exec(txTable); err != nil {
		return fmt.Errorf("failed to create lounge_owner_wallet_transactions table: %w", err)
	}
	return nil
}

// ============================================================================
// WALLET CRUD
// ============================================================================

// GetOrCreateWallet fetches the wallet for ownerID, creating one if absent.
func (r *LoungeOwnerWalletRepository) GetOrCreateWallet(ownerID uuid.UUID) (*models.LoungeOwnerWallet, error) {
	// Try insert (idempotent due to ON CONFLICT DO NOTHING)
	insertQ := `
		INSERT INTO lounge_owner_wallets (id, owner_id, balance, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, 0, NOW(), NOW())
		ON CONFLICT (owner_id) DO NOTHING`
	if _, err := r.db.Exec(insertQ, ownerID); err != nil {
		return nil, fmt.Errorf("wallet upsert: %w", err)
	}

	// Fetch
	var w models.LoungeOwnerWallet
	fetchQ := `SELECT id, owner_id, balance, created_at, updated_at
	           FROM lounge_owner_wallets WHERE owner_id = $1`
	if err := r.db.Get(&w, fetchQ, ownerID); err != nil {
		return nil, fmt.Errorf("wallet fetch: %w", err)
	}
	return &w, nil
}

// GetWalletByOwnerID fetches the wallet without creating one.
func (r *LoungeOwnerWalletRepository) GetWalletByOwnerID(ownerID uuid.UUID) (*models.LoungeOwnerWallet, error) {
	var w models.LoungeOwnerWallet
	q := `SELECT id, owner_id, balance, created_at, updated_at
	      FROM lounge_owner_wallets WHERE owner_id = $1`
	err := r.db.Get(&w, q, ownerID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("wallet get: %w", err)
	}
	return &w, nil
}

// ============================================================================
// CREDIT  (Admin → Owner wallet)
// ============================================================================

// CreditWallet adds amount to the owner's wallet and records a transaction.
// adminID is the UUID of the admin performing the settlement.
func (r *LoungeOwnerWalletRepository) CreditWallet(
	ownerID uuid.UUID,
	amount float64,
	description string,
	periodStart, periodEnd time.Time,
	adminID uuid.UUID,
) (*models.LoungeOwnerWalletTransaction, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Lock the wallet row and get current balance
	var wallet models.LoungeOwnerWallet
	lockQ := `SELECT id, owner_id, balance, created_at, updated_at
	          FROM lounge_owner_wallets WHERE owner_id = $1 FOR UPDATE`
	if err = tx.Get(&wallet, lockQ, ownerID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("wallet not found for owner %s", ownerID)
		}
		return nil, fmt.Errorf("lock wallet: %w", err)
	}

	// Update balance
	newBalance := wallet.Balance + amount
	updateQ := `UPDATE lounge_owner_wallets SET balance = $1, updated_at = NOW() WHERE id = $2`
	if _, err = tx.Exec(updateQ, newBalance, wallet.ID); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}

	// Insert transaction record
	txID := uuid.New()
	adminUUID := adminID
	insertQ := `
		INSERT INTO lounge_owner_wallet_transactions
		  (id, wallet_id, owner_id, type, ref_type, amount, status, description,
		   period_start, period_end, admin_id, created_at, updated_at)
		VALUES
		  ($1, $2, $3, 'credit', 'settlement', $4, 'completed', $5,
		   $6, $7, $8, NOW(), NOW())`
	if _, err = tx.Exec(insertQ,
		txID, wallet.ID, ownerID, amount, description,
		periodStart, periodEnd, adminUUID); err != nil {
		return nil, fmt.Errorf("insert credit transaction: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit credit: %w", err)
	}

	return &models.LoungeOwnerWalletTransaction{
		ID:          txID,
		WalletID:    wallet.ID,
		OwnerID:     ownerID,
		Type:        models.WalletTxCredit,
		RefType:     models.WalletTxRefTypeSettlement,
		Amount:      amount,
		Status:      models.WalletTxStatusCompleted,
		Description: description,
		PeriodStart: &periodStart,
		PeriodEnd:   &periodEnd,
		AdminID:     &adminUUID,
	}, nil
}

// ============================================================================
// DEBIT  (Owner → Bank withdrawal)
// ============================================================================

// DebitWallet deducts amount from the owner's wallet and records a transaction.
// Returns an error if the balance would go negative.
func (r *LoungeOwnerWalletRepository) DebitWallet(
	ownerID uuid.UUID,
	amount float64,
	description string,
	bankDetailsID uuid.UUID,
) (*models.LoungeOwnerWalletTransaction, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Lock wallet row
	var wallet models.LoungeOwnerWallet
	lockQ := `SELECT id, owner_id, balance, created_at, updated_at
	          FROM lounge_owner_wallets WHERE owner_id = $1 FOR UPDATE`
	if err = tx.Get(&wallet, lockQ, ownerID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("wallet not found for owner %s", ownerID)
		}
		return nil, fmt.Errorf("lock wallet: %w", err)
	}

	if wallet.Balance < amount {
		err = fmt.Errorf("insufficient balance: have %.2f, need %.2f", wallet.Balance, amount)
		return nil, err
	}

	// Update balance
	newBalance := wallet.Balance - amount
	updateQ := `UPDATE lounge_owner_wallets SET balance = $1, updated_at = NOW() WHERE id = $2`
	if _, err = tx.Exec(updateQ, newBalance, wallet.ID); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}

	// Insert transaction record
	txID := uuid.New()
	bdID := bankDetailsID
	insertQ := `
		INSERT INTO lounge_owner_wallet_transactions
		  (id, wallet_id, owner_id, type, ref_type, amount, status, description,
		   bank_details_id, created_at, updated_at)
		VALUES
		  ($1, $2, $3, 'debit', 'withdrawal', $4, 'completed', $5,
		   $6, NOW(), NOW())`
	if _, err = tx.Exec(insertQ,
		txID, wallet.ID, ownerID, amount, description, bdID); err != nil {
		return nil, fmt.Errorf("insert debit transaction: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit debit: %w", err)
	}

	return &models.LoungeOwnerWalletTransaction{
		ID:            txID,
		WalletID:      wallet.ID,
		OwnerID:       ownerID,
		Type:          models.WalletTxDebit,
		RefType:       models.WalletTxRefTypeWithdrawal,
		Amount:        amount,
		Status:        models.WalletTxStatusCompleted,
		Description:   description,
		BankDetailsID: &bdID,
	}, nil
}

// ============================================================================
// TRANSACTION HISTORY
// ============================================================================

// ListTransactions returns all transactions for a given owner, newest first.
func (r *LoungeOwnerWalletRepository) ListTransactions(ownerID uuid.UUID, limit, offset int) ([]models.LoungeOwnerWalletTransaction, error) {
	q := `
		SELECT id, wallet_id, owner_id, type, ref_type, amount, status, description,
		       period_start, period_end, bank_details_id, admin_id, created_at, updated_at
		FROM lounge_owner_wallet_transactions
		WHERE owner_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	var rows []models.LoungeOwnerWalletTransaction
	if err := r.db.Select(&rows, q, ownerID, limit, offset); err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	return rows, nil
}

// ListAllTransactionsForAdmin returns all wallet transactions for admin dashboard, newest first.
func (r *LoungeOwnerWalletRepository) ListAllTransactionsForAdmin(limit, offset int) ([]models.LoungeOwnerWalletTransaction, error) {
	q := `
		SELECT id, wallet_id, owner_id, type, ref_type, amount, status, description,
		       period_start, period_end, bank_details_id, admin_id, created_at, updated_at
		FROM lounge_owner_wallet_transactions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	var rows []models.LoungeOwnerWalletTransaction
	if err := r.db.Select(&rows, q, limit, offset); err != nil {
		return nil, fmt.Errorf("list all transactions: %w", err)
	}
	return rows, nil
}

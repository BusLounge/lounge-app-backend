package models

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// LOUNGE SPECIAL PACKAGE (lounge_special_packages table)
// ============================================================================

// LoungeSpecialPackageType represents the package tier
type LoungeSpecialPackageType string

const (
	LoungeSpecialPackageTypePlatinum LoungeSpecialPackageType = "platinum"
	LoungeSpecialPackageTypeGold     LoungeSpecialPackageType = "gold"
	LoungeSpecialPackageTypeStandard LoungeSpecialPackageType = "standard"
)

// LoungeSpecialPackage represents a special package offered by a lounge
type LoungeSpecialPackage struct {
	ID          uuid.UUID                `db:"id" json:"id"`
	LoungeID    uuid.UUID                `db:"lounge_id" json:"lounge_id"`
	PackageName string                   `db:"package_name" json:"package_name"`
	ImageURL    *string                  `db:"image_url" json:"image_url,omitempty"`
	PackageType LoungeSpecialPackageType `db:"package_type" json:"package_type"`
	Description string                   `db:"description" json:"description"`
	Price       string                   `db:"price" json:"price"` // DECIMAL(10,2) as string
	IsActive    bool                     `db:"is_active" json:"is_active"`
	CreatedAt   time.Time                `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time                `db:"updated_at" json:"updated_at"`
}

package database

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// LoungeSpecialPackageRepository handles DB operations for lounge special packages
type LoungeSpecialPackageRepository struct {
	db DB
}

// NewLoungeSpecialPackageRepository creates a new repository
func NewLoungeSpecialPackageRepository(db DB) *LoungeSpecialPackageRepository {
	return &LoungeSpecialPackageRepository{db: db}
}

// CreateSpecialPackage inserts a new special package into the database
func (r *LoungeSpecialPackageRepository) CreateSpecialPackage(pkg *models.LoungeSpecialPackage) error {
	pkg.ID = uuid.New()
	pkg.CreatedAt = time.Now()
	pkg.UpdatedAt = time.Now()
	pkg.IsActive = true

	query := `
		INSERT INTO lounge_special_packages (
			id, lounge_id, package_name, image_url, package_type,
			description, price, is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10
		)`

	_, err := r.db.Exec(
		query,
		pkg.ID,
		pkg.LoungeID,
		pkg.PackageName,
		pkg.ImageURL,
		string(pkg.PackageType),
		pkg.Description,
		pkg.Price,
		pkg.IsActive,
		pkg.CreatedAt,
		pkg.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create special package: %w", err)
	}

	log.Printf("INFO: Created special package %s for lounge %s", pkg.ID, pkg.LoungeID)
	return nil
}

// GetSpecialPackagesByLoungeID retrieves all active special packages for a lounge
func (r *LoungeSpecialPackageRepository) GetSpecialPackagesByLoungeID(loungeID uuid.UUID) ([]models.LoungeSpecialPackage, error) {
	query := `
		SELECT id, lounge_id, package_name, image_url, package_type,
		       description, price, is_active, created_at, updated_at
		FROM lounge_special_packages
		WHERE lounge_id = $1 AND is_active = TRUE
		ORDER BY
			CASE package_type
				WHEN 'platinum' THEN 1
				WHEN 'gold'     THEN 2
				WHEN 'standard' THEN 3
				ELSE 4
			END,
			created_at DESC`

	rows, err := r.db.Query(query, loungeID)
	if err != nil {
		return nil, fmt.Errorf("query special packages: %w", err)
	}
	defer rows.Close()

	var packages []models.LoungeSpecialPackage
	for rows.Next() {
		var pkg models.LoungeSpecialPackage
		if err := rows.Scan(
			&pkg.ID,
			&pkg.LoungeID,
			&pkg.PackageName,
			&pkg.ImageURL,
			&pkg.PackageType,
			&pkg.Description,
			&pkg.Price,
			&pkg.IsActive,
			&pkg.CreatedAt,
			&pkg.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan special package: %w", err)
		}
		packages = append(packages, pkg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return packages, nil
}

// GetSpecialPackageByID retrieves a single special package by ID
func (r *LoungeSpecialPackageRepository) GetSpecialPackageByID(pkgID uuid.UUID) (*models.LoungeSpecialPackage, error) {
	query := `
		SELECT id, lounge_id, package_name, image_url, package_type,
		       description, price, is_active, created_at, updated_at
		FROM lounge_special_packages
		WHERE id = $1`

	var pkg models.LoungeSpecialPackage
	err := r.db.QueryRow(query, pkgID).Scan(
		&pkg.ID,
		&pkg.LoungeID,
		&pkg.PackageName,
		&pkg.ImageURL,
		&pkg.PackageType,
		&pkg.Description,
		&pkg.Price,
		&pkg.IsActive,
		&pkg.CreatedAt,
		&pkg.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get special package by id: %w", err)
	}
	return &pkg, nil
}

// UpdateSpecialPackage updates an existing special package
func (r *LoungeSpecialPackageRepository) UpdateSpecialPackage(pkg *models.LoungeSpecialPackage) error {
	pkg.UpdatedAt = time.Now()

	query := `
		UPDATE lounge_special_packages
		SET package_name = $1,
		    image_url    = $2,
		    package_type = $3,
		    description  = $4,
		    price        = $5,
		    updated_at   = $6
		WHERE id = $7`

	result, err := r.db.Exec(
		query,
		pkg.PackageName,
		pkg.ImageURL,
		string(pkg.PackageType),
		pkg.Description,
		pkg.Price,
		pkg.UpdatedAt,
		pkg.ID,
	)
	if err != nil {
		return fmt.Errorf("update special package: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("special package not found: %s", pkg.ID)
	}
	return nil
}

// DeleteSpecialPackage soft-deletes a special package (sets is_active = false)
func (r *LoungeSpecialPackageRepository) DeleteSpecialPackage(pkgID uuid.UUID) error {
	query := `
		UPDATE lounge_special_packages
		SET is_active = FALSE, updated_at = $1
		WHERE id = $2`

	result, err := r.db.Exec(query, time.Now(), pkgID)
	if err != nil {
		return fmt.Errorf("delete special package: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("special package not found: %s", pkgID)
	}
	return nil
}

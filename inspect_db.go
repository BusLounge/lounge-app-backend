//go:build ignore
// +build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	db, err := sql.Open("postgres", "postgresql://postgres.pttatcukzpceljcrwehk:KQ95tJUYdFX251VR@aws-1-us-east-1.pooler.supabase.com:5432/postgres")
	if err != nil {
		log.Fatal(err)
	}
	query := `
		UPDATE lounge_staff
		SET
			approval_status = $1::lounge_staff_approval_status,
			employment_status = COALESCE($2::lounge_staff_employment_status, employment_status),
			hired_date = CASE
				WHEN $1::text = 'approved' THEN CURRENT_DATE
				ELSE hired_date
			END,	
			updated_at = NOW()
		WHERE id = $3
	`

	res, err := db.Exec(query, "approved", "active", "600d7ac0-946d-4b49-b666-5eb2b0cfeb0d")
	if err != nil {
		fmt.Printf("ERROR executing update: %v\n", err)
	} else {
		rows, _ := res.RowsAffected()
		fmt.Printf("SUCCESS! Rows affected: %d\n", rows)
	}

}

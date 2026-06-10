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
	defer db.Close()

	rows, err := db.Query(`
		SELECT conname, pg_get_constraintdef(con.oid), 'NO', 'NULL'
		FROM pg_constraint con
		JOIN pg_class rel ON rel.oid = con.conrelid
		WHERE rel.relname = 'lounge_special_packages'
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Constraints in lounge_special_packages:")
	for rows.Next() {
		var colName, dataType, isNullable string
		var colDefault sql.NullString
		if err := rows.Scan(&colName, &dataType, &isNullable, &colDefault); err != nil {
			log.Fatal(err)
		}
		defVal := "NULL"
		if colDefault.Valid {
			defVal = colDefault.String
		}
		fmt.Printf("- %s: %s (nullable: %s, default: %s)\n", colName, dataType, isNullable, defVal)
	}
}

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
		SELECT enumlabel, 'enum', 'NO', 'NULL'
		FROM pg_enum 
		JOIN pg_type ON pg_enum.enumtypid = pg_type.oid 
		WHERE pg_type.typname = 'transport_types'
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Values in transport_types enum:")
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

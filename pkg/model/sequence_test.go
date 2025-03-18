package model

import (
	"database/sql"
	"fmt"
	"testing"

	_ "modernc.org/sqlite"
)

func TestGetGenome(t *testing.T) {
	// Create a mock database connection
	db, err := sql.Open("sqlite", "/data/db/gene_table.db")

	if err != nil {
		panic(err)
	}

	defer db.Close()

	// Call the function
	result, err := GetGenomes(db)

	if err != nil {
		t.Error("Should not happen")
	}

	fmt.Println(result)

}

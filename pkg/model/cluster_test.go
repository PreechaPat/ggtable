package model

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/yumyai/ggtable/pkg/handler/types"
)

// Check if the query function works as intend.
func TestClusterQuery(t *testing.T) {

	db, err := sql.Open("sqlite", "../../db/gene_table_minified.db")

	if err != nil {
		panic(err)
	}

	defer db.Close()

	// Create a search request

	r, err := searchCluster(db, types.SearchRequest{
		Search_for:   "heat",
		Search_field: "",
		Page:         1,
		Page_size:    30,
	})

	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", r[0])

}

func TestPivoting(t *testing.T) {

	db, err := sql.Open("sqlite", "../../db/gene_table_minified.db")

	if err != nil {
		panic(err)
	}

	defer db.Close()

	r, err := searchCluster(db, types.SearchRequest{
		Search_for:   "heat",
		Search_field: "function",
		Page:         1,
		Page_size:    30,
	})

	if err != nil {
		panic(err)
	}

	for i := range 3 {
		fmt.Printf("%+v\n", r[i])
	}
}

func TestCountRow(t *testing.T) {

	db, err := sql.Open("sqlite", "../../db/gene_table_minified.db")

	if err != nil {
		panic(err)
	}

	defer db.Close()

	r, err := CountRowByQuery(db, types.SearchRequest{
		Search_for:   "heat",
		Search_field: "",
		Page:         1,
		Page_size:    30,
	})

	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", r)

}

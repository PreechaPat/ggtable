package model

import (
	"context"
	"database/sql"
	"fmt"
)

var (
	MAP_HEADER    map[string]string
	ALL_GENOME_ID []string
)

// Initialize the map from genome_id to genome_fullname using data from the database
func InitMapHeader(db *sql.DB) error {
	ctx := context.TODO()

	rows, err := db.QueryContext(ctx, `SELECT genome_id, genome_fullname FROM genome_info`)
	if err != nil {
		return fmt.Errorf("InitMapHeader: query failed: %w", err)
	}
	defer rows.Close()

	m := make(map[string]string)
	for rows.Next() {
		var id, fullname string
		if err := rows.Scan(&id, &fullname); err != nil {
			return fmt.Errorf("InitMapHeader: scan failed: %w", err)
		}
		m[id] = fullname
	}

	MAP_HEADER = m

	genomeIDs := make([]string, 0, len(MAP_HEADER))
	for id := range MAP_HEADER {
		genomeIDs = append(genomeIDs, id)
	}
	ALL_GENOME_ID = genomeIDs
	return nil
}

func SetGenomeID(genomeIDs []string) {

	// TODO: Check for overlap between genomeIDs and ALL_GENOME_ID
	// Report back only the valid IDs and use those to set ALL_GENOME_ID
	ALL_GENOME_ID = genomeIDs
}

package sqldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/yumyai/ggtable/pkg/model"
)

// Genomes loads every genome in genome_info, in table order. A genome without
// a full name is labelled with its ID.
func (db *DB) Genomes(ctx context.Context) (model.GenomeCatalog, error) {
	const q = `
		SELECT genome_id, COALESCE(NULLIF(genome_fullname, ''), genome_id)
		FROM genome_info
		WHERE COALESCE(genome_id, '') <> ''
		ORDER BY rowid`
	rows, err := db.sql.QueryContext(ctx, q)
	if err != nil {
		return model.GenomeCatalog{}, fmt.Errorf("load genomes: %w", err)
	}
	defer rows.Close()

	var ids []string
	names := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return model.GenomeCatalog{}, fmt.Errorf("load genomes: %w", err)
		}
		if _, dup := names[id]; dup {
			return model.GenomeCatalog{}, fmt.Errorf("load genomes: genome_id %q appears more than once in genome_info", id)
		}
		ids = append(ids, id)
		names[id] = name
	}
	if err := rows.Err(); err != nil {
		return model.GenomeCatalog{}, fmt.Errorf("load genomes: %w", err)
	}
	return model.NewGenomeCatalog(ids, names), nil
}

// ClusterIDsForGene returns the clusters a gene belongs to.
func (db *DB) ClusterIDsForGene(ctx context.Context, genomeID, geneID string) ([]string, error) {
	const q = `SELECT DISTINCT cluster_id FROM gene_matches WHERE genome_id = ? AND gene_id = ?`
	rows, err := db.sql.QueryContext(ctx, q, genomeID, geneID)
	if err != nil {
		return nil, fmt.Errorf("find clusters of gene %s/%s: %w", genomeID, geneID, err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("find clusters of gene %s/%s: %w", genomeID, geneID, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find clusters of gene %s/%s: %w", genomeID, geneID, err)
	}
	return ids, nil
}

// SampleGene returns any one matched gene, so startup can check that the
// BLAST databases use the same sequence IDs as the SQLite tables.
func (db *DB) SampleGene(ctx context.Context) (model.GeneRef, error) {
	var ref model.GeneRef
	const q = `SELECT genome_id, contig_id, gene_id FROM gene_matches WHERE gene_id <> '' LIMIT 1`
	if err := db.sql.QueryRowContext(ctx, q).Scan(&ref.GenomeID, &ref.ContigID, &ref.GeneID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ref, notFound("gene match", "any")
		}
		return ref, fmt.Errorf("sample gene_matches: %w", err)
	}
	return ref, nil
}

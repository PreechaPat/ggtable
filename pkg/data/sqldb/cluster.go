package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yumyai/ggtable/pkg/model"
)

// Every cluster read works in two steps inside one transaction:
//  1. scaffold: fill the temp table unique_clusters with the clusters of the
//     page, in display order, and temp_genome_ids with the genome filter;
//  2. hydrate: load those clusters, then their genes and regions.

// Cluster loads one cluster with all of its genes and regions.
// It returns an error wrapping model.ErrNotFound if the cluster does not exist.
func (db *DB) Cluster(ctx context.Context, clusterID string) (*model.Cluster, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var clusters []*model.Cluster
	err := db.withReadTx(ctx, func(tx *sql.Tx) error {
		if err := createGenomeFilter(ctx, tx, nil); err != nil {
			return err
		}
		const q = `
			CREATE TEMPORARY TABLE unique_clusters AS
			SELECT cluster_id, cog_id, expected_length, function_description, representative_gene
			FROM gene_clusters WHERE cluster_id = ? LIMIT 1`
		if _, err := tx.ExecContext(ctx, q, clusterID); err != nil {
			return fmt.Errorf("select cluster: %w", err)
		}
		var err error
		clusters, err = hydrate(ctx, tx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("load cluster %q: %w", clusterID, err)
	}
	if len(clusters) == 0 {
		return nil, notFound("cluster", clusterID)
	}
	return clusters[0], nil
}

// createGenomeFilter fills temp_genome_ids; an empty list means no filter.
func createGenomeFilter(ctx context.Context, tx *sql.Tx, ids []string) error {
	if _, err := tx.ExecContext(ctx, `CREATE TEMPORARY TABLE IF NOT EXISTS temp_genome_ids (genome_id TEXT)`); err != nil {
		return fmt.Errorf("create genome filter: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO temp_genome_ids (genome_id) VALUES (?)`)
	if err != nil {
		return fmt.Errorf("create genome filter: %w", err)
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.ExecContext(ctx, id); err != nil {
			return fmt.Errorf("add genome %q to filter: %w", id, err)
		}
	}
	return nil
}

// genomeFilter is the WHERE condition that applies temp_genome_ids to column col.
func genomeFilter(col string) string {
	return `(NOT EXISTS (SELECT 1 FROM temp_genome_ids) OR ` + col + ` IN (SELECT genome_id FROM temp_genome_ids))`
}

// hydrate returns the clusters in unique_clusters, in order, with the genes
// and regions that pass the genome filter. Clusters with no members are kept.
func hydrate(ctx context.Context, tx *sql.Tx) ([]*model.Cluster, error) {
	clusters, byID, err := loadScaffold(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err := hydrateGenes(ctx, tx, byID); err != nil {
		return nil, err
	}
	if err := hydrateRegions(ctx, tx, byID); err != nil {
		return nil, err
	}
	return clusters, nil
}

func loadScaffold(ctx context.Context, tx *sql.Tx) ([]*model.Cluster, map[string]*model.Cluster, error) {
	const q = `
		SELECT cluster_id,
		       COALESCE(cog_id, '') AS cog_id,
		       COALESCE(expected_length, '') AS expected_length,
		       COALESCE(function_description, '') AS function_description,
		       COALESCE(representative_gene, '') AS representative_gene
		FROM unique_clusters ORDER BY rowid`
	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return nil, nil, fmt.Errorf("read clusters: %w", err)
	}
	defer rows.Close()

	var clusters []*model.Cluster
	byID := map[string]*model.Cluster{}
	for rows.Next() {
		var p model.ClusterProperty
		if err := rows.Scan(&p.ClusterID, &p.CogID, &p.ExpectedLength, &p.FunctionDescription, &p.RepresentativeGene); err != nil {
			return nil, nil, fmt.Errorf("read gene_clusters row: %w", err)
		}
		cl := model.NewCluster(p)
		clusters = append(clusters, cl)
		byID[p.ClusterID] = cl
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("read clusters: %w", err)
	}
	return clusters, byID, nil
}

func hydrateGenes(ctx context.Context, tx *sql.Tx, byID map[string]*model.Cluster) error {
	q := `
		SELECT gm.cluster_id,
		       COALESCE(gm.genome_id, '') AS genome_id,
		       COALESCE(gm.contig_id, '') AS contig_id,
		       COALESCE(gm.gene_id, '') AS gene_id,
		       COALESCE(ROUND(100.0 * gi.gene_length / gc.expected_length, 2), 0) AS completeness,
		       COALESCE(gi.description, '') AS description,
		       COALESCE(gi.start_location, 0) AS start_location,
		       COALESCE(gi.end_location, 0) AS end_location
		FROM unique_clusters gc
		JOIN gene_matches gm ON gm.cluster_id = gc.cluster_id
		LEFT JOIN gene_info gi ON gi.genome_id = gm.genome_id AND gi.gene_id = gm.gene_id
		WHERE ` + genomeFilter("gm.genome_id") + `
		ORDER BY gc.rowid, gm.rowid`
	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return fmt.Errorf("read genes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var clusterID string
		g := &model.Gene{Region: &model.Region{}}
		if err := rows.Scan(&clusterID, &g.Region.GenomeID, &g.Region.ContigID, &g.GeneID,
			&g.Completeness, &g.Description, &g.Region.Start, &g.Region.End); err != nil {
			return fmt.Errorf("read gene_matches/gene_info row (start/end/length must be integers): %w", err)
		}
		cl := byID[clusterID]
		genome := cl.Genome(g.Region.GenomeID)
		genome.Genes = append(genome.Genes, g)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read genes: %w", err)
	}
	return nil
}

func hydrateRegions(ctx context.Context, tx *sql.Tx, byID map[string]*model.Cluster) error {
	q := `
		SELECT rm.cluster_id,
		       COALESCE(rm.genome_id, '') AS genome_id,
		       COALESCE(rm.contig_id, '') AS contig_id,
		       COALESCE(rm.start_location, 0) AS start_location,
		       COALESCE(rm.end_location, 0) AS end_location
		FROM unique_clusters gc
		JOIN region_matches rm ON rm.cluster_id = gc.cluster_id
		WHERE ` + genomeFilter("rm.genome_id") + `
		ORDER BY gc.rowid, rm.rowid`
	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return fmt.Errorf("read regions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var clusterID string
		r := &model.Region{}
		if err := rows.Scan(&clusterID, &r.GenomeID, &r.ContigID, &r.Start, &r.End); err != nil {
			return fmt.Errorf("read region_matches row (start/end must be integers): %w", err)
		}
		genome := byID[clusterID].Genome(r.GenomeID)
		genome.Regions = append(genome.Regions, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read regions: %w", err)
	}
	return nil
}

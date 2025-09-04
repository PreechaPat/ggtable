// TODO: Check if these methods hold connection far too long than it should have.

package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/handler/request"
	"go.uber.org/zap"
)

/***************
 * TX LIFECYCLE
 ***************/

// withTxRollback runs fn in a transaction and rollback
// Use only with query where temp table exists.
func withTxRollback(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		// Explicit: terminate the tx, release the connection to the pool.
		_ = tx.Rollback()
	}()
	if err := fn(tx); err != nil {
		return err // rollback happens in defer
	}
	return nil
}

// createBaseUniqueClusters builds an unpaginated temp table "base_unique_clusters"
// and also writes the total row count into a single-row temp table "result_total".
func createBaseUniqueClusters(tx *sql.Tx, baseSQL string, args ...any) error {
	// Drop if exists to allow reuse within the same tx (idempotent-ish usage).
	if _, err := tx.Exec(`DROP TABLE IF EXISTS base_unique_clusters;`); err != nil {
		return fmt.Errorf("drop base_unique_clusters: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS result_total;`); err != nil {
		return fmt.Errorf("drop result_total: %w", err)
	}

	// Materialize the full, filtered set (no ORDER, no LIMIT/OFFSET).
	ddl := `CREATE TEMPORARY TABLE base_unique_clusters AS ` + baseSQL
	if _, err := tx.Exec(ddl, args...); err != nil {
		return fmt.Errorf("create base_unique_clusters: %w", err)
	}

	// Capture total count once, cheaply reused by callers.
	if _, err := tx.Exec(`CREATE TEMPORARY TABLE result_total AS SELECT COUNT(*) AS total FROM base_unique_clusters;`); err != nil {
		return fmt.Errorf("create result_total: %w", err)
	}

	return nil
}

// pageFromBaseUniqueClusters builds the paginated/ordered view into "unique_clusters".
func pageFromBaseUniqueClusters(tx *sql.Tx, orderBy string, limit, offset int) error {
	if _, err := tx.Exec(`DROP TABLE IF EXISTS unique_clusters;`); err != nil {
		return fmt.Errorf("drop unique_clusters: %w", err)
	}

	sql := fmt.Sprintf(`
		CREATE TEMPORARY TABLE unique_clusters AS
		SELECT cluster_id, cog_id, expected_length, function_description, representative_gene
		FROM base_unique_clusters
		ORDER BY %s
		LIMIT ? OFFSET ?;`, orderBy)

	if _, err := tx.Exec(sql, limit, offset); err != nil {
		return fmt.Errorf("create unique_clusters: %w", err)
	}
	return nil
}

func cleanupTempTables(tx *sql.Tx) {
	_, _ = tx.Exec(`DROP TABLE IF EXISTS temp_genome_ids`)
	_, _ = tx.Exec(`DROP TABLE IF EXISTS matched_clusters`)
	_, _ = tx.Exec(`DROP TABLE IF EXISTS unique_clusters`)
	_, _ = tx.Exec(`DROP TABLE IF EXISTS result_total`)
	_, _ = tx.Exec(`DROP TABLE IF EXISTS basednique_clusters`)
}

/***************************
 * COMMON DATA MODEL HELPERS
 ***************************/

// ensureCluster returns the existing cluster or creates a new shell.
func ensureCluster(m map[string]*Cluster, prop ClusterProperty) *Cluster {
	cl, ok := m[prop.ClusterID]
	if !ok {
		cl = &Cluster{
			ClusterProperty: prop,
			Genomes:         map[string]*Genome{},
		}
		m[prop.ClusterID] = cl
	}
	return cl
}

func ensureGenome(cl *Cluster, genomeID string) *Genome {
	g, ok := cl.Genomes[genomeID]
	if !ok {
		g = &Genome{
			Genes:   []*Gene{},
			Regions: []*Region{},
		}
		cl.Genomes[genomeID] = g
	}
	return g
}

// TODO: Sort by other means than cluster_id
func clustersFromMapSorted(m map[string]*Cluster) []*Cluster {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]*Cluster, 0, len(keys))
	for _, k := range keys {
		out = append(out, m[k])
	}
	return out
}

/**********************
 * SCAFFOLD (TEMP TABS)
 **********************/

// buildTempGenomeIDs creates and (optionally) populates temp_genome_ids.
func buildTempGenomeIDs(tx *sql.Tx, ids []string) error {
	ddl := fmt.Sprintf(`CREATE TEMPORARY TABLE IF NOT EXISTS temp_genome_ids (genome_id TEXT);`)
	if _, err := tx.Exec(ddl); err != nil {
		return fmt.Errorf("create temp_genome_ids: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}
	stmt, err := tx.Prepare(`INSERT INTO temp_genome_ids (genome_id) VALUES (?);`)
	if err != nil {
		return fmt.Errorf("prepare insert genome_id: %w", err)
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			return fmt.Errorf("insert genome_id %q: %w", id, err)
		}
	}
	return nil
}

func geneNameScaffoldUniqueClusters(tx *sql.Tx, req request.ClusterSearchRequest) error {
	// temp_genome_ids first (TEXT as in original GeneName scaffold)
	if err := buildTempGenomeIDs(tx, req.Genome_IDs); err != nil {
		return err
	}

	like := "%" + req.Search_For + "%"

	const matchedSQL = `
		CREATE TEMPORARY TABLE matched_clusters AS
		SELECT DISTINCT gm.cluster_id
		FROM gene_info gi
		JOIN gene_matches gm
			ON gi.gene_id = gm.gene_id AND gi.genome_id = gm.genome_id
		WHERE gi.gene_id LIKE ?
		  AND (
			gm.genome_id IN (SELECT genome_id FROM temp_genome_ids)
			OR (SELECT COUNT(*) FROM temp_genome_ids)=0
		  );
	`
	if _, err := tx.Exec(matchedSQL, like); err != nil {
		return fmt.Errorf("create matched_clusters: %w", err)
	}

	orderBy, err := orderByExpr(req.Order_By)
	if err != nil {
		return err
	}

	const uniqueTpl = `
		CREATE TEMPORARY TABLE unique_clusters AS
		SELECT gc.cluster_id, gc.cog_id, gc.expected_length, gc.function_description, gc.representative_gene
		FROM gene_clusters gc
		JOIN matched_clusters mc ON mc.cluster_id = gc.cluster_id
		ORDER BY %s
		LIMIT ? OFFSET ?;
	`
	sql := fmt.Sprintf(uniqueTpl, orderBy)
	limit := req.Page_Size
	offset := (req.Page - 1) * req.Page_Size

	if _, err := tx.Exec(sql, limit, offset); err != nil {
		return fmt.Errorf("create unique_clusters: %w", err)
	}
	return nil
}

// propScaffoldUniqueClusters implements the original “scaffoldClusters” behavior.
func propScaffoldUniqueClusters(tx *sql.Tx, req request.ClusterSearchRequest) error {
	// temp_genome_ids as INTEGER per original
	if _, err := tx.Exec(`CREATE TEMPORARY TABLE temp_genome_ids (genome_id INTEGER);`); err != nil {
		return fmt.Errorf("create temp_genome_ids: %w", err)
	}

	where, err := whereFilterExpr(req.Search_Field)
	if err != nil {
		return err
	}
	orderBy, err := orderByExpr(req.Order_By)
	if err != nil {
		return err
	}
	limit := req.Page_Size
	offset := (req.Page - 1) * req.Page_Size
	like := "%" + req.Search_For + "%"

	const tpl = `
		CREATE TEMPORARY TABLE unique_clusters AS
			SELECT gc.cluster_id, gc.cog_id, gc.expected_length, gc.function_description, gc.representative_gene
			FROM gene_clusters gc
			WHERE (%s)
			ORDER BY %s
			LIMIT ? OFFSET ?;
	`
	sql := fmt.Sprintf(tpl, where, orderBy)

	if _, err := tx.Exec(sql, like, limit, offset); err != nil {
		return fmt.Errorf("create unique_clusters: %w", err)
	}

	// Populate temp_genome_ids
	if err := populateTempGenomeIDsInteger(tx, req.Genome_IDs); err != nil {
		return err
	}
	return nil
}

func populateTempGenomeIDsInteger(tx *sql.Tx, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	stmt, err := tx.Prepare(`INSERT INTO temp_genome_ids (genome_id) VALUES (?);`)
	if err != nil {
		return fmt.Errorf("prepare insert genome_id: %w", err)
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			return fmt.Errorf("insert genome_id %q: %w", id, err)
		}
	}
	return nil
}

func whereFilterExpr(field request.ClusterField) (string, error) {
	switch field {
	case request.ClusterFieldFunction:
		return "gc.function_description LIKE ?", nil
	case request.ClusterFieldCOGID:
		return "gc.cog_id LIKE ?", nil
	case request.ClusterFieldClusterID:
		return "gc.cluster_id LIKE ?", nil
	default:
		logger.Error("error in query section")
		return "", fmt.Errorf("no search_field")
	}
}

func orderByExpr(field request.ClusterField) (string, error) {
	switch field {
	case request.ClusterFieldFunction:
		return "gc.function_description", nil
	case request.ClusterFieldCOGID:
		return "gc.cog_id", nil
	case request.ClusterFieldClusterID:
		return "gc.cluster_id", nil
	default:
		logger.Error("error in order_by section")
		return "", fmt.Errorf("no order_by field")
	}
}

/*****************
 * FILL: GENES/REG
 *****************/

func geneNameHydrateGenes(tx *sql.Tx, clusterMap map[string]*Cluster) error {
	const q = `
		SELECT
			gc.cluster_id,
			gc.cog_id,
			gc.expected_length,
			gc.function_description,
			json_group_array(
				json_object(
					'gene_id', gm.gene_id,
					'completeness', ROUND(100.0 * gi.gene_length / gc.expected_length, 2),
					'description', gi.description,
					'region', json_object(
						'genome_id', gm.genome_id,
						'contig_id', gm.contig_id,
						'start', gi.start_location,
						'end', gi.end_location
					)
				)
			) AS genes
		FROM unique_clusters gc
		LEFT JOIN gene_matches gm ON gc.cluster_id = gm.cluster_id
		LEFT JOIN gene_info gi ON gm.gene_id = gi.gene_id AND gm.genome_id = gi.genome_id
		WHERE
			(
				gm.genome_id IN (SELECT genome_id FROM temp_genome_ids)
				OR (SELECT COUNT(*) FROM temp_genome_ids)=0
			)
		GROUP BY gc.cluster_id;
	`
	return scanGenes(tx, q, clusterMap)
}

func propHydrateGenes(tx *sql.Tx, clusterMap map[string]*Cluster) error {
	const q = `
		SELECT
			gc.cluster_id, gc.cog_id, gc.expected_length, gc.function_description,
			json_group_array(
				json_object(
					'gene_id', gm.gene_id,
					'completeness', ROUND(100.0 * gi.gene_length / gc.expected_length, 2),
					'description', gi.description,
					'region', json_object(
						'genome_id', gm.genome_id,
						'contig_id', gm.contig_id,
						'start', gi.start_location,
						'end', gi.end_location
					)
				)
			) AS genes
		FROM unique_clusters gc
		LEFT JOIN gene_matches gm ON gc.cluster_id = gm.cluster_id
		LEFT JOIN gene_info gi ON gm.gene_id = gi.gene_id AND gm.genome_id = gi.genome_id
		WHERE gm.genome_id IN (SELECT genome_id FROM temp_genome_ids)
		GROUP BY gc.cluster_id;
	`
	return scanGenes(tx, q, clusterMap)
}

func scanGenes(tx *sql.Tx, q string, clusterMap map[string]*Cluster) error {
	rows, err := tx.Query(q)
	if err != nil {
		logger.Error("Gene query execution failed.")
		return fmt.Errorf("gene query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var genesJSON string
		var p ClusterProperty
		if err := rows.Scan(&p.ClusterID, &p.CogID, &p.ExpectedLength, &p.FunctionDescription, &genesJSON); err != nil {
			logger.Error("Scan gene row failed")
			return fmt.Errorf("scan gene row: %w", err)
		}
		cl := ensureCluster(clusterMap, p)

		if genesJSON == "" || genesJSON == "null" {
			continue
		}

		var genes []*Gene
		if err := json.Unmarshal([]byte(genesJSON), &genes); err != nil {
			return fmt.Errorf("unmarshal genes: %w", err)
		}
		for _, g := range genes {
			if g == nil || g.Region == nil {
				continue
			}
			genomeID := g.Region.GenomeID
			ensureGenome(cl, genomeID).Genes = append(ensureGenome(cl, genomeID).Genes, g)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("gene rows err: %w", err)
	}
	return nil
}

func geneNameHydrateRegions(tx *sql.Tx, clusterMap map[string]*Cluster) error {
	const q = `
		SELECT
			gc.cluster_id,
			gc.cog_id,
			gc.expected_length,
			gc.function_description,
			json_group_array(
				json_object(
					'genome_id', rm.genome_id,
					'contig_id', rm.contig_id,
					'start', rm.start_location,
					'end', rm.end_location
				)
			) AS regions
		FROM unique_clusters gc
		LEFT JOIN region_matches rm ON gc.cluster_id = rm.cluster_id
		WHERE
			(
				rm.genome_id IN (SELECT genome_id FROM temp_genome_ids)
				OR (SELECT COUNT(*) FROM temp_genome_ids)=0
			)
		GROUP BY gc.cluster_id;
	`
	return scanRegions(tx, q, clusterMap)
}

func propHydrateRegions(tx *sql.Tx, clusterMap map[string]*Cluster) error {
	const q = `
		SELECT
			gc.cluster_id, gc.cog_id, gc.expected_length, gc.function_description,
			json_group_array(
				json_object(
					'genome_id', rm.genome_id,
					'contig_id', rm.contig_id,
					'start', rm.start_location,
					'end', rm.end_location
				)
			) AS regions
		FROM unique_clusters gc
		LEFT JOIN region_matches rm ON gc.cluster_id = rm.cluster_id
		WHERE rm.genome_id IN (SELECT genome_id FROM temp_genome_ids)
		GROUP BY gc.cluster_id;
	`
	return scanRegions(tx, q, clusterMap)
}

func scanRegions(tx *sql.Tx, q string, clusterMap map[string]*Cluster) error {
	rows, err := tx.Query(q)
	if err != nil {
		return fmt.Errorf("region query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p ClusterProperty
		var regionsJSON string
		if err := rows.Scan(&p.ClusterID, &p.CogID, &p.ExpectedLength, &p.FunctionDescription, &regionsJSON); err != nil {
			return fmt.Errorf("scan region row: %w", err)
		}
		cl := ensureCluster(clusterMap, p)

		if regionsJSON == "" || regionsJSON == "null" {
			continue
		}
		var regions []*Region
		if err := json.Unmarshal([]byte(regionsJSON), &regions); err != nil {
			return fmt.Errorf("unmarshal regions: %w", err)
		}
		for _, r := range regions {
			if r == nil {
				continue
			}
			genomeID := r.GenomeID
			ensureGenome(cl, genomeID).Regions = append(ensureGenome(cl, genomeID).Regions, r)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("region rows err: %w", err)
	}
	return nil
}

/********************
 * PUBLIC ENTRYPOINTS
 ********************/

// SearchGeneCluster selects the main strategy based on Search_Field.
func SearchGeneCluster(db *sql.DB, req request.ClusterSearchRequest) ([]*Cluster, error) {
	var clusters []*Cluster

	// Keep total timeout similar to originals; bump slightly for safety
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	err := withTxRollback(ctx, db, &sql.TxOptions{ReadOnly: true}, func(tx *sql.Tx) error {
		clusterMap := make(map[string]*Cluster)

		if req.Search_Field == request.ClusterFieldGeneID {
			// Gene-name path
			if err := geneNameScaffoldUniqueClusters(tx, req); err != nil {
				return fmt.Errorf("scaffold by gene name: %w", err)
			}

			if err := geneNameHydrateGenes(tx, clusterMap); err != nil {
				return err
			}
			if err := geneNameHydrateRegions(tx, clusterMap); err != nil {
				return err
			}
		} else {
			// Property path
			if err := propScaffoldUniqueClusters(tx, req); err != nil {
				return fmt.Errorf("scaffold by prop: %w", err)
			}

			if err := propHydrateGenes(tx, clusterMap); err != nil {
				return err
			}
			if err := propHydrateRegions(tx, clusterMap); err != nil {
				return err
			}
		}

		cl := clustersFromMapSorted(clusterMap)
		// Preserve prior behavior: return zero-length slice (not nil) when empty.
		if len(cl) == 0 {
			cl = make([]*Cluster, 0)
		}
		clusters = cl
		return nil
	})
	if err != nil {
		logger.Error("Error at query", zap.String("Error", err.Error()))
		return nil, err
	}
	return clusters, nil
}

// GetMainPage returns unfiltered clusters.
func GetMainPage(db *sql.DB, req request.ClusterSearchRequest) ([]*Cluster, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var clusters []*Cluster

	err := withTxRollback(ctx, db, &sql.TxOptions{ReadOnly: true}, func(tx *sql.Tx) error {
		limit := req.Page_Size
		offset := (req.Page - 1) * req.Page_Size

		withVT := `
			WITH vt AS (
				SELECT gc.cluster_id, gc.cog_id, gc.expected_length, gc.function_description, gc.representative_gene
				FROM gene_clusters gc
				ORDER BY gc.cluster_id
				LIMIT ? OFFSET ?
			)
		`
		geneQuery := withVT + `
			SELECT
				vt.cluster_id, vt.cog_id, vt.expected_length, vt.function_description,
				COALESCE(
					json_group_array(
						json_object(
							'gene_id', gm.gene_id,
							'completeness', ROUND(100.0 * gi.gene_length / vt.expected_length, 2),
							'description', gi.description,
							'region', json_object(
								'genome_id', gm.genome_id,
								'contig_id', gm.contig_id,
								'start', gi.start_location,
								'end', gi.end_location
							)
						)
					),
					json('[]')
				) AS genes
			FROM vt
			LEFT JOIN gene_matches gm ON vt.cluster_id = gm.cluster_id
			LEFT JOIN gene_info gi ON gm.gene_id = gi.gene_id AND gm.genome_id = gi.genome_id
			GROUP BY vt.cluster_id, vt.cog_id, vt.expected_length, vt.function_description;
		`

		clusterMap := make(map[string]*Cluster)

		rows, err := tx.Query(geneQuery, limit, offset)
		if err != nil {
			return fmt.Errorf("gene query: %w", err)
		}
		func() {
			defer rows.Close()
			for rows.Next() {
				var p ClusterProperty
				var genesJSON string
				if err := rows.Scan(&p.ClusterID, &p.CogID, &p.ExpectedLength, &p.FunctionDescription, &genesJSON); err != nil {
					err = fmt.Errorf("scan gene row: %w", err)
					return
				}
				cl := ensureCluster(clusterMap, p)

				var genes []*Gene
				if err2 := json.Unmarshal([]byte(genesJSON), &genes); err2 != nil {
					err = fmt.Errorf("unmarshal genes: %w", err2)
					return
				}
				for _, g := range genes {
					if g == nil || g.Region == nil {
						continue
					}
					genomeID := g.Region.GenomeID
					ensureGenome(cl, genomeID).Genes = append(ensureGenome(cl, genomeID).Genes, g)
				}
			}
			if err2 := rows.Err(); err == nil && err2 != nil {
				err = fmt.Errorf("gene rows err: %w", err2)
			}
		}()
		if err != nil {
			return err
		}

		regionQuery := withVT + `
			SELECT
				vt.cluster_id, vt.cog_id, vt.expected_length, vt.function_description,
				COALESCE(
					json_group_array(
						json_object(
							'genome_id', rm.genome_id,
							'contig_id', rm.contig_id,
							'start', rm.start_location,
							'end', rm.end_location
						)
					),
					json('[]')
				) AS regions
			FROM vt
			LEFT JOIN region_matches rm ON vt.cluster_id = rm.cluster_id
			GROUP BY vt.cluster_id, vt.cog_id, vt.expected_length, vt.function_description;
		`

		rrows, err := tx.Query(regionQuery, limit, offset)
		if err != nil {
			return fmt.Errorf("region query: %w", err)
		}
		func() {
			defer rrows.Close()
			for rrows.Next() {
				var p ClusterProperty
				var regionsJSON string
				if err := rrows.Scan(&p.ClusterID, &p.CogID, &p.ExpectedLength, &p.FunctionDescription, &regionsJSON); err != nil {
					err = fmt.Errorf("scan region row: %w", err)
					return
				}
				cl := ensureCluster(clusterMap, p)

				var regions []*Region
				if err2 := json.Unmarshal([]byte(regionsJSON), &regions); err2 != nil {
					err = fmt.Errorf("unmarshal regions: %w", err2)
					return
				}
				for _, r := range regions {
					if r == nil {
						continue
					}
					genomeID := r.GenomeID
					ensureGenome(cl, genomeID).Regions = append(ensureGenome(cl, genomeID).Regions, r)
				}
			}
			if err2 := rrows.Err(); err == nil && err2 != nil {
				err = fmt.Errorf("region rows err: %w", err2)
			}
		}()
		if err != nil {
			return err
		}

		clusters = clustersFromMapSorted(clusterMap)
		if len(clusters) == 0 {
			clusters = make([]*Cluster, 0)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return clusters, nil
}

/*****************
 * COUNT ENDPOINTS
 *****************/

func CountRowByQuery(db *sql.DB, req request.ClusterSearchRequest) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	where, err := whereFilterExpr(req.Search_Field)
	if err != nil {
		return 0, err
	}

	sql := `SELECT COUNT(cluster_id) FROM gene_clusters AS gc WHERE (` + where + `)`
	like := "%" + req.Search_For + "%"

	var count int
	if err := db.QueryRowContext(ctx, sql, like).Scan(&count); err != nil {
		logger.Error("CountRowByQuery error", zap.String("err", err.Error()))
		return 0, err
	}
	return count, nil
}

func CountSearchRow(db *sql.DB, req request.ClusterSearchRequest) (int, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := withTxRollback(ctx, db, &sql.TxOptions{ReadOnly: true}, func(tx *sql.Tx) error {

		if req.Search_Field == request.ClusterFieldGeneID {
			// Gene-name path

		} else {
			// Property path
			where, err := whereFilterExpr(req.Search_Field)
			if err != nil {
				return err
			}
			sql := `SELECT COUNT(cluster_id) FROM gene_clusters AS gc WHERE (` + where + `)`
			like := "%" + req.Search_For + "%"

			var count int
			if err := tx.QueryRowContext(ctx, sql, like).Scan(&count); err != nil {
				logger.Error("CountRowByQuery error", zap.String("err", err.Error()))
				return err
			}
		}
		return nil
	})

	if err != nil {
		logger.Error("Error at query", zap.String("Error", err.Error()))
		return 0, err
	}
	return 0, nil
}

func CountAllRow(db *sql.DB) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sql := `SELECT COUNT(cluster_id) FROM gene_clusters`
	var count int
	if err := db.QueryRowContext(ctx, sql).Scan(&count); err != nil {
		logger.Error("CountAllRow error", zap.String("err", err.Error()))
		return 0, err
	}
	return count, nil
}

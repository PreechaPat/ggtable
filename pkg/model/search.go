// TODO: Check if these methods hold connection far too long than it should have.

package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/handler/request"
	"go.uber.org/zap"
)

/********************
 * PUBLIC FUNCTIONS
 ********************/

// SearchGeneCluster selects the main strategy based on Search_Field.
func SearchGeneCluster(db *sql.DB, req request.ClusterSearchRequest) ([]*Cluster, error) {
	// Keep total timeout similar to originals; bump slightly for safety
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	clusterMap := make(map[string]*Cluster)
	var orderedIDs []string

	err := withTxRollback(ctx, db, &sql.TxOptions{}, func(tx *sql.Tx) error {
		return performClusterQuery(tx, req, true, clusterMap, &orderedIDs)
	})

	if err != nil {
		logger.Error("Error at query", zap.String("Error", err.Error()))
		return nil, err
	}

	clusters := make([]*Cluster, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		if cl, ok := clusterMap[id]; ok {
			clusters = append(clusters, cl)
		}
	}

	return clusters, nil
}

// GetMainPage returns unfiltered clusters.
func GetMainPage(db *sql.DB, req request.ClusterSearchRequest) ([]*Cluster, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clusterMap := make(map[string]*Cluster)
	var orderedIDs []string

	err := withTxRollback(ctx, db, &sql.TxOptions{ReadOnly: true}, func(tx *sql.Tx) error {
		return performClusterQuery(tx, req, false, clusterMap, &orderedIDs)
	})

	if err != nil {
		logger.Error("Error at query", zap.String("Error", err.Error()))
		return nil, err
	}

	clusters := make([]*Cluster, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		if cl, ok := clusterMap[id]; ok {
			clusters = append(clusters, cl)
		}
	}

	return clusters, nil
}

func CountSearchRow(db *sql.DB, req request.ClusterSearchRequest) (int, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int

	err := withTxRollback(ctx, db, &sql.TxOptions{ReadOnly: true}, func(tx *sql.Tx) error {

		if req.Search_Field == request.ClusterFieldGeneID {
			// Gene-name path
			if err := buildTempGenomeIDs(tx, req.Genome_IDs); err != nil {
				return err
			}

			const q = `
				SELECT COUNT(DISTINCT gm.cluster_id)
				FROM gene_matches gm
				WHERE gm.gene_id = ?
				AND (
					NOT EXISTS (SELECT 1 FROM temp_genome_ids)
					OR gm.genome_id IN (SELECT genome_id FROM temp_genome_ids)
				);
			`
			if err := tx.QueryRow(q, req.Search_For).Scan(&count); err != nil {
				return fmt.Errorf("count gene-name unique clusters: %w", err)
			}

		} else {
			// Property path
			where, err := whereFilterExpr(req.Search_Field)
			if err != nil {
				return err
			}
			sql := `SELECT COUNT(cluster_id) FROM gene_clusters AS gc WHERE (` + where + `)`
			like := "%" + req.Search_For + "%"

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
	return count, nil
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

/*************************
 * QUERY ORCHESTRATION
 *************************/

// performClusterQuery orchestrates the search/retrieval process within a transaction.
// It builds a temporary table of unique clusters and then hydrates them with gene and region data.
func performClusterQuery(tx *sql.Tx, req request.ClusterSearchRequest, isSearch bool, clusterMap map[string]*Cluster, orderedIDs *[]string) error {
	if err := scaffoldUniqueClusters(tx, req, isSearch); err != nil {
		return fmt.Errorf("scaffolding unique clusters: %w", err)
	}

	if err := hydrateGenes(tx, clusterMap); err != nil {
		return err
	}
	if err := hydrateRegions(tx, clusterMap); err != nil {
		return err
	}

	var err error
	*orderedIDs, err = getOrderedClusterIDs(tx)
	if err != nil {
		return err
	}

	return nil
}

// scaffoldUniqueClusters creates and populates the unique_clusters temporary table
// based on whether it's a search or a main page view.
func scaffoldUniqueClusters(tx *sql.Tx, req request.ClusterSearchRequest, isSearch bool) error {
	if !isSearch {
		return mainPageScaffoldUniqueClusters(tx, req)
	}

	if req.Search_Field == request.ClusterFieldGeneID {
		return geneNameScaffoldUniqueClusters(tx, req)
	}
	return propScaffoldUniqueClusters(tx, req)
}

/**************************************
 * SCAFFOLDING (TEMP TABLE GENERATION)
 **************************************/

// mainPageScaffoldUniqueClusters creates unique_clusters for the main page view (no filtering).
func mainPageScaffoldUniqueClusters(tx *sql.Tx, req request.ClusterSearchRequest) error {
	if err := buildTempGenomeIDs(tx, req.Genome_IDs); err != nil {
		return err
	}

	const uniqueTpl = `
		CREATE TEMPORARY TABLE unique_clusters AS
		SELECT gc.cluster_id, gc.cog_id, gc.expected_length, gc.function_description, gc.representative_gene
		FROM gene_clusters gc
		ORDER BY gc.cluster_id
		LIMIT ? OFFSET ?;
	`
	limit := req.Page_Size
	offset := (req.Page - 1) * req.Page_Size

	if _, err := tx.Exec(uniqueTpl, limit, offset); err != nil {
		return fmt.Errorf("create unique_clusters for main page: %w", err)
	}
	return nil
}

// geneNameScaffoldUniqueClusters creates unique_clusters based on a gene ID search.
func geneNameScaffoldUniqueClusters(tx *sql.Tx, req request.ClusterSearchRequest) error {
	// temp_genome_ids first (TEXT as in original GeneName scaffold)
	if err := buildTempGenomeIDs(tx, req.Genome_IDs); err != nil {
		return err
	}

	geneID := req.Search_For

	const matchedSQL = `
		CREATE TEMPORARY TABLE matched_clusters AS
		SELECT DISTINCT gm.cluster_id
		FROM gene_matches gm
		WHERE gm.gene_id = ?
		AND (
			NOT EXISTS (SELECT 1 FROM temp_genome_ids)
			OR gm.genome_id IN (SELECT genome_id FROM temp_genome_ids)
		);
	`
	if _, err := tx.Exec(matchedSQL, geneID); err != nil {
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

// propScaffoldUniqueClusters creates unique_clusters based on a property search (e.g., function, COG ID).
func propScaffoldUniqueClusters(tx *sql.Tx, req request.ClusterSearchRequest) error {

	if err := buildTempGenomeIDs(tx, req.Genome_IDs); err != nil {
		return err
	}

	// building sql query
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

	return nil
}

// buildTempGenomeIDs creates and (optionally) populates temp_genome_ids.
func buildTempGenomeIDs(tx *sql.Tx, ids []string) error {
	ddl := `CREATE TEMPORARY TABLE IF NOT EXISTS temp_genome_ids (genome_id TEXT);`
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

// whereFilterExpr returns the SQL WHERE clause for property searches.
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

// orderByExpr returns the SQL ORDER BY clause.
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

/********************************
 * HYDRATION (POPULATING RESULTS)
 ********************************/

// hydrateGenes populates the Genes field of clusters in the map.
func hydrateGenes(tx *sql.Tx, clusterMap map[string]*Cluster) error {
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

// scanGenes scans gene query results into the cluster map.
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

// hydrateRegions populates the Regions field of clusters in the map.
func hydrateRegions(tx *sql.Tx, clusterMap map[string]*Cluster) error {
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

// scanRegions scans region query results into the cluster map.
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

/*************************
 * DATA MODEL HELPERS
 *************************/

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

// ensureGenome returns the existing genome for a cluster or creates a new one.
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

// getOrderedClusterIDs retrieves cluster IDs in the correct order from the temp table.
func getOrderedClusterIDs(tx *sql.Tx) ([]string, error) {
	const q = `SELECT cluster_id FROM unique_clusters ORDER BY rowid;` // rowid preserves insertion order
	rows, err := tx.Query(q)
	if err != nil {
		return nil, fmt.Errorf("query ordered cluster IDs: %w", err)
	}
	defer rows.Close()

	var orderedIDs []string
	for rows.Next() {
		var clusterID string
		if err := rows.Scan(&clusterID); err != nil {
			return nil, fmt.Errorf("scan ordered cluster ID: %w", err)
		}
		orderedIDs = append(orderedIDs, clusterID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ordered cluster IDs rows err: %w", err)
	}
	return orderedIDs, nil
}

/*************************
 * TRANSACTION HELPER
 *************************/

// withTxRollback runs fn in a transaction and guarantees a rollback.
// This is used for queries that rely on temporary tables, ensuring they are
// cleaned up automatically when the transaction ends.
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

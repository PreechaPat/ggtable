package model

// TODO: It seems that these functions "HOLD" a db connection and doesn't disconnect them properly
// This can be see if I limit database connection to 5-10 at the time.
import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/handler/request"
	"go.uber.org/zap"
)

func scaffoldClustersByGeneName(tx *sql.Tx, searchRequest request.ClusterSearchRequest) error {
	// 1) temp table: match base schema type exactly
	// If gm.genome_id is INTEGER, use INTEGER here. Change TEXT->INTEGER if needed.
	if _, err := tx.Exec(`CREATE TEMPORARY TABLE IF NOT EXISTS temp_genome_ids (genome_id TEXT)`); err != nil {
		return fmt.Errorf("create temp_genome_ids: %w", err)
	}

	// 2) populate temp_genome_ids first (so the filter actually works)
	for _, id := range searchRequest.Genome_IDs {
		if _, err := tx.Exec(`INSERT INTO temp_genome_ids (genome_id) VALUES (?)`, id); err != nil {
			return fmt.Errorf("populate temp_genome_ids: %w", err)
		}
	}

	// 3) build matched_clusters with a single statement + parameters
	like := "%" + searchRequest.Search_For + "%"

	// If gi.gene_id is not TEXT, switch to a text column, e.g., gi.gene_name LIKE ?
	// or use: WHERE CAST(gi.gene_id AS TEXT) LIKE ?
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

	// 4) create unique_clusters with ORDER/LIMIT/OFFSET in one param’d statement
	var orderBy string
	switch searchRequest.Order_By {
	case request.ClusterFieldFunction:
		orderBy = "gc.function_description"
	case request.ClusterFieldCOGID:
		orderBy = "gc.cog_id"
	case request.ClusterFieldClusterID:
		orderBy = "gc.cluster_id"
	default:
		return fmt.Errorf("no order_by field")
	}

	const uniqueTpl = `
        CREATE TEMPORARY TABLE unique_clusters AS
        SELECT gc.cluster_id, gc.cog_id, gc.expected_length, gc.function_description, gc.representative_gene
        FROM gene_clusters gc
        JOIN matched_clusters mc ON mc.cluster_id = gc.cluster_id
        ORDER BY %s
        LIMIT ? OFFSET ?;
    `
	uniqueSQL := fmt.Sprintf(uniqueTpl, orderBy)

	limit := searchRequest.Page_Size
	offset := (searchRequest.Page - 1) * searchRequest.Page_Size

	if _, err := tx.Exec(uniqueSQL, limit, offset); err != nil {
		return fmt.Errorf("create unique_clusters: %w", err)
	}
	return nil
}

// New: main search function that uses the gene-name scaffold.
func searchClusterByGeneName(db *sql.DB, searchRequest request.ClusterSearchRequest) ([]*Cluster, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, connErr := db.Conn(ctx)
	if connErr != nil {
		return nil, fmt.Errorf("fail to get a connection %w", connErr)
	}
	defer conn.Close()

	tx, txErr := conn.BeginTx(ctx, nil)
	if txErr != nil {
		return nil, fmt.Errorf("fail to begin tx %w", txErr)
	}
	defer tx.Rollback()

	if err := scaffoldClustersByGeneName(tx, searchRequest); err != nil {
		return nil, fmt.Errorf("failed to scaffold by gene name: %w", err)
	}

	// ==== Genes per cluster (same shape as your existing code) ====
	const geneQuery = `
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
		LEFT JOIN gene_info gi
		ON gm.gene_id = gi.gene_id AND gm.genome_id = gi.genome_id
		WHERE
		(
			gm.genome_id IN (SELECT genome_id FROM temp_genome_ids)
			OR (SELECT COUNT(*) FROM temp_genome_ids)=0
		)
		GROUP BY gc.cluster_id;
		`

	geneRows, err := tx.Query(geneQuery)
	if err != nil {
		logger.Error("Gene query execution failed.")
		return nil, fmt.Errorf("gene query execution failed: %w", err)
	}
	defer geneRows.Close()

	clusterMap := make(map[string]*Cluster)
	for geneRows.Next() {
		var genesJSON string
		var cp ClusterProperty
		if err := geneRows.Scan(&cp.ClusterID, &cp.CogID, &cp.ExpectedLength, &cp.FunctionDescription, &genesJSON); err != nil {
			logger.Error("Scan gene row failed")
			return nil, fmt.Errorf("failed to scan gene row: %w", err)
		}

		// Ensure entry
		if _, ok := clusterMap[cp.ClusterID]; !ok {
			clusterMap[cp.ClusterID] = &Cluster{
				ClusterProperty: cp,
				Genomes:         map[string]*Genome{},
			}
		}

		var genes []*Gene
		if genesJSON != "" && genesJSON != "null" {
			if err := json.Unmarshal([]byte(genesJSON), &genes); err != nil {
				return nil, fmt.Errorf("failed to unmarshal genes: %w", err)
			}
			for _, g := range genes {
				genomeID := g.Region.GenomeID
				if _, exists := clusterMap[cp.ClusterID].Genomes[genomeID]; !exists {
					clusterMap[cp.ClusterID].Genomes[genomeID] = &Genome{
						Genes:   []*Gene{},
						Regions: []*Region{},
					}
				}
				clusterMap[cp.ClusterID].Genomes[genomeID].Genes = append(clusterMap[cp.ClusterID].Genomes[genomeID].Genes, g)
			}
		}
	}

	// ==== Regions per cluster (same shape) ====
	const regionQuery = `
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

	regionRows, err := tx.Query(regionQuery)
	if err != nil {
		return nil, fmt.Errorf("region query execution failed: %w", err)
	}
	defer regionRows.Close()

	for regionRows.Next() {
		var cp ClusterProperty
		var regionsJSON string
		if err := regionRows.Scan(&cp.ClusterID, &cp.CogID, &cp.ExpectedLength, &cp.FunctionDescription, &regionsJSON); err != nil {
			return nil, fmt.Errorf("failed to scan region row: %w", err)
		}

		if _, ok := clusterMap[cp.ClusterID]; !ok {
			clusterMap[cp.ClusterID] = &Cluster{
				ClusterProperty: cp,
				Genomes:         map[string]*Genome{},
			}
		}
		var regions []*Region
		if regionsJSON != "" && regionsJSON != "null" {
			if err := json.Unmarshal([]byte(regionsJSON), &regions); err != nil {
				return nil, fmt.Errorf("failed to unmarshal regions: %w", err)
			}
			cl := clusterMap[cp.ClusterID]
			for _, r := range regions {
				genomeID := r.GenomeID
				if _, exists := cl.Genomes[genomeID]; !exists {
					cl.Genomes[genomeID] = &Genome{
						Genes:   []*Gene{},
						Regions: []*Region{},
					}
				}
				cl.Genomes[genomeID].Regions = append(cl.Genomes[genomeID].Regions, r)
			}
		}
	}

	keys := make([]string, 0, len(clusterMap))
	for k := range clusterMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	results := make([]*Cluster, 0, len(keys))
	for _, k := range keys {
		results = append(results, clusterMap[k])
	}
	return results, nil
}

func scaffoldClusters(tx *sql.Tx, searchRequest request.ClusterSearchRequest) error {
	// Create temporary tables for filtering, temp_genome_ids and unique_clusters
	PREP := `
		CREATE TEMPORARY TABLE temp_genome_ids (genome_id INTEGER);
		CREATE TEMPORARY TABLE unique_clusters AS
			SELECT gc.cluster_id, gc.cog_id, gc.expected_length, gc.function_description, gc.representative_gene
			FROM gene_clusters gc
			-- Add filtering condition for cluster_id
			WHERE
			(
			    {{WHERE_CLUSTER_FILTER}}
            )
			{{ORDER_CLUSTER_BY}}
			LIMIT ? OFFSET ?
	`
	// Filter cluster
	var clusterFilterDescription string

	switch searchRequest.Search_Field {

	case request.ClusterFieldFunction:
		clusterFilterDescription = "gc.function_description LIKE ?"
	case request.ClusterFieldCOGID:
		clusterFilterDescription = "gc.cog_id LIKE ?"
	case request.ClusterFieldClusterID:
		clusterFilterDescription = "gc.cluster_id LIKE ?"
	default:
		logger.Error("error in query section")
		return fmt.Errorf("no search_field")
	}

	var clusterOrderDescription string
	switch searchRequest.Order_By {
	case request.ClusterFieldFunction:
		clusterOrderDescription += " ORDER BY gc.function_description"
	case request.ClusterFieldCOGID:
		clusterOrderDescription += " ORDER BY gc.cog_id"
	case request.ClusterFieldClusterID:
		clusterOrderDescription += " ORDER BY gc.cluster_id"
	default:
		logger.Error("error in order_by section")
		return fmt.Errorf("no order_by field")
	}

	PREPR := strings.ReplaceAll(PREP, "{{WHERE_CLUSTER_FILTER}}", clusterFilterDescription)
	PREPQ := strings.ReplaceAll(PREPR, "{{ORDER_CLUSTER_BY}}", clusterOrderDescription)

	// Prepare query arguments
	var PREPARGS []interface{}

	// Add search term if present
	PREPARGS = append(PREPARGS, "%"+searchRequest.Search_For+"%")

	limit := searchRequest.Page_Size
	offset := (searchRequest.Page - 1) * searchRequest.Page_Size
	PREPARGS = append(PREPARGS, limit, offset)

	_, err := tx.Exec(PREPQ, PREPARGS...)
	if err != nil {
		return fmt.Errorf("failed to create temporary tables: %w", err)
	}

	// Populate the temporary genome IDs table
	for _, id := range searchRequest.Genome_IDs {
		_, err := tx.Exec(`INSERT INTO temp_genome_ids (genome_id) VALUES (?)`, id)
		if err != nil {
			return fmt.Errorf("failed to populate temp_genome_ids: %w", err)
		}
	}

	return nil
}

// Main search function for cluster search
func searchClusterByProp(db *sql.DB, searchRequest request.ClusterSearchRequest) ([]*Cluster, error) {

	// Time out
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, conn_err := db.Conn(ctx)

	if conn_err != nil {
		return nil, fmt.Errorf("fail to get a connection %w", conn_err)
	}

	defer conn.Close()

	tx, tx_err := conn.BeginTx(ctx, nil)

	if tx_err != nil {
		return nil, fmt.Errorf("fail to begin tx %w", tx_err)
	}

	defer tx.Rollback()

	err := scaffoldClusters(tx, searchRequest)

	if err != nil {
		return nil, fmt.Errorf("failed to create temporary tables: %w", err)
	}

	// Execute gene query
	geneQuery := `
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

	geneRows, err := tx.Query(geneQuery)
	if err != nil {
		logger.Error("Gene query executation failed.")
		return nil, fmt.Errorf("gene query execution failed: %w", err)
	}
	defer geneRows.Close()

	// Process gene results
	clusterMap := make(map[string]*Cluster)
	for geneRows.Next() {

		var genesJSON string
		var cluster_prop ClusterProperty
		var clusterID string

		if err := geneRows.Scan(&cluster_prop.ClusterID, &cluster_prop.CogID, &cluster_prop.ExpectedLength, &cluster_prop.FunctionDescription, &genesJSON); err != nil {
			logger.Error("Scan gene row failed")
			return nil, fmt.Errorf("failed to scan gene row: %w", err)
		}

		clusterID = cluster_prop.ClusterID

		clusterMap[clusterID] = &Cluster{
			ClusterProperty: cluster_prop,
			Genomes:         map[string]*Genome{}, // Initialize genome map
		}

		// This shouldn't be a problem even if the result is nil.
		var genes []*Gene
		if err := json.Unmarshal([]byte(genesJSON), &genes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genes: %w", err)
		}

		for _, gene := range genes {
			genomeID := gene.Region.GenomeID
			if _, exists := clusterMap[clusterID].Genomes[genomeID]; !exists {
				clusterMap[clusterID].Genomes[genomeID] = &Genome{
					Genes:   []*Gene{},
					Regions: []*Region{},
				}
			}
			clusterMap[clusterID].Genomes[genomeID].Genes = append(clusterMap[clusterID].Genomes[genomeID].Genes, gene)
		}
	}

	regionQuery := `
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
	// Execute the region query
	regionRows, err := tx.Query(regionQuery)
	if err != nil {
		return nil, fmt.Errorf("region query execution failed: %w", err)
	}
	defer regionRows.Close()

	// Process region results
	for regionRows.Next() {
		var clusterID string
		var regionsJSON string

		var cluster_prop ClusterProperty

		if err := regionRows.Scan(&cluster_prop.ClusterID, &cluster_prop.CogID, &cluster_prop.ExpectedLength, &cluster_prop.FunctionDescription, &regionsJSON); err != nil {
			return nil, fmt.Errorf("failed to scan region row: %w", err)
		}

		clusterID = cluster_prop.ClusterID

		var regions []*Region
		if err := json.Unmarshal([]byte(regionsJSON), &regions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal regions: %w", err)
		}

		if _, exists := clusterMap[clusterID]; !exists {
			clusterMap[clusterID] = &Cluster{
				ClusterProperty: cluster_prop,
				Genomes:         map[string]*Genome{}, // Initialize genome map
			}
		}

		if cluster, exists := clusterMap[clusterID]; exists {
			for _, region := range regions {
				genomeID := region.GenomeID
				if _, exists := cluster.Genomes[genomeID]; !exists {
					cluster.Genomes[genomeID] = &Genome{
						Genes:   []*Gene{},
						Regions: []*Region{},
					}
				}
				cluster.Genomes[genomeID].Regions = append(cluster.Genomes[genomeID].Regions, region)
			}
		} else {
			logger.Error("Clustering is missing", zap.String("ClusterID", clusterID))
		}
	}

	var results []*Cluster

	keys := make([]string, 0, len(clusterMap))
	for key := range clusterMap {
		keys = append(keys, key)
	}

	sort.Strings(keys) // Sort keys

	for _, key := range keys {
		results = append(results, clusterMap[key])
	}

	return results, nil
}

// Getting all available clusters (no filters), grouped like searchClusterByProp
func getAllClusters(db *sql.DB, searchParams request.ClusterSearchRequest) ([]*Cluster, error) {

	// Time out
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, conn_err := db.Conn(ctx)

	if conn_err != nil {
		return nil, fmt.Errorf("fail to get a connection %w", conn_err)
	}

	defer conn.Close()

	tx, tx_err := conn.BeginTx(ctx, nil)

	if tx_err != nil {
		return nil, fmt.Errorf("fail to begin tx %w", tx_err)
	}

	defer tx.Rollback()

	// Check if all arguments present, error otherwise
	limit := searchParams.Page_Size
	offset := (searchParams.Page - 1) * searchParams.Page_Size

	// It is quite fast WITHOUT temporary tables
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

	// Execute gene aggregation
	geneRows, err := tx.Query(geneQuery, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("gene query execution failed: %w", err)
	}
	defer geneRows.Close()

	clusterMap := make(map[string]*Cluster)
	for geneRows.Next() {
		var genesJSON string
		var prop ClusterProperty

		if err := geneRows.Scan(&prop.ClusterID, &prop.CogID, &prop.ExpectedLength, &prop.FunctionDescription, &genesJSON); err != nil {
			return nil, fmt.Errorf("failed to scan gene row: %w", err)
		}

		cid := prop.ClusterID
		if _, ok := clusterMap[cid]; !ok {
			clusterMap[cid] = &Cluster{
				ClusterProperty: prop,
				Genomes:         map[string]*Genome{},
			}
		}

		// genesJSON is guaranteed to be a JSON array (possibly empty) due to COALESCE(json('[]'))
		var genes []*Gene
		if err := json.Unmarshal([]byte(genesJSON), &genes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genes: %w", err)
		}

		for _, g := range genes {
			if g == nil || g.Region == nil {
				continue
			}
			genomeID := g.Region.GenomeID
			if _, exists := clusterMap[cid].Genomes[genomeID]; !exists {
				clusterMap[cid].Genomes[genomeID] = &Genome{
					Genes:   []*Gene{},
					Regions: []*Region{},
				}
			}
			clusterMap[cid].Genomes[genomeID].Genes = append(clusterMap[cid].Genomes[genomeID].Genes, g)
		}
	}
	if err := geneRows.Err(); err != nil {
		return nil, fmt.Errorf("gene rows error: %w", err)
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
	// Execute region aggregation
	regionRows, err := tx.Query(regionQuery, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("region query execution failed: %w", err)
	}
	defer regionRows.Close()

	for regionRows.Next() {
		var regionsJSON string
		var prop ClusterProperty

		if err := regionRows.Scan(&prop.ClusterID, &prop.CogID, &prop.ExpectedLength, &prop.FunctionDescription, &regionsJSON); err != nil {
			return nil, fmt.Errorf("failed to scan region row: %w", err)
		}

		cid := prop.ClusterID
		if _, ok := clusterMap[cid]; !ok {
			clusterMap[cid] = &Cluster{
				ClusterProperty: prop,
				Genomes:         map[string]*Genome{},
			}
		}

		var regions []*Region
		if err := json.Unmarshal([]byte(regionsJSON), &regions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal regions: %w", err)
		}

		for _, r := range regions {
			if r == nil {
				continue
			}
			genomeID := r.GenomeID
			if _, exists := clusterMap[cid].Genomes[genomeID]; !exists {
				clusterMap[cid].Genomes[genomeID] = &Genome{
					Genes:   []*Gene{},
					Regions: []*Region{},
				}
			}
			clusterMap[cid].Genomes[genomeID].Regions = append(clusterMap[cid].Genomes[genomeID].Regions, r)
		}
	}
	if err := regionRows.Err(); err != nil {
		return nil, fmt.Errorf("region rows error: %w", err)
	}

	keys := make([]string, 0, len(clusterMap))
	for k := range clusterMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	results := make([]*Cluster, 0, len(keys))
	for _, k := range keys {
		results = append(results, clusterMap[k])
	}

	return results, nil
}

// TODO: This is hard
// func searchClusterByGeneID(db *sql.DB, geneID string) ([]*Cluster, error) {
// }

// Use this to separate from the search function
func GetMainPage(db *sql.DB, search_request request.ClusterSearchRequest) ([]*Cluster, error) {

	res, query_err := getAllClusters(db, search_request)

	if query_err != nil {
		return nil, query_err
	}

	// If nothing return, make a zero array so encoder doesn't crash.
	if len(res) == 0 {
		ret := make([]*Cluster, 0)
		return ret, nil
	}

	return res, nil

}

// Search for cluster base on request
func SearchGeneCluster(db *sql.DB, search_request request.ClusterSearchRequest) ([]*Cluster, error) {

	var gene_result []*Cluster
	var query_err error
	searchField := search_request.Search_Field
	if searchField == request.ClusterFieldGeneName {
		gene_result, query_err = searchClusterByGeneName(db, search_request)
	} else {
		gene_result, query_err = searchClusterByProp(db, search_request)
	}

	if query_err != nil {
		logger.Error("Error at in query", zap.String("Error:", query_err.Error()))
		return nil, query_err
	}

	// If nothing return, make a zero array so encoder doesn't crash.
	if len(gene_result) == 0 {
		ret := make([]*Cluster, 0)
		return ret, nil
	}

	return gene_result, nil
}

// Count how many row this query will return. Use for calc the number of return page.
func CountRowByQuery(db *sql.DB, searchRequest request.ClusterSearchRequest) (rownum int, err error) {

	searchFor := searchRequest.Search_For

	ctx := context.TODO()
	PREP := `select COUNT(cluster_id) from gene_clusters as gc
	  WHERE 
	  (
	    {{WHERE_CLUSTER_FILTER}}
	  )`

	clusterFilterDescription := ""

	switch searchRequest.Search_Field {
	case request.ClusterFieldFunction:
		clusterFilterDescription = "gc.function_description LIKE ?"
	case request.ClusterFieldCOGID:
		clusterFilterDescription = "gc.cog_id LIKE ?"
	case request.ClusterFieldClusterID:
		clusterFilterDescription = "gc.cluster_id LIKE ?"
	default:
		logger.Error("error in query section")
		return 0, fmt.Errorf("no search_field")
	}

	PREPQ := strings.ReplaceAll(PREP, "{{WHERE_CLUSTER_FILTER}}", clusterFilterDescription)

	stm, err := db.PrepareContext(ctx, PREPQ)

	if err != nil {
		logger.Error(err.Error())
		return -1, err
	}

	defer stm.Close()

	// Prepare query arguments
	var PREPARGS []interface{}

	// Add search term if present
	PREPARGS = append(PREPARGS, "%"+searchFor+"%")

	row, _ := stm.QueryContext(ctx, PREPARGS...)

	row.Next()
	var count int
	err = row.Scan(&count)

	if err != nil {
		panic(err)
	}

	return count, nil
}

func CountAllRow(db *sql.DB) (rownum int, err error) {

	ctx := context.TODO()
	qstring := `select COUNT(cluster_id) from gene_clusters`

	stm, err := db.PrepareContext(ctx, qstring)

	if err != nil {
		logger.Error(err.Error())
		return -1, err
	}

	defer stm.Close()

	row, _ := stm.QueryContext(ctx)

	row.Next()
	var count int
	err = row.Scan(&count)

	if err != nil {
		panic(err)
	}

	return count, nil
}

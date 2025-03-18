package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/handler/types"
	"go.uber.org/zap"
)

// Group gene cluster from query into the same id
func groupGeneClusterByID(clusterQueries []*ClusterQuery) map[string]*Cluster {
	clustersMap := make(map[string]*Cluster)

	for _, clusterQuery := range clusterQueries {
		clusterID := clusterQuery.ClusterProperty.ClusterID
		if clusterQuery == nil || clusterID == "" || clusterQuery.genome_id == "" {
			continue // Skip invalid or incomplete ClusterQuery
		}

		if _, exists := clustersMap[clusterID]; !exists {
			clustersMap[clusterID] = &Cluster{
				ClusterProperty: clusterQuery.ClusterProperty,
				Genomes:         make(map[string]*Genome),
			}
		}

		if _, exists := clustersMap[clusterID].Genomes[clusterQuery.genome_id]; !exists {
			clustersMap[clusterID].Genomes[clusterQuery.genome_id] = &Genome{
				Genes:   make([]*Gene, 0),
				Regions: make([]*Region, 0),
			}
		}

		currentGenome := clustersMap[clusterID].Genomes[clusterQuery.genome_id]
		if currentGenome == nil {
			return nil // Handle nil error
		}

		region := &Region{
			GenomeID: clusterQuery.genome_id,
			ContigID: clusterQuery.contig_id,
			Start:    clusterQuery.start_location,
			End:      clusterQuery.end_location,
		}

		if clusterQuery.match == "gene" {
			g := Gene{
				GeneID:       clusterQuery.gene_id,
				Completeness: clusterQuery.completeness,
				Region:       region,
				Description:  clusterQuery.gene_description,
			}
			currentGenome.Genes = append(currentGenome.Genes, &g)
		} else if clusterQuery.match == "region" {
			currentGenome.Regions = append(currentGenome.Regions, region)
		}
	}

	return clustersMap
}

// Convert ClusterQuery into array of cluster struct
// Use in old queries
func arrangeClusterData(clusterQueries []*ClusterQuery) []*Cluster {

	clusterMap := groupGeneClusterByID(clusterQueries)

	// Convert map to slice by cluster_id order
	var keys []string
	for cluster_id := range clusterMap {
		keys = append(keys, cluster_id)
	}
	// Sort the cluster_id
	sort.Strings(keys)

	// Iterate over the sorted cluster_id and append the corresponding values to clusters
	var clusters []*Cluster
	for _, key := range keys {
		clusters = append(clusters, clusterMap[key])
	}

	return clusters

}

// ADD THESE TO FILTER AND HAVE A PROPER PAGINATION
// INNER JOIN gene_matches gm ON gc.cluster_id = gm.cluster_id
// -- Join with region_matches if additional filtering is needed
// INNER JOIN region_matches rm ON gc.cluster_id = rm.cluster_id

// (
//
//		gm.genome_id IN (SELECT genome_id FROM temp_genome_ids)
//	  OR
//		rm.genome_id IN (SELECT genome_id FROM temp_genome_ids)
//
// )
// AND
func searchCluster(db *sql.DB, searchRequest types.SearchRequest) ([]*Cluster, error) {
	ctx := context.TODO()

	// Create temporary tables
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
			ORDER BY gc.cluster_id
			LIMIT ? OFFSET ?
	`
	// Filter cluster
	clusterFilterDescription := ""

	switch search_field := searchRequest.Search_field; search_field {

	case "function":
		clusterFilterDescription = "gc.function_description LIKE ?"
	case "cog":
		clusterFilterDescription = "gc.cog_id LIKE ?"
	case "cluster_id":
		clusterFilterDescription = "gc.cluster_id LIKE ?"
	default:
		logger.Error("error in query section")
		return nil, fmt.Errorf("no search_field")
	}

	PREPQ := strings.ReplaceAll(PREP, "{{WHERE_CLUSTER_FILTER}}", clusterFilterDescription)

	// Prepare query arguments
	var PREPARGS []interface{}

	// Add search term if present
	PREPARGS = append(PREPARGS, "%"+searchRequest.Search_for+"%")

	limit := searchRequest.Page_size
	offset := (searchRequest.Page - 1) * searchRequest.Page_size
	PREPARGS = append(PREPARGS, limit, offset)
	_, err := db.ExecContext(ctx, PREPQ, PREPARGS...)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary tables: %w", err)
	}
	defer db.ExecContext(ctx, `
		DROP TABLE IF EXISTS temp_genome_ids;
		DROP TABLE IF EXISTS unique_clusters;
	`) // Cleanup

	// Populate the temporary genome IDs table
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	for _, id := range searchRequest.GenomeIDs {
		_, err := tx.ExecContext(ctx, `INSERT INTO temp_genome_ids (genome_id) VALUES (?)`, id)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to populate temp_genome_ids: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Queries
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

	// Execute the gene query
	geneRows, err := db.QueryContext(ctx, geneQuery)
	if err != nil {
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
			return nil, fmt.Errorf("failed to scan gene row: %w", err)
		}

		clusterID = cluster_prop.ClusterID

		clusterMap[clusterID] = &Cluster{
			ClusterProperty: cluster_prop,
			Genomes:         map[string]*Genome{}, // Initialize genome map
		}

		// This shouldn't be a problem even if the rsesult is nil.
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

	// Execute the region query
	regionRows, err := db.QueryContext(ctx, regionQuery)
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

			logger.Debug("Create more clusterMap")
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

	// Convert map to result slice to order
	var results []*Cluster

	// Sort by.
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

// Getting all available cluster
func getAllClusters(db *sql.DB, searchParams types.SearchRequest) ([]*ClusterQuery, error) {

	ctx := context.TODO()

	qstring := `
		WITH vt AS (
		SELECT gc.cluster_id, gc.cog_id, gc.representative_gene, gc.expected_length, gc.function_description
		FROM gene_clusters gc
		ORDER BY gc.cluster_id
		LIMIT ? OFFSET ?
	),
	matches AS (
		SELECT gm.cluster_id, 'gene' AS match, gm.genome_id, gm.contig_id, gm.gene_id,
		       100.0 * gi.gene_length / vt.expected_length AS completeness,
		       gi.start_location, gi.end_location, '' AS gene_description
		FROM vt
		LEFT JOIN gene_matches gm ON vt.cluster_id = gm.cluster_id
		LEFT JOIN gene_info gi ON gm.gene_id = gi.gene_id AND gm.genome_id = gi.genome_id
		UNION ALL
		SELECT rm.cluster_id, 'region' AS match, rm.genome_id, rm.contig_id, '' AS gene_id,
		       -1.0 AS completeness, rm.start_location, rm.end_location, '' AS gene_description
		FROM vt
		LEFT JOIN region_matches rm ON vt.cluster_id = rm.cluster_id
	)
	SELECT vt.cluster_id, vt.cog_id, vt.expected_length, vt.function_description, vt.representative_gene,
	       match, matches.genome_id, matches.contig_id, matches.gene_id,
	       matches.completeness, matches.start_location, matches.end_location, matches.gene_description
	FROM vt
	LEFT JOIN matches ON vt.cluster_id = matches.cluster_id;
	`
	page_size := searchParams.Page_size
	page := searchParams.Page

	stm, err := db.PrepareContext(ctx, qstring)
	if err != nil {
		return nil, err
	}
	defer stm.Close()

	// Search term, limit, offset
	// Use 1 as a first
	limit := page_size
	offset := (page - 1) * page_size
	rows, err := stm.QueryContext(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*ClusterQuery

	for rows.Next() {

		var cluster ClusterQuery
		err := rows.Scan(
			&cluster.ClusterProperty.ClusterID, &cluster.ClusterProperty.CogID, &cluster.ClusterProperty.ExpectedLength,
			&cluster.ClusterProperty.FunctionDescription, &cluster.ClusterProperty.RepresentativeGene,
			&cluster.match, &cluster.genome_id, &cluster.contig_id, &cluster.gene_id,
			&cluster.completeness, &cluster.start_location, &cluster.end_location, &cluster.gene_description,
		)
		if err != nil {
			return nil, err
		}

		results = append(results, &cluster)

	}

	return results, nil
}

// Use this to separate from the search function
func GetMainPage(db *sql.DB, search_request types.SearchRequest) ([]*Cluster, error) {

	gene_result, query_err := getAllClusters(db, search_request)

	if query_err != nil {
		return nil, query_err
	}

	// If nothing return, make a zero array so encoder doesn't crash.
	if len(gene_result) == 0 {
		ret := make([]*Cluster, 0)
		return ret, nil
	}

	res := arrangeClusterData(gene_result)

	return res, nil

}

// Search for gene cluster based on request
func SearchGeneCluster(db *sql.DB, search_request types.SearchRequest) ([]*Cluster, error) {

	gene_result, query_err := searchCluster(db, search_request)

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
func CountRowByQuery(db *sql.DB, searchRequest types.SearchRequest) (rownum int, err error) {

	searchFor := searchRequest.Search_for

	ctx := context.TODO()
	PREP := `select COUNT(cluster_id) from gene_clusters as gc
	  WHERE 
	  (
	    {{WHERE_CLUSTER_FILTER}}
	  )`

	clusterFilterDescription := ""

	switch search_field := searchRequest.Search_field; search_field {

	case "function":
		clusterFilterDescription = "gc.function_description LIKE ?"
	case "cog":
		clusterFilterDescription = "gc.cog_id LIKE ?"
	case "cluster_id":
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

// For search pages
func GetAllClusters(db *sql.DB, search_request types.SearchRequest) ([]*Cluster, error) {

	gene_result, query_err := getAllClusters(db, search_request)

	if query_err != nil {
		return nil, query_err
	}

	// If nothing return, make a zero array so encoder can do it correctly.
	if len(gene_result) == 0 {
		ret := make([]*Cluster, 0)
		return ret, nil
	}

	res := arrangeClusterData(gene_result)
	return res, nil
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

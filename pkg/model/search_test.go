package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	_ "modernc.org/sqlite" // Replace with your database driver

	"github.com/yumyai/ggtable/pkg/handler/types"
)

func searchClusterTesting(db *sql.DB, searchRequest types.SearchRequest) ([]*Cluster, error) {
	ctx := context.TODO()

	// Create temporary tables

	PREP := `
		CREATE TEMPORARY TABLE temp_genome_ids (genome_id INTEGER);
		CREATE TEMPORARY TABLE unique_clusters AS
			SELECT gc.cluster_id, gc.cog_id, gc.expected_length, gc.function_description, gc.representative_gene
			FROM gene_clusters gc
			{{WHERE_CLUSTER_FILTER}}
			ORDER BY cluster_id
	`

	whereFunctionDescription := ""
	if searchRequest.Search_for != "" {
		whereFunctionDescription = "WHERE gc.function_description LIKE ?"
	}
	PREPQ := strings.ReplaceAll(PREP, "{{WHERE_CLUSTER_FILTER}}", whereFunctionDescription)

	// Prepare query arguments
	var PREPARGS []interface{}

	// Add search term if present
	if searchRequest.Search_for != "" {
		PREPARGS = append(PREPARGS, "%"+searchRequest.Search_for+"%")
	}
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
			gc.cluster_id, gc.cog_id, gc.expected_length, gc.function_description, gc.representative_gene,
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
		FROM gene_clusters gc
		LEFT JOIN gene_matches gm ON gc.cluster_id = gm.cluster_id
		LEFT JOIN gene_info gi ON gm.gene_id = gi.gene_id AND gm.genome_id = gi.genome_id
		WHERE gc.cluster_id IN (SELECT cluster_id FROM unique_clusters)
		  AND gm.genome_id IN (SELECT genome_id FROM temp_genome_ids)
		GROUP BY gc.cluster_id;
	`

	regionQuery := `
		SELECT 
			gc.cluster_id, gc.cog_id, gc.expected_length, gc.function_description, gc.representative_gene,
			json_group_array(
				json_object(
					'genome_id', rm.genome_id,
					'contig_id', rm.contig_id,
					'start', rm.start_location,
					'end', rm.end_location
				)
			) AS regions
		FROM gene_clusters gc
		LEFT JOIN region_matches rm ON gc.cluster_id = rm.cluster_id
		WHERE gc.cluster_id IN (SELECT cluster_id FROM unique_clusters)
		  AND rm.genome_id IN (SELECT genome_id FROM temp_genome_ids)
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
		var c_property ClusterProperty
		var clusterID string
		var genesJSON string

		if err := geneRows.Scan(&c_property.ClusterID, &c_property.CogID, &c_property.ExpectedLength,
			&c_property.FunctionDescription, &c_property.RepresentativeGene, &genesJSON); err != nil {
			return nil, fmt.Errorf("failed to scan gene row: %w", err)
		}

		clusterID = c_property.ClusterID

		var genes []*Gene
		if err := json.Unmarshal([]byte(genesJSON), &genes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genes: %w", err)
		}

		clusterMap[clusterID] = &Cluster{
			ClusterProperty: c_property,
			Genomes:         map[string]*Genome{}, // Initialize genome map
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
		var c_property ClusterProperty
		var clusterID string
		var regionsJSON string

		if err := regionRows.Scan(&c_property.ClusterID, &c_property.CogID, &c_property.ExpectedLength,
			&c_property.FunctionDescription, &c_property.RepresentativeGene, &regionsJSON); err != nil {
			return nil, fmt.Errorf("failed to scan region row: %w", err)
		}

		clusterID = c_property.ClusterID

		var regions []*Region
		if err := json.Unmarshal([]byte(regionsJSON), &regions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal regions: %w", err)
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
		}
	}

	// Convert map to result slice
	// Sort by? cluster ID?
	var results []*Cluster
	for _, cluster := range clusterMap {
		results = append(results, cluster)
	}

	return results, nil
}

func TestQuery(t *testing.T) {
	// Replace with your database connection details
	db, err := sql.Open("sqlite", "/data/db/gene_table.db")
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// GENOME_ID_SMALL := []string{"CBS57885", "CBS57785", "CAO", "EQ10", "CBS57385m"}
	GENOME_ID_ALL := ALL_GENOME_ID

	// GENOME_ID_VALIDATION

	searchRequest := types.SearchRequest{
		Search_for: "heat",
		Page:       1,
		Page_size:  50,
		GenomeIDs:  GENOME_ID_ALL,
	}

	output, err := searchClusterTesting(db, searchRequest)
	if err != nil {
		t.Fatalf("Error running query: %v", err)
	}

	if len(output) != 0 {
		for _, v := range output {
			t.Log(v)
		}
	}
}

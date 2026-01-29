package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// --- helpers ---------------------------------------------------------------

func fetchClusterGenesMap(ctx context.Context, db *sql.DB, clusterID string) (map[string][]*Gene, error) {
	const q = `
		SELECT
			COALESCE(
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
				),
				json('[]')
			)
		FROM gene_clusters gc
		LEFT JOIN gene_matches gm ON gc.cluster_id = gm.cluster_id
		LEFT JOIN gene_info gi ON gm.gene_id = gi.gene_id AND gm.genome_id = gi.genome_id
		WHERE gc.cluster_id = ?;
	`

	var genesJSON string
	if err := db.QueryRowContext(ctx, q, clusterID).Scan(&genesJSON); err != nil {
		return nil, fmt.Errorf("fetchClusterGenesMap scan: %w", err)
	}

	var genes []*Gene
	if err := json.Unmarshal([]byte(genesJSON), &genes); err != nil {
		return nil, fmt.Errorf("fetchClusterGenesMap unmarshal: %w", err)
	}

	out := make(map[string][]*Gene)
	for _, g := range genes {
		genomeID := g.Region.GenomeID
		out[genomeID] = append(out[genomeID], g)
	}
	return out, nil
}

func fetchClusterRegionsMap(ctx context.Context, db *sql.DB, clusterID string) (map[string][]*Region, error) {
	const q = `
		SELECT
			COALESCE(
				json_group_array(
					json_object(
						'genome_id', rm.genome_id,
						'contig_id', rm.contig_id,
						'start', rm.start_location,
						'end', rm.end_location
					)
				) FILTER (WHERE rm.genome_id IS NOT NULL),
				json('[]')
			)
		FROM gene_clusters gc
		LEFT JOIN region_matches rm ON gc.cluster_id = rm.cluster_id
		WHERE gc.cluster_id = ?;
	`

	var regionsJSON string
	if err := db.QueryRowContext(ctx, q, clusterID).Scan(&regionsJSON); err != nil {
		return nil, fmt.Errorf("fetchClusterRegionsMap scan: %w", err)
	}

	var regions []*Region
	if err := json.Unmarshal([]byte(regionsJSON), &regions); err != nil {
		return nil, fmt.Errorf("fetchClusterRegionsMap unmarshal: %w", err)
	}

	out := make(map[string][]*Region)
	for _, r := range regions {
		// TODO: Deal with empty return of region later. Some cluster has NO region.
		out[r.GenomeID] = append(out[r.GenomeID], r)
	}
	return out, nil
}

// --- main -----------------------------------------------------------------

func getCluster(db *sql.DB, clusterID string) (*Cluster, error) {
	ctx := context.TODO()

	// fetch core cluster property here (no separate helper)
	const propQ = `
		SELECT cluster_id, cog_id, expected_length, function_description
		FROM gene_clusters
		WHERE cluster_id = ?
		LIMIT 1;
	`
	var prop ClusterProperty
	if err := db.QueryRowContext(ctx, propQ, clusterID).
		Scan(&prop.ClusterID, &prop.CogID, &prop.ExpectedLength, &prop.FunctionDescription); err != nil {
		return nil, err // returns sql.ErrNoRows if not found
	}

	genesMap, err := fetchClusterGenesMap(ctx, db, clusterID)
	if err != nil {
		return nil, err
	}
	regionsMap, err := fetchClusterRegionsMap(ctx, db, clusterID)
	if err != nil {
		return nil, err
	}

	cluster := &Cluster{
		ClusterProperty: prop,
		Genomes:         map[string]*Genome{},
	}

	// assemble from maps
	for genomeID, genes := range genesMap {
		if _, ok := cluster.Genomes[genomeID]; !ok {
			cluster.Genomes[genomeID] = &Genome{Genes: []*Gene{}, Regions: []*Region{}}
		}
		cluster.Genomes[genomeID].Genes = append(cluster.Genomes[genomeID].Genes, genes...)
	}
	for genomeID, regions := range regionsMap {
		if _, ok := cluster.Genomes[genomeID]; !ok {
			cluster.Genomes[genomeID] = &Genome{Genes: []*Gene{}, Regions: []*Region{}}
		}
		cluster.Genomes[genomeID].Regions = append(cluster.Genomes[genomeID].Regions, regions...)
	}

	return cluster, nil
}

func GetCluster(db *sql.DB, clusterID string) (*Cluster, error) {
	return getCluster(db, clusterID)
}

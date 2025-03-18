package model

import (
	"context"
	"database/sql"
	"fmt"
)

// pivotGeneData processes sorted results into a slice of ClusterRow
func arrangeSingleClusterData(clusterQueries []*ClusterQuery) *Cluster {

	clustersMap := groupGeneClusterByID(clusterQueries)

	if len(clustersMap) == 1 {
		for _, v := range clustersMap {
			return v
		}
	}

	return &Cluster{} // Return empty result instead of nil
}

// Get cluster information. Similar to search, but it is for getting one cluster.
func getCluster(db *sql.DB, cluster_id string) ([]*ClusterQuery, error) {

	ctx := context.TODO()

	qstring := `
		WITH vt AS (
			SELECT gc.cluster_id, gc.cog_id, gc.representative_gene, gc.expected_length, gc.function_description
			FROM gene_clusters gc
			WHERE gc.cluster_id == ?
			ORDER BY gc.cluster_id
		),
		matches AS (
			SELECT gm.cluster_id, "gene" as match, gm.genome_id, gm.contig_id,
			  gm.gene_id, 100.0 * gi.gene_length / vt.expected_length AS completeness, gi.start_location, gi.end_location, 
			  gi.description AS gene_description
			FROM vt
			LEFT JOIN gene_matches gm ON vt.cluster_id = gm.cluster_id
			LEFT JOIN gene_info gi ON gm.gene_id = gi.gene_id AND gm.genome_id = gi.genome_id
		UNION ALL
			SELECT rm.cluster_id, "region" as match, rm.genome_id, rm.contig_id, 
			  "" AS gene_id, -1 AS completeness, rm.start_location, rm.end_location, "" AS gene_description
			FROM vt
			LEFT JOIN region_matches rm ON vt.cluster_id = rm.cluster_id
		)
		SELECT vt.cluster_id, vt.cog_id, vt.expected_length, vt.function_description, matches.match, matches.genome_id, matches.contig_id, matches.gene_id,
			matches.completeness, matches.start_location, matches.end_location, matches.gene_description
			FROM vt
			LEFT JOIN matches ON vt.cluster_id = matches.cluster_id;
	`

	stm, err := db.PrepareContext(ctx, qstring)
	if err != nil {
		return nil, err
	}

	defer stm.Close()

	rows, err := stm.QueryContext(ctx, cluster_id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var clusterQueries = make([]*ClusterQuery, 0, 15)

	// It should match one and only one.
	for rows.Next() {

		var r ClusterQuery

		if err := rows.Scan(
			&r.ClusterProperty.ClusterID, &r.ClusterProperty.CogID,
			&r.ClusterProperty.ExpectedLength, &r.ClusterProperty.FunctionDescription,
			&r.match, &r.genome_id, &r.contig_id, &r.gene_id,
			&r.completeness, &r.start_location, &r.end_location, &r.gene_description); err != nil {
			panic(fmt.Sprintf("Error scanning row: %v\nGenome ID: %s, Contig ID: %s, Gene ID: %s",
				err, r.genome_id, r.contig_id, r.gene_id))
		}

		clusterQueries = append(clusterQueries, &r)
	}

	return clusterQueries, nil
}

// Public functions
func GetCluster(db *sql.DB, cluster_id string) (*Cluster, error) {

	query_res, queryErr := getCluster(db, cluster_id)

	if queryErr != nil {
		panic(queryErr)
	}

	result := arrangeSingleClusterData(query_res)

	if result == nil {
		panic("No cluster data found")
	}

	return result, nil
}

package model

import (
	"context"
	"fmt"
)

// ClusterFASTA assembles genes and, for nucleotide downloads, located regions.
// It returns no partial output on failure. Labels are a presentation concern.
func ClusterFASTA(ctx context.Context, clusters ClusterStore, seqs SequenceStore, id string, protein bool) ([]byte, error) {
	cluster, err := clusters.Cluster(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load cluster: %w", err)
	}
	genes, err := seqs.Genes(ctx, cluster.GeneRefs(), protein)
	if err != nil {
		return nil, fmt.Errorf("fetch gene sequences: %w", err)
	}
	if protein {
		return genes, nil
	}
	regions, err := seqs.Regions(ctx, cluster.RegionRefs())
	if err != nil {
		return nil, fmt.Errorf("fetch region sequences: %w", err)
	}
	return append(genes, regions...), nil
}

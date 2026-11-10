package model

// ClusterSummary contains domain statistics independent of HTML presentation.
type ClusterSummary struct {
	TotalGenes         int
	TotalRegions       int
	RepresentativeGene *Gene
}

// Summary chooses the longest located gene, breaking ties by genome ID then input order.
func (c *Cluster) Summary() ClusterSummary {
	var result ClusterSummary
	longest := 0
	for _, id := range c.sortedGenomeIDs() {
		genome := c.Genomes[id]
		result.TotalGenes += len(genome.Genes)
		result.TotalRegions += len(genome.Regions)
		for _, gene := range genome.Genes {
			if gene.Region == nil {
				continue
			}
			length := gene.Region.Length()
			if result.RepresentativeGene == nil || length > longest {
				result.RepresentativeGene = gene
				longest = length
			}
		}
	}
	return result
}

// Length returns the inclusive length for either strand.
func (r Region) Length() int {
	if r.End < r.Start {
		return r.Start - r.End + 1
	}
	return r.End - r.Start + 1
}

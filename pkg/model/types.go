// Package model holds the domain types and pure rules of the gene table.
// It defines storage contracts and domain operations; concrete SQLite and BLAST
// implementations live in pkg/data.
package model

import (
	"errors"
	"sort"
)

// ErrNotFound is returned (wrapped) by stores when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// ErrInvalidSearch identifies unsupported search or sort requests.
var ErrInvalidSearch = errors.New("invalid search")

type Region struct {
	GenomeID string `json:"genome_id"`
	ContigID string `json:"contig_id"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
}

type Gene struct {
	GeneID       string  `json:"gene_id"`
	Completeness float64 `json:"completeness"`
	Region       *Region `json:"region"`
	Description  string  `json:"description"`
}

// Genome holds one cluster's members in one genome.
type Genome struct {
	Genes   []*Gene   `json:"genes"`
	Regions []*Region `json:"regions"`
}

type ClusterProperty struct {
	ClusterID           string `json:"cluster_id"`
	CogID               string `json:"cog_id"`
	RepresentativeGene  string `json:"rep_gene"`
	ExpectedLength      string `json:"expected_length"`
	FunctionDescription string `json:"function_description"`
}

type Cluster struct {
	ClusterProperty ClusterProperty    `json:"cluster_properties"`
	Genomes         map[string]*Genome `json:"genomes"`
}

// NewCluster returns a cluster with no members.
func NewCluster(prop ClusterProperty) *Cluster {
	return &Cluster{ClusterProperty: prop, Genomes: map[string]*Genome{}}
}

// Genome returns the cluster's entry for genomeID, creating it if needed.
func (c *Cluster) Genome(genomeID string) *Genome {
	g, ok := c.Genomes[genomeID]
	if !ok {
		g = &Genome{Genes: []*Gene{}, Regions: []*Region{}}
		c.Genomes[genomeID] = g
	}
	return g
}

// GeneRefs lists the sequence IDs of every gene in the cluster, ordered by genome ID.
func (c *Cluster) GeneRefs() []GeneRef {
	var refs []GeneRef
	for _, id := range c.sortedGenomeIDs() {
		for _, g := range c.Genomes[id].Genes {
			refs = append(refs, GeneRef{GenomeID: g.Region.GenomeID, ContigID: g.Region.ContigID, GeneID: g.GeneID})
		}
	}
	return refs
}

// RegionRefs lists every region in the cluster that has a location, ordered by genome ID.
// Regions without a location (start and end 0, allowed by the legacy format) are skipped.
func (c *Cluster) RegionRefs() []RegionRef {
	var refs []RegionRef
	for _, id := range c.sortedGenomeIDs() {
		for _, r := range c.Genomes[id].Regions {
			if r.Start == 0 && r.End == 0 {
				continue
			}
			refs = append(refs, RegionRef{GenomeID: r.GenomeID, ContigID: r.ContigID, Start: uint64(r.Start), End: uint64(r.End)})
		}
	}
	return refs
}

func (c *Cluster) sortedGenomeIDs() []string {
	ids := make([]string, 0, len(c.Genomes))
	for id := range c.Genomes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

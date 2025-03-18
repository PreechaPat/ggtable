package model

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

type Genome struct {
	Genes   []*Gene   `json:"genes"`
	Regions []*Region `json:"regions"`
}

// Sub struct, embed
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

// Return from query
type ClusterQuery struct {
	ClusterProperty  ClusterProperty
	match            string
	genome_id        string
	contig_id        string
	gene_id          string
	completeness     float64
	start_location   int
	end_location     int
	gene_description string
}

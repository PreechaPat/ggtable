package types

import "fmt"

// Request and response.. Maybe relegate this into other later?
type BlastSearchRequest struct {
	BlastType string `json:"blast_type"`
	Sequence  string `json:"sequence"`
}

type SearchField string

const (
	SearchFieldFunction  SearchField = "function"
	SearchFieldCOG       SearchField = "cog"
	SearchFieldClusterID SearchField = "cluster_id"
)

type SearchRequest struct {
	Search_for   string
	Search_field string
	Page         int
	Page_size    int
	GenomeIDs    []string
}

type GeneRequest struct {
	Genome_ID string `json:"genome_id"`
	Contig_ID string `json:"contig_id"`
	Gene_ID   string `json:"gene_id"`
	Is_Prot   bool   `json:"is_prot"`
}

type RegionRequest struct {
	Genome_ID string `json:"genome_id"`
	Contig_ID string `json:"contig_id"`
	Start     uint64 `json:"start"`
	End       uint64 `json:"end"`
	Is_Prot   bool   `json:"is_prot"`
}

// type MultipleGenesRequest struct {
// 	Request string        `json:"request"`
// 	Payload []GeneRequest `json:"payload"`
// }

// type MultipleRegionsRequest struct {
// 	Request string          `json:"request"`
// 	Payload []RegionRequest `json:"payload"`
// }

// Method for samtools request
func (g GeneRequest) String() string {
	return fmt.Sprintf(
		"%s|%s|%s",
		g.Genome_ID, g.Contig_ID, g.Gene_ID,
	)
}

// String method for RegionRequest
func (r RegionRequest) String() string {
	return fmt.Sprintf(
		"%s|%s:%d-%d",
		r.Genome_ID, r.Contig_ID, r.Start, r.End,
	)
}

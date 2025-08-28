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

// Structure for querying
type ClusterSearchRequest struct {
	Search_for              string   `json:"search_for"`                 // Term or keyword to search
	Search_field            string   `json:"search_field"`               // Field to search within (e.g., gene_symbol, description)
	Page                    int      `json:"page"`                       // Page number for pagination (starting at 1)
	Page_size               int      `json:"page_size"`                  // Number of results per page
	GenomeIDs               []string `json:"genome_ids"`                 // Genome IDs to limit the search
	RequireGenesFromGenomes []string `json:"require_genes_from_genomes"` // Filter: only include clusters with these genes from the specified genomes
}

// Get gene sequence
type GeneGetRequest struct {
	Genome_ID string `json:"genome_id"`
	Contig_ID string `json:"contig_id"`
	Gene_ID   string `json:"gene_id"`
	Is_Prot   bool   `json:"is_prot"`
}

// Get region sequence
type RegionGetRequest struct {
	Genome_ID string `json:"genome_id"`
	Contig_ID string `json:"contig_id"`
	Start     uint64 `json:"start"`
	End       uint64 `json:"end"`
	Is_Prot   bool   `json:"is_prot"`
}

// Methods for building samtools request strings
func (g GeneGetRequest) String() string {
	return fmt.Sprintf(
		"%s|%s|%s",
		g.Genome_ID, g.Contig_ID, g.Gene_ID,
	)
}

func (r RegionGetRequest) String() string {
	return fmt.Sprintf(
		"%s|%s:%d-%d",
		r.Genome_ID, r.Contig_ID, r.Start, r.End,
	)
}

// Methods

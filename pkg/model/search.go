package model

// ClusterField names a cluster column a search can filter or sort on.
type ClusterField int

const (
	ClusterFieldFunction ClusterField = iota
	ClusterFieldCOGID
	ClusterFieldClusterID
	ClusterFieldGeneID
	// ClusterFieldTODO is the zero-information field: unknown or missing input.
	ClusterFieldTODO
)

func (s ClusterField) String() string {
	switch s {
	case ClusterFieldFunction:
		return "function"
	case ClusterFieldCOGID:
		return "cog_id"
	case ClusterFieldClusterID:
		return "cluster_id"
	case ClusterFieldGeneID:
		return "gene_id"
	default:
		return "TODO"
	}
}

// ParseClusterField maps a form value to a field; unknown values give ClusterFieldTODO.
func ParseClusterField(field string) ClusterField {
	switch field {
	case "function":
		return ClusterFieldFunction
	case "cog_id":
		return ClusterFieldCOGID
	case "cluster_id":
		return ClusterFieldClusterID
	case "gene_id":
		return ClusterFieldGeneID
	default:
		return ClusterFieldTODO
	}
}

// ClusterSearchRequest describes one page of the cluster heatmap.
type ClusterSearchRequest struct {
	Search_For              string       `json:"search_for"`                 // term to search for
	Search_Field            ClusterField `json:"search_field"`               // field to search in
	Order_By                ClusterField `json:"order_by"`                   // field to sort on
	Order_Dir               string       `json:"order_dir"`                  // asc or desc (not applied by the query yet)
	Page                    int          `json:"page"`                       // 1-based page number
	Page_Size               int          `json:"page_size"`                  // clusters per page
	Genome_IDs              []string     `json:"genome_ids"`                 // only show members from these genomes; empty = all
	RequireGenesFromGenomes []string     `json:"require_genes_from_genomes"` // unused
	Color_By                string       `json:"color_by"`                   // gene_copy_number or max_gene_completeness
}

// Offset is the number of clusters before the requested page.
func (r ClusterSearchRequest) Offset() int {
	if r.Page < 1 {
		return 0
	}
	return (r.Page - 1) * r.Page_Size
}

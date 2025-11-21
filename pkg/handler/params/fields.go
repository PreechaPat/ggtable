package params

type ClusterField int

const (
	ClusterFieldFunction ClusterField = iota
	ClusterFieldCOGID
	ClusterFieldClusterID
	ClusterFieldGeneID
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
	case ClusterFieldTODO:
		return "TODO"
	default:
		return "TODO"
	}
}

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
	case "TODO":
		return ClusterFieldTODO
	default:
		return ClusterFieldTODO // default to function
	}
}

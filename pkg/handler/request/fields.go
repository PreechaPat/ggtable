package request

type ClusterField int

const (
	ClusterFieldFunction ClusterField = iota
	ClusterFieldCOG
	ClusterFieldClusterID
	ClusterFieldTODO
)

func (s ClusterField) String() string {
	switch s {
	case ClusterFieldFunction:
		return "function"
	case ClusterFieldCOG:
		return "cog"
	case ClusterFieldClusterID:
		return "cluster_id"
	case ClusterFieldTODO:
		return "TODO"
	default:
		return "TODO"
	}
}

func NewClusterField(field string) ClusterField {
	switch field {
	case "function":
		return ClusterFieldFunction
	case "cog":
		return ClusterFieldCOG
	case "cluster_id":
		return ClusterFieldClusterID
	case "TODO":
		return ClusterFieldTODO
	default:
		return ClusterFieldTODO // default to function
	}
}

package types

type SearchField int

const (
	SearchFieldFunction SearchField = iota
	SearchFieldCOG
	SearchFieldClusterID
)

func (s SearchField) String() string {
	switch s {
	case SearchFieldFunction:
		return "function"
	case SearchFieldCOG:
		return "cog"
	case SearchFieldClusterID:
		return "cluster_id"
	default:
		return "unknown"
	}
}

func ParseSearchField(field string) SearchField {
	switch field {
	case "function":
		return SearchFieldFunction
	case "cog":
		return SearchFieldCOG
	case "cluster_id":
		return SearchFieldClusterID
	default:
		return SearchFieldFunction // default to function
	}
}

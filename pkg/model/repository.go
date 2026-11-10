package model

import "context"

// ClusterStore reads cluster metadata. Implementations own storage and query details.
type ClusterStore interface {
	Cluster(context.Context, string) (*Cluster, error)
	ClusterIDsForGene(context.Context, string, string) ([]string, error)
	MainPage(context.Context, ClusterSearchRequest) ([]*Cluster, error)
	Search(context.Context, ClusterSearchRequest) ([]*Cluster, error)
	CountSearch(context.Context, ClusterSearchRequest) (int, error)
	CountClusters(context.Context) (int, error)
}

// SequenceStore reads FASTA records and executes searches against sequence data.
type SequenceStore interface {
	Gene(context.Context, GeneRef, bool) ([]byte, error)
	Region(context.Context, RegionRef) ([]byte, error)
	Genes(context.Context, []GeneRef, bool) ([]byte, error)
	Regions(context.Context, []RegionRef) ([]byte, error)
	Search(context.Context, string, string) ([]byte, error)
}

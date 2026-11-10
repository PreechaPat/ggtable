package handler

import (
	"github.com/yumyai/ggtable/pkg/model"
	"github.com/yumyai/ggtable/pkg/render"
)

// AppContext carries the dependencies every handler needs.
type AppContext struct {
	Clusters     model.ClusterStore  // SQLite: clusters, genes, regions
	Seqs         model.SequenceStore // BLAST DBs: sequences and BLAST search
	Genomes      model.GenomeCatalog // heatmap columns and genome names
	BlastManager *model.BlastManager // in-memory BLAST jobs
	Site         render.SiteInfo     // page header title/subtitle
}

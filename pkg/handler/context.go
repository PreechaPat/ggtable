package handler

// DI for all handlers and models alike.

import (
	"github.com/yumyai/ggtable/pkg/db"
	"github.com/yumyai/ggtable/pkg/render"
)

type AppContext struct {
	GCDB         *db.GeneClusterDB
	BlastManager *db.BlastManager
	ProtBLASTDB  string
	NuclBLASTDB  string
	Site         render.SiteInfo // page header title/subtitle
}

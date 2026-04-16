package handler

// DI for all handlers and models alike.

import (
	"github.com/yumyai/ggtable/pkg/db"
)

type AppContext struct {
	GCDB         *db.GeneClusterDB
	BlastManager *db.BlastManager
	ProtBLASTDB  string
	NuclBLASTDB  string
}

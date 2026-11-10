package data

import (
	"github.com/yumyai/ggtable/pkg/data/blastdb"
	"github.com/yumyai/ggtable/pkg/data/sqldb"
	"github.com/yumyai/ggtable/pkg/model"
)

var _ model.ClusterStore = (*sqldb.DB)(nil)
var _ model.SequenceStore = (*blastdb.DB)(nil)

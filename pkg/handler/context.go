package handler

// DI for all handlers and models alike.

import (
	"database/sql"

	mydb "github.com/yumyai/ggtable/pkg/db"
)

type DBContext struct {
	DB           *sql.DB
	Sequence_DB  *mydb.SequenceDB
	ProtBLAST_DB string
	NuclBLAST_DB string
}

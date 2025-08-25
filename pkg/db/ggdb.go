package db

import "database/sql"

type GGDB struct {
	genetableSQL *sql.DB
	SeqDB        *SequenceDB
}

func NewGGDB(db *sql.DB, seqdb *SequenceDB) *GGDB {
	// Check for db schema and version here later
	return &GGDB{
		genetableSQL: db,
		SeqDB:        seqdb,
	}
}

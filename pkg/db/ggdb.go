package db

import "database/sql"

type GeneClusterDB struct {
	SQL   *sql.DB
	SeqDB *SequenceDB
}

func NewGeneClusterDB(db *sql.DB, seqdb *SequenceDB) *GeneClusterDB {
	// Check for db schema and version here later
	return &GeneClusterDB{
		SQL:   db,
		SeqDB: seqdb,
	}
}

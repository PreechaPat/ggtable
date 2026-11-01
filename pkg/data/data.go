// Package data opens the data directory ggtable serves: one SQLite database
// and three BLAST databases, at fixed paths that ggprep and ggorigin write.
//
//	<dir>/db/gene_table.db
//	<dir>/db/blastdb/genetable_genes_prot   gene proteins
//	<dir>/db/blastdb/genetable_genes_nucl   gene nucleotides
//	<dir>/db/blastdb/genetable_genomes      contigs
package data

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yumyai/ggtable/pkg/data/blastdb"
	"github.com/yumyai/ggtable/pkg/data/sqldb"
	"github.com/yumyai/ggtable/pkg/model"
)

// Paths inside the data directory.
const (
	SQLiteFile = "db/gene_table.db"
	ProtDB     = "db/blastdb/genetable_genes_prot"
	NuclDB     = "db/blastdb/genetable_genes_nucl"
	GenomeDB   = "db/blastdb/genetable_genomes"
)

// Store is an opened, checked data directory.
type Store struct {
	Clusters *sqldb.DB
	Seqs     *blastdb.DB
	Genomes  model.GenomeCatalog
}

// Open opens dir and checks it before the server starts: the layout, the
// SQLite schema, the BLAST databases, and that one gene from the SQLite
// tables can be fetched from the BLAST databases (same build, same IDs).
// Every error says what is wrong and how to fix it.
func Open(dir string) (*Store, error) {
	if err := checkDir(dir); err != nil {
		return nil, err
	}
	clusters, err := sqldb.Open(filepath.Join(dir, SQLiteFile))
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			clusters.Close()
		}
	}()

	seqs, err := blastdb.Open(filepath.Join(dir, ProtDB), filepath.Join(dir, NuclDB), filepath.Join(dir, GenomeDB))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	genomes, err := clusters.Genomes(ctx)
	if err != nil {
		return nil, err
	}
	sample, err := clusters.SampleGene(ctx)
	if err != nil {
		return nil, fmt.Errorf("gene_matches has no genes: %w", err)
	}
	if err := seqs.CheckGene(ctx, sample); err != nil {
		return nil, err
	}

	ok = true
	return &Store{Clusters: clusters, Seqs: seqs, Genomes: genomes}, nil
}

// Close releases the SQLite connection pool.
func (s *Store) Close() error { return s.Clusters.Close() }

// checkDir reports a wrong data directory before looking at any file, and
// spots the common mistake of pointing at db/ instead of its parent.
func checkDir(dir string) error {
	info, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("it does not exist; pass -data or set GGTABLE_DATA")
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("it is not a directory")
	}
	if _, err := os.Stat(filepath.Join(dir, SQLiteFile)); err == nil {
		return nil
	}
	if _, err := os.Stat(filepath.Join(dir, "gene_table.db")); err == nil {
		return fmt.Errorf("it has gene_table.db at its top level; point -data at its parent, which must contain %s", SQLiteFile)
	}
	return fmt.Errorf("it has no %s; expected layout: %s, %s.*, %s.*, %s.*",
		SQLiteFile, SQLiteFile, ProtDB, NuclDB, GenomeDB)
}

// Package testdb builds a small SQLite database with the schema ggprep emits,
// so tests can exercise real queries without the multi-GB production data.
package testdb

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// Schema mirrors ggprep/steps/build_sqlite.sh (only the tables ggtable reads).
const Schema = `
CREATE TABLE gene_clusters (
  cluster_id TEXT PRIMARY KEY, unknown_column TEXT, cog_id TEXT,
  representative_gene TEXT, function_description TEXT, expected_length INTEGER);
CREATE TABLE gene_matches (
  gene_match_id TEXT PRIMARY KEY, cluster_id TEXT, genome_id TEXT,
  contig_id TEXT, gene_id TEXT);
CREATE TABLE region_matches (
  region_match_id TEXT PRIMARY KEY, cluster_id TEXT, genome_id TEXT,
  contig_id TEXT, contig_length INTEGER, start_location INTEGER, end_location INTEGER);
CREATE TABLE genome_info (
  genome_id TEXT PRIMARY KEY, genome_local_url TEXT, genome_fullname TEXT,
  genome_fullname_ns TEXT, genome_shortname TEXT, genome_url TEXT);
CREATE TABLE gene_info (
  genome_id TEXT, gene_id TEXT, start_location INTEGER, end_location INTEGER,
  gene_length INTEGER, attribute TEXT, contig_id TEXT, NA TEXT,
  description TEXT, score REAL, PRIMARY KEY (genome_id, gene_id));
`

// Data is a fixed dataset the tests pin their expectations to.
//
//	G1 "Genome One", G2 "Genome Two", G3 "Genome Three"
//	c1  "ATP synthase"  COG0001  expected 100  genes: G1 g1a(100) g1b(50), G2 g2a(80)   region: G3
//	c2  "Kinase"        COG0002  expected 200  genes: G2 g2b(200)
//	c3  "Hypothetical"  COG0003  expected 300  (no genes, no regions)
//	c4  "ATP binding"   COG0004  expected 400  genes: G1 g1c(100)
const Data = `
INSERT INTO genome_info (genome_id, genome_fullname) VALUES
  ('G1','Genome One'), ('G2','Genome Two'), ('G3','Genome Three');

INSERT INTO gene_clusters (cluster_id, cog_id, representative_gene, function_description, expected_length) VALUES
  ('c1','COG0001','g1a','ATP synthase',100),
  ('c2','COG0002','g2b','Kinase',200),
  ('c3','COG0003','','Hypothetical',300),
  ('c4','COG0004','g1c','ATP binding',400);

INSERT INTO gene_info (genome_id, gene_id, start_location, end_location, gene_length, contig_id, description) VALUES
  ('G1','g1a',1,300,100,'ctgA','synthase alpha'),
  ('G1','g1b',400,550,50,'ctgA','synthase alpha fragment'),
  ('G2','g2a',10,250,80,'ctgB','synthase alpha 2'),
  ('G2','g2b',10,610,200,'ctgB','kinase'),
  ('G1','g1c',900,2100,100,'ctgA','binding');

INSERT INTO gene_matches (gene_match_id, cluster_id, genome_id, contig_id, gene_id) VALUES
  ('m0','c1','G1','ctgA','g1a'),
  ('m1','c1','G1','ctgA','g1b'),
  ('m2','c1','G2','ctgB','g2a'),
  ('m3','c2','G2','ctgB','g2b'),
  ('m4','c4','G1','ctgA','g1c');

INSERT INTO region_matches (region_match_id, cluster_id, genome_id, contig_id, contig_length, start_location, end_location) VALUES
  ('r0','c1','G3','ctgC',5000,100,900);
`

// New returns an open handle to a fresh, populated database in t.TempDir().
func New(t testing.TB) *sql.DB {
	t.Helper()
	return open(t, Schema+Data)
}

// Empty returns a database with the schema but no rows.
func Empty(t testing.TB) *sql.DB {
	t.Helper()
	return open(t, Schema)
}

// WriteSQLite writes the populated database to path.
func WriteSQLite(t testing.TB, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("testdb: open %s: %v", path, err)
	}
	defer db.Close()
	if _, err := db.Exec(Schema + Data); err != nil {
		t.Fatalf("testdb: populate %s: %v", path, err)
	}
}

func open(t testing.TB, script string) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gene_table.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("testdb: open %s: %v", path, err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(script); err != nil {
		t.Fatalf("testdb: populate %s: %v", path, err)
	}
	return db
}

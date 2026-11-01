// Package sqldb reads clusters, genes, regions and genomes from the SQLite
// database built by ggprep/steps/build_sqlite.sh (or ggorigin step 06).
package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/yumyai/ggtable/pkg/model"
	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// DB is a read-only view of gene_table.db. It is safe for concurrent use.
type DB struct {
	sql *sql.DB
}

// requiredColumns lists every table and column the queries read.
var requiredColumns = map[string][]string{
	"gene_clusters":  {"cluster_id", "cog_id", "representative_gene", "function_description", "expected_length"},
	"gene_matches":   {"cluster_id", "genome_id", "contig_id", "gene_id"},
	"region_matches": {"cluster_id", "genome_id", "contig_id", "start_location", "end_location"},
	"genome_info":    {"genome_id", "genome_fullname"},
	"gene_info":      {"genome_id", "gene_id", "start_location", "end_location", "gene_length", "description"},
}

// Open opens an existing database file and checks it has the expected schema.
// It never creates a file: a wrong path is an error.
func Open(path string) (*DB, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open SQLite database %s: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("cannot open SQLite database %s: it is a directory, expected the gene_table.db file", path)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=synchronous(NORMAL)", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("cannot open SQLite database %s: %w", path, err)
	}
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(3 * time.Minute)

	db := New(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.CheckSchema(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return db, nil
}

// New wraps an open connection without checking it (for tests).
func New(conn *sql.DB) *DB { return &DB{sql: conn} }

// Close closes the connection pool.
func (db *DB) Close() error { return db.sql.Close() }

// CheckSchema reports every missing table or column in one error, and fails
// if there are no genomes or clusters.
func (db *DB) CheckSchema(ctx context.Context) error {
	tables := make([]string, 0, len(requiredColumns))
	for t := range requiredColumns {
		tables = append(tables, t)
	}
	sort.Strings(tables)

	var problems []string
	for _, table := range tables {
		have, err := db.columns(ctx, table)
		if err != nil {
			return fmt.Errorf("cannot read schema (is this a SQLite file?): %w", err)
		}
		if len(have) == 0 {
			problems = append(problems, fmt.Sprintf("table %s is missing", table))
			continue
		}
		var missing []string
		for _, col := range requiredColumns[table] {
			if !have[col] {
				missing = append(missing, col)
			}
		}
		if len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("table %s is missing columns %s", table, strings.Join(missing, ", ")))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("not a gene table database: %s; rebuild it with ggprep (make sqlite) or ggorigin", strings.Join(problems, "; "))
	}

	for _, table := range []string{"genome_info", "gene_clusters"} {
		var n int
		if err := db.sql.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
			return fmt.Errorf("count rows in %s: %w", table, err)
		}
		if n == 0 {
			return fmt.Errorf("table %s is empty; the import step probably failed, rebuild the database", table)
		}
	}
	return nil
}

func (db *DB) columns(ctx context.Context, table string) (map[string]bool, error) {
	// table comes from requiredColumns, never from user input.
	rows, err := db.sql.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

// withReadTx runs fn in a read-only transaction that is always rolled back,
// which also drops the temporary tables the search queries create.
func (db *DB) withReadTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.sql.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		// Rollback is the intended end of every read; its error carries nothing to act on.
		_ = tx.Rollback()
	}()
	return fn(tx)
}

func notFound(what, id string) error {
	return fmt.Errorf("%s %q: %w", what, id, model.ErrNotFound)
}

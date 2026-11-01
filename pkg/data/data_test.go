package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yumyai/ggtable/internal/testdb"
)

func TestOpen(t *testing.T) {
	dir := testdb.DataDir(t)
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if got := store.Genomes.IDs(); len(got) != 3 {
		t.Errorf("genomes = %v, want 3", got)
	}
}

// Each broken data directory must fail with a message that names the problem.
func TestOpen_Errors(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string // returns the -data argument
		want  string
	}{
		{"missing dir", func(t *testing.T) string { return filepath.Join(t.TempDir(), "nope") }, "does not exist"},
		{"empty dir", func(t *testing.T) string { return t.TempDir() }, "expected layout"},
		{"pointed at db/", func(t *testing.T) string { return filepath.Join(testdb.DataDir(t), "db") }, "point -data at its parent"},
		{"missing BLAST DB", func(t *testing.T) string {
			dir := testdb.DataDir(t)
			removeGlob(t, filepath.Join(dir, GenomeDB)+".*")
			return dir
		}, "genetable_genomes (no .nin/.nal file)"},
		{"wrong schema", func(t *testing.T) string {
			dir := testdb.DataDir(t)
			path := filepath.Join(dir, SQLiteFile)
			os.Remove(path)
			os.WriteFile(path, []byte("not a database"), 0o644)
			return dir
		}, "is this a SQLite file?"},
		{"BLAST IDs use another separator", func(t *testing.T) string {
			dir := testdb.DataDir(t)
			otherIDs := map[string]string{}
			for id, seq := range testdb.Proteins {
				otherIDs[strings.ReplaceAll(id, "//", "__")] = seq
			}
			removeGlob(t, filepath.Join(dir, ProtDB)+".*")
			testdb.MakeBlastDB(t, filepath.Join(dir, ProtDB), "prot", otherIDs)
			return dir
		}, "sequence IDs are not GENOME//CONTIG//GENE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Open(tt.setup(t))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Open() = %v, want error containing %q", err, tt.want)
			}
		})
	}
}

func removeGlob(t *testing.T, pattern string) {
	t.Helper()
	files, _ := filepath.Glob(pattern)
	if len(files) == 0 {
		t.Fatalf("no files match %s", pattern)
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			t.Fatal(err)
		}
	}
}

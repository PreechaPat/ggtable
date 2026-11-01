package testdb

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Gene and contig sequences matching Data. They are random but fixed (seeded),
// long enough for BLAST to report a self hit.
var (
	Genes = map[string]string{ // GENOME//CONTIG//GENE -> nucleotides
		"G1//ctgA//g1a": randomSeq(1, "ACGT", 300),
		"G1//ctgA//g1b": randomSeq(2, "ACGT", 150),
		"G2//ctgB//g2a": randomSeq(3, "ACGT", 240),
		"G2//ctgB//g2b": randomSeq(4, "ACGT", 600),
		"G1//ctgA//g1c": randomSeq(5, "ACGT", 300),
	}
	Proteins = map[string]string{ // GENOME//CONTIG//GENE -> amino acids
		"G1//ctgA//g1a": randomSeq(11, "ACDEFGHIKLMNPQRSTVWY", 100),
		"G1//ctgA//g1b": randomSeq(12, "ACDEFGHIKLMNPQRSTVWY", 50),
		"G2//ctgB//g2a": randomSeq(13, "ACDEFGHIKLMNPQRSTVWY", 80),
		"G2//ctgB//g2b": randomSeq(14, "ACDEFGHIKLMNPQRSTVWY", 200),
		"G1//ctgA//g1c": randomSeq(15, "ACDEFGHIKLMNPQRSTVWY", 100),
	}
	Contigs = map[string]string{ // GENOME//CONTIG -> nucleotides
		"G1//ctgA": randomSeq(21, "ACGT", 2200),
		"G2//ctgB": randomSeq(22, "ACGT", 700),
		"G3//ctgC": randomSeq(23, "ACGT", 5000),
	}
)

func randomSeq(seed int64, alphabet string, n int) string {
	r := rand.New(rand.NewSource(seed))
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[r.Intn(len(alphabet))]
	}
	return string(b)
}

// RequireBlast skips the test when NCBI BLAST+ is not installed.
func RequireBlast(t testing.TB) {
	t.Helper()
	for _, exe := range []string{"makeblastdb", "blastdbcmd", "blastn", "blastp"} {
		if _, err := exec.LookPath(exe); err != nil {
			t.Skipf("%s not on PATH; install NCBI BLAST+ (e.g. conda activate bio) to run this test", exe)
		}
	}
}

// DataDir builds a complete data directory in t.TempDir(), laid out like
// ggprep's output (db/gene_table.db and db/blastdb/*), and returns its path.
// It skips the test when BLAST+ is not installed.
func DataDir(t testing.TB) string {
	t.Helper()
	RequireBlast(t)
	dir := t.TempDir()
	blastDir := filepath.Join(dir, "db", "blastdb")
	if err := os.MkdirAll(blastDir, 0o755); err != nil {
		t.Fatalf("testdb: %v", err)
	}
	WriteSQLite(t, filepath.Join(dir, "db", "gene_table.db"))
	MakeBlastDB(t, filepath.Join(blastDir, "genetable_genes_prot"), "prot", Proteins)
	MakeBlastDB(t, filepath.Join(blastDir, "genetable_genes_nucl"), "nucl", Genes)
	MakeBlastDB(t, filepath.Join(blastDir, "genetable_genomes"), "nucl", Contigs)
	return dir
}

// MakeBlastDB writes seqs (ID -> sequence) to a BLAST database at out,
// with -parse_seqids as ggprep/steps/build_blastdb.sh does.
func MakeBlastDB(t testing.TB, out, dbtype string, seqs map[string]string) {
	t.Helper()
	var fasta strings.Builder
	for id, seq := range seqs {
		fmt.Fprintf(&fasta, ">%s\n%s\n", id, seq)
	}
	cmd := exec.Command("makeblastdb", "-in", "-", "-parse_seqids", "-dbtype", dbtype, "-title", filepath.Base(out), "-out", out)
	cmd.Stdin = strings.NewReader(fasta.String())
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("testdb: makeblastdb %s: %v\n%s", out, err, b)
	}
}

package blastdb

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yumyai/ggtable/internal/testdb"
	"github.com/yumyai/ggtable/pkg/model"
)

var ctx = context.Background()

func openFixture(t *testing.T) *DB {
	t.Helper()
	dir := filepath.Join(testdb.DataDir(t), "db", "blastdb")
	db, err := Open(filepath.Join(dir, "genetable_genes_prot"), filepath.Join(dir, "genetable_genes_nucl"), filepath.Join(dir, "genetable_genomes"))
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// seqOf returns the sequence of a single-record FASTA, without line breaks.
func seqOf(t *testing.T, fasta []byte) string {
	t.Helper()
	_, body, ok := strings.Cut(string(fasta), "\n")
	if !ok || !strings.HasPrefix(string(fasta), ">") {
		t.Fatalf("not FASTA: %q", fasta)
	}
	return strings.ReplaceAll(body, "\n", "")
}

func TestOpen_Missing(t *testing.T) {
	testdb.RequireBlast(t)
	dir := t.TempDir()
	_, err := Open(filepath.Join(dir, "p"), filepath.Join(dir, "n"), filepath.Join(dir, "g"))
	if err == nil {
		t.Fatal("Open() of missing DBs succeeded")
	}
	for _, want := range []string{filepath.Join(dir, "p") + " (no .pin/.pal file)", filepath.Join(dir, "g"), "make blastdb"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestGene(t *testing.T) {
	db := openFixture(t)
	ref := model.GeneRef{GenomeID: "G2", ContigID: "ctgB", GeneID: "g2b"}

	prot, err := db.Gene(ctx, ref, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(prot), ">G2//ctgB//g2b") || seqOf(t, prot) != testdb.Proteins[ref.SeqID()] {
		t.Errorf("protein = %q", prot)
	}
	nucl, err := db.Gene(ctx, ref, false)
	if err != nil || seqOf(t, nucl) != testdb.Genes[ref.SeqID()] {
		t.Errorf("nucleotide = %q, %v", nucl, err)
	}

	_, err = db.Gene(ctx, model.GeneRef{GenomeID: "G2", ContigID: "ctgB", GeneID: "nope"}, true)
	if err == nil || !strings.Contains(err.Error(), "G2//ctgB//nope") {
		t.Errorf("unknown gene error = %v, want the ID and blastdbcmd's reason", err)
	}
}

func TestRegion(t *testing.T) {
	db := openFixture(t)
	ref := model.RegionRef{GenomeID: "G3", ContigID: "ctgC", Start: 100, End: 900}
	fasta, err := db.Region(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := seqOf(t, fasta), testdb.Contigs["G3//ctgC"][99:900]; got != want {
		t.Errorf("region has %d bases, want %d (1-based inclusive range)", len(got), len(want))
	}
	if _, err := db.Region(ctx, model.RegionRef{GenomeID: "G3", ContigID: "ctgC"}); err == nil {
		t.Error("Region() accepted a region without a location")
	}
}

func TestGenesAndRegions_Batch(t *testing.T) {
	db := openFixture(t)
	genes, err := db.Genes(ctx, []model.GeneRef{
		{GenomeID: "G1", ContigID: "ctgA", GeneID: "g1a"},
		{GenomeID: "G2", ContigID: "ctgB", GeneID: "g2a"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(genes, []byte(">")); n != 2 {
		t.Errorf("Genes() returned %d records, want 2:\n%s", n, genes)
	}

	regions, err := db.Regions(ctx, []model.RegionRef{
		{GenomeID: "G3", ContigID: "ctgC", Start: 100, End: 900},
		{GenomeID: "G1", ContigID: "ctgA", Start: 1, End: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	records := strings.Split(strings.TrimPrefix(string(regions), ">"), "\n>")
	if len(records) != 2 {
		t.Fatalf("Regions() returned %d records, want 2:\n%s", len(records), regions)
	}
	if got := seqOf(t, []byte(">"+records[1])); got != testdb.Contigs["G1//ctgA"][:50] {
		t.Errorf("second region = %q, want the first 50 bases of ctgA", got)
	}

	if out, err := db.Genes(ctx, nil, true); err != nil || out != nil {
		t.Errorf("Genes(nil) = %q, %v", out, err)
	}
}

func TestSearch(t *testing.T) {
	db := openFixture(t)
	query := ">q\n" + testdb.Proteins["G2//ctgB//g2b"]

	report, err := db.Search(ctx, "blastp", query)
	if err != nil {
		t.Fatal(err)
	}
	cat := model.NewGenomeCatalog([]string{"G2"}, map[string]string{"G2": "Genome Two"})
	out, err := model.AnnotateBlastReport(bytes.NewReader(report), cat)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Genome Two-G2//ctgB//g2b", `<a href="/cluster/heatmap/G2/ctgB/g2b">View in gene table</a>`} {
		if !strings.Contains(out, want) {
			t.Errorf("annotated report is missing %q", want)
		}
	}

	if _, err := db.Search(ctx, "tblastx", query); err == nil {
		t.Error("Search() accepted tblastx")
	}
}

func TestCheckGene(t *testing.T) {
	db := openFixture(t)
	if err := db.CheckGene(ctx, model.GeneRef{GenomeID: "G1", ContigID: "ctgA", GeneID: "g1a"}); err != nil {
		t.Errorf("CheckGene(known gene) = %v", err)
	}
	err := db.CheckGene(ctx, model.GeneRef{GenomeID: "G1", ContigID: "ctgA", GeneID: "other"})
	if err == nil || !strings.Contains(err.Error(), "different builds") {
		t.Errorf("CheckGene(unknown gene) = %v", err)
	}
}

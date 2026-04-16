package db

import (
	"fmt"
	"os"
	"path"
	"testing"
)

func mockInput(t testing.T) {
	tmpDir, err := os.MkdirTemp("", "seqdb_test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create the necessary directory structure
	sequencesDir := path.Join(tmpDir, "sequences")
	concatDir := path.Join(sequencesDir, "concat_sequences")
	err = os.MkdirAll(concatDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}

	// Create mock fasta files
	mockFaa := path.Join(concatDir, "all_genes.faa.gz")
	mockFna := path.Join(concatDir, "all_genes.fna.gz")

	// Write mock data to files (you may need to adjust this based on your actual data format)
	mockData := ">KCB09|gene123|KCB0_00064\nMOCKPROTEINSEQUENCE\n"
	err = os.WriteFile(mockFaa, []byte(mockData), 0644)
	if err != nil {
		t.Fatalf("Failed to write mock faa file: %v", err)
	}
	err = os.WriteFile(mockFna, []byte(mockData), 0644)
	if err != nil {
		t.Fatalf("Failed to write mock fna file: %v", err)
	}
}

func TestGetGeneSequence(t *testing.T) {
	sdb := SequenceDB{
		Dir: "/data/db/sequence_db",
	}

	seq, err := sdb.GetGeneSequence("KCB09", "contig000007", "KCB09_00064", true)

	if err != nil {
		t.Errorf("Out")
	}

	fmt.Println(seq)

}
func TestGetRegionSequence(t *testing.T) {

	sdb := SequenceDB{
		Dir: "/data/db/sequence_db",
	}

	seq, err := sdb.GetRegionSequence("KCB09", "contig000007", 1, 20)

	if err != nil {
		t.Errorf("Error shouldn't happen")
	}

	// Check that the sequence length is ....
	fmt.Println(seq)
}

func TestGetmultipleGenes(t *testing.T) {

	sdb := SequenceDB{
		Dir: "/data/db/sequence_db",
	}

	gene_reqs := []string{
		"KCB09|contig000007|KCB09_00064",
		"MCC17|contig000758|MCC17_10868",
	}

	seq, err := sdb.GetMultipleGene(gene_reqs, true)

	if err != nil {
		t.Errorf("Error shouldn't happen")
	}

	// Check that the sequence length is ....
	fmt.Println(seq)
}

func TestGetMultipleRegion(t *testing.T) {

	sdb := SequenceDB{
		Dir: "/data/db/sequence_db",
	}

	region_reqs := []string{
		"KCB09|contig000007:1-20",
		"KCB09|contig000017:200-400",
	}

	seq, err := sdb.GetMultipleRegion(region_reqs)

	if err != nil {
		t.Errorf("Error shouldn't happen")
	}

	// Check that the sequence length is ....
	fmt.Println(seq)
}

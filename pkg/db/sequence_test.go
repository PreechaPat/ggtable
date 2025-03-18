package db

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/yumyai/ggtable/pkg/handler/types"
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

	req := types.GeneRequest{
		Genome_ID: "KCB09",
		Contig_ID: "contig000007",
		Gene_ID:   "KCB09_00064",
		Is_Prot:   true,
	}

	seq, err := sdb.GetGeneSequence(req)

	if err != nil {
		t.Errorf("Out")
	}

	fmt.Println(seq)

}
func TestGetRegionSequence(t *testing.T) {

	sdb := SequenceDB{
		Dir: "/data/db/sequence_db",
	}

	req := types.RegionRequest{
		Genome_ID: "KCB09",
		Contig_ID: "contig000007",
		Start:     1,
		End:       20,
		Is_Prot:   false,
	}

	seq, err := sdb.GetRegionSequence(req)

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

	gene_reqs := []*types.GeneRequest{
		{
			Genome_ID: "KCB09",
			Contig_ID: "contig000007",
			Gene_ID:   "KCB09_00064",
			Is_Prot:   true,
		},
		{
			Genome_ID: "MCC17",
			Contig_ID: "contig000758",
			Gene_ID:   "MCC17_10868",
			Is_Prot:   true,
		},
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

	region_reqs := []*types.RegionRequest{
		&types.RegionRequest{
			Genome_ID: "KCB09",
			Contig_ID: "contig000007",
			Start:     1,
			End:       20,
			Is_Prot:   false,
		},
		&types.RegionRequest{
			Genome_ID: "KCB09",
			Contig_ID: "contig000017",
			Start:     200,
			End:       400,
			Is_Prot:   false,
		},
	}

	seq, err := sdb.GetMultipleRegion(region_reqs)

	if err != nil {
		t.Errorf("Error shouldn't happen")
	}

	// Check that the sequence length is ....
	fmt.Println(seq)
}

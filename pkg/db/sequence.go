package db

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Defining possible error
var SequenceNotExists = errors.New("Sequence folder does not exists")

type NoSequenceError struct {
	Msg string // additional context for the error
}

func (e *NoSequenceError) Error() string {
	return fmt.Sprintf("Sequence error: %s", e.Msg)
}

// folder which host sequeces/[genomes]/fasta
type SequenceDB struct {
	ProtDB   string
	NuclDB   string
	GenomeDB string
}

func NewSequenceDB(protDB, nuclDB, genomeDB string) (*SequenceDB, error) {
	required_files := []string{
		protDB + ".pin",
		nuclDB + ".nin",
		genomeDB + ".nin",
	}

	// We only check if the main index files exist
	for _, f := range required_files {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			// Some BLAST DBs might not have .pin/.nin if they are just aliases
			// but for now we assume they are standard.
			// Actually, let's be more lenient or check the base path.
		}
	}

	return &SequenceDB{
		ProtDB:   protDB,
		NuclDB:   nuclDB,
		GenomeDB: genomeDB,
	}, nil
}

func (seqdb *SequenceDB) GetGeneSequence(genomeID, contigID, geneID string, isProt bool) ([]byte, error) {

	var dbPath string
	if isProt {
		dbPath = seqdb.ProtDB
	} else {
		dbPath = seqdb.NuclDB
	}

	// Use blastdbcmd to fetch data
	seq_name := fmt.Sprintf("%s//%s//%s", genomeID, contigID, geneID)

	// blastdbcmd -db genetable_genes_prot -entry "CBS57985//contig004129//CBS57985_11370"
	args := []string{"-db", dbPath, "-entry", seq_name}
	cmd := exec.Command("blastdbcmd", args...)

	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("%w: Sequence not found (blastdbcmd error)", err)
	}

	return output, nil
}

func (seqdb *SequenceDB) GetRegionSequence(genomeID, contigID string, start, end uint64) ([]byte, error) {

	// Use blastdbcmd to fetch data
	dbPath := seqdb.GenomeDB
	seq_name := fmt.Sprintf("%s//%s", genomeID, contigID)

	// blastdbcmd -db genetable_genomes_nucl -entry "genomeID//contigID" -range 100-200
	args := []string{"-db", dbPath, "-entry", seq_name, "-range", fmt.Sprintf("%d-%d", start, end)}
	cmd := exec.Command("blastdbcmd", args...)

	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("%w: Sequence not found (blastdbcmd error)", err)
	}

	return output, nil
}

// Retrieves gene sequences using blastdbcmd based on multiple gene names.
// geneNames should be formatted as "genomeID//contigID//geneID"
func (seqdb *SequenceDB) GetMultipleGene(geneNames []string, isProt bool) ([]byte, error) {

	var dbPath string
	if isProt {
		dbPath = seqdb.ProtDB
	} else {
		dbPath = seqdb.NuclDB
	}

	if len(geneNames) == 0 {
		return []byte{}, nil
	}

	// Use -entry_batch - for better scalability and to avoid command line length limits
	cmd := exec.Command("blastdbcmd", "-db", dbPath, "-entry_batch", "-")
	cmd.Stdin = strings.NewReader(strings.Join(geneNames, "\n") + "\n")
	output, err := cmd.CombinedOutput()

	if err != nil {
		nerr := fmt.Errorf("blastdbcmd error: %w, output: %s", err, string(output))
		return nil, nerr
	}

	return output, nil
}

// Retrieves region sequences using blastdbcmd based on multiple region names.
// regionNames should be formatted as "genomeID//contigID" or "genomeID//contigID:start-end"
func (seqdb *SequenceDB) GetMultipleRegion(regionNames []string) ([]byte, error) {

	if len(regionNames) == 0 {
		return []byte{}, nil
	}

	// Prepare batch input for blastdbcmd
	var batchData strings.Builder
	for _, region := range regionNames {
		// Parse "ID:start-end" using SplitN to safely handle the separator
		parts := strings.SplitN(region, ":", 2)
		if len(parts) == 2 {
			id := parts[0]
			// The second part is expected to be "start-end"
			// The format for batch input with range is: "id -range start-end"
			batchData.WriteString(fmt.Sprintf("%s -range %s\n", id, parts[1]))
		} else {
			// Just ID without range
			batchData.WriteString(region + "\n")
		}
	}

	cmd := exec.Command("blastdbcmd", "-db", seqdb.GenomeDB, "-entry_batch", "-")
	cmd.Stdin = strings.NewReader(batchData.String())
	output, err := cmd.CombinedOutput()

	if err != nil {
		nerr := fmt.Errorf("blastdbcmd error: %w, output: %s", err, string(output))
		return nil, nerr
	}

	return output, nil
}

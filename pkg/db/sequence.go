package db

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
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
	Dir string
}

func NewSequenceDB(dir string) (*SequenceDB, error) {
	required_folders := []string{
		dir,
		path.Join(dir, "concat_sequences"),
		path.Join(dir, "concat_sequences", "genetable_genes.fna.gz"),
		path.Join(dir, "concat_sequences", "genetable_genes.faa.gz"),
		path.Join(dir, "concat_sequences", "genetable_genomes.fna.gz"),
	}

	var errs error

	for _, folder := range required_folders {
		if _, err := os.Stat(folder); os.IsNotExist(err) {
			errs = fmt.Errorf("%w: %s", os.ErrNotExist, folder)
		}
	}

	if errs != nil {
		return nil, errs
	} else {
		return &SequenceDB{
			Dir: dir,
		}, nil
	}
}

func (seqdb *SequenceDB) getConcatAllGeneNucl() string {

	return path.Join(seqdb.Dir, "concat_sequences", "genetable_genes.fna.gz")
}

func (seqdb *SequenceDB) getConcatAllGeneProt() string {

	return path.Join(seqdb.Dir, "concat_sequences", "genetable_genes.faa.gz")
}

func (seqdb *SequenceDB) getConcatContigNucl() string {

	return path.Join(seqdb.Dir, "concat_sequences", "genetable_genomes.fna.gz")
}

func (seqdb *SequenceDB) GetGeneSequence(genomeID, contigID, geneID string, isProt bool) ([]byte, error) {

	var all_fasta_file string
	if isProt {
		all_fasta_file = seqdb.getConcatAllGeneProt()
	} else {
		all_fasta_file = seqdb.getConcatAllGeneNucl()
	}

	// Use samtools to fetch data
	seq_name := fmt.Sprintf("%s|%s|%s", genomeID, contigID, geneID)

	// samtools faidx all_genes.faa.gz "KCB09|contig000007|KCB09_00064:50-100"
	args := []string{"faidx", all_fasta_file, seq_name}
	cmd := exec.Command("samtools", args...)

	// The call looks like this.
	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("%w: Sequence not found", err)
	}

	return output, nil
}

func (seqdb *SequenceDB) GetRegionSequence(genomeID, contigID string, start, end uint64) ([]byte, error) {

	// Use samtools to fetch data
	// "KCB09|contig000007|KCB09_00064:50-100"
	all_contigs_file := seqdb.getConcatContigNucl()
	seq_name := fmt.Sprintf("%s|%s:%d-%d", genomeID, contigID, start, end)

	args := []string{"faidx", all_contigs_file, seq_name}
	cmd := exec.Command("samtools", args...)

	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("%w: Sequence not found", err)
	}

	return output, nil
}

// Retrieves gene sequences using Samtools faidx based on multiple gene names.
// geneNames should be formatted as "genomeID|contigID|geneID"
func (seqdb *SequenceDB) GetMultipleGene(geneNames []string, isProt bool) ([]byte, error) {

	var geneInputBuffer bytes.Buffer
	var all_gene_file string

	// Input for samtools ( stdin )
	// The input is multiple lines of sequences id e.g. KCB09|contig000007|KCB09_00064:50-100
	for _, s := range geneNames {
		geneInputBuffer.WriteString(s)
		geneInputBuffer.WriteString("\n")
	}

	if isProt {
		all_gene_file = seqdb.getConcatAllGeneProt()
	} else {
		all_gene_file = seqdb.getConcatAllGeneNucl()
	}

	// cat test.txt | samtools faidx all_genes.faa.gz -r -
	geneArgs := []string{"faidx", all_gene_file, "-r", "-"}
	cmd := exec.Command("samtools", geneArgs...)
	cmd.Stdin = &geneInputBuffer
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Will be print to output (due to stderr.)
		nerr := fmt.Errorf("%s - %s", err, output)
		return nil, nerr
	}

	return output, nil

}

// Retrieves region sequences using Samtools faidx based on multiple region names.
// regionNames should be formatted as "genomeID|contigID:start-end"
func (seqdb *SequenceDB) GetMultipleRegion(regionNames []string) ([]byte, error) {

	var contigInputBuffer bytes.Buffer

	// Make buffer for region
	for _, s := range regionNames {
		contigInputBuffer.WriteString(s)
		contigInputBuffer.WriteString("\n")
	}

	all_contigs_file := seqdb.getConcatContigNucl()
	contigArgs := []string{"faidx", all_contigs_file, "-r", "-"}
	cmd := exec.Command("samtools", contigArgs...)
	// Set up the input for the command
	cmd.Stdin = &contigInputBuffer

	// Capture the stdout and stderr
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Will be print to output (due to stderr.)
		nerr := fmt.Errorf("%s - %s", err, output)
		return nil, nerr
	}

	return output, nil
}

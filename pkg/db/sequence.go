package db

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path"

	"github.com/yumyai/ggtable/internal/util"
	"github.com/yumyai/ggtable/pkg/handler/types"
)

// Defining possible error
var SequenceNotExists = errors.New("Something very specific happened!")

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

// Self check when create
func (seqdb *SequenceDB) Init() error {

	if !util.DirExists(seqdb.Dir) {
		return fmt.Errorf("%w: Base folder does not exists", fs.ErrNotExist)
	}

	if !util.DirExists(seqdb.Dir) {
		return fmt.Errorf("%w: Sequences folder does not exists", fs.ErrNotExist)
	}

	// TODO: Make further check

	return nil
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

func (seqdb *SequenceDB) GetGeneSequence(req types.GeneRequest) ([]byte, error) {

	genome_id := req.Genome_ID
	contig_id := req.Contig_ID
	gene_id := req.Gene_ID
	prot := req.Is_Prot
	var all_fasta_file string
	if prot {
		all_fasta_file = seqdb.getConcatAllGeneProt()
	} else {
		all_fasta_file = seqdb.getConcatAllGeneNucl()
	}

	// Use samtools to fetch data
	seq_name := fmt.Sprintf("%s|%s|%s", genome_id, contig_id, gene_id)

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

func (seqdb *SequenceDB) GetRegionSequence(req types.RegionRequest) ([]byte, error) {

	// Use samtools to fetch data
	// "KCB09|contig000007|KCB09_00064:50-100"
	all_contigs_file := seqdb.getConcatContigNucl()
	seq_name := req.String()

	args := []string{"faidx", all_contigs_file, seq_name}
	cmd := exec.Command("samtools", args...)

	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("%w: Sequence not found", err)
	}

	return output, nil
}

// Retrieves gene sequences using Samtools faidx based on multiple gene requests.
// TODO:
//   - Consider preloading Samtools to improve responsiveness.
func (seqdb *SequenceDB) GetMultipleGene(genereqs []*types.GeneRequest, is_prot bool) ([]byte, error) {

	var geneInputBuffer bytes.Buffer
	var all_gene_file string

	// Input for samtools ( stdin )
	// The input is multiple lines of sequences id e.g. KCB09|contig000007|KCB09_00064:50-100
	for _, s := range genereqs {
		geneInputBuffer.WriteString(s.String())
		geneInputBuffer.WriteString("\n")
	}

	if is_prot {
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

func (seqdb *SequenceDB) GetMultipleRegion(regreqs []*types.RegionRequest) ([]byte, error) {

	var contigInputBuffer bytes.Buffer

	// Make buffer for region
	for _, s := range regreqs {
		contigInputBuffer.WriteString(s.String())
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

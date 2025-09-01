package db

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/yumyai/ggtable/pkg/handler/request"
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

func (seqdb *SequenceDB) GetGeneSequence(req request.GeneGetRequest) ([]byte, error) {

	// genome_id := req.Genome_ID
	// contig_id := req.Contig_ID
	// gene_id := req.Gene_ID
	prot := req.Is_Prot
	var all_fasta_file string
	if prot {
		all_fasta_file = seqdb.getConcatAllGeneProt()
	} else {
		all_fasta_file = seqdb.getConcatAllGeneNucl()
	}

	// Use samtools to fetch data
	seq_name := geneRequestToSAMrequest(&req)

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

func (seqdb *SequenceDB) GetRegionSequence(req request.RegionGetRequest) ([]byte, error) {

	// Use samtools to fetch data
	// "KCB09|contig000007|KCB09_00064:50-100"
	all_contigs_file := seqdb.getConcatContigNucl()
	seq_name := regionRequestToSAMrequest(&req)

	args := []string{"faidx", all_contigs_file, seq_name}
	cmd := exec.Command("samtools", args...)

	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("%w: Sequence not found", err)
	}

	return output, nil
}

// Retrieves gene sequences using Samtools faidx based on multiple gene requests.
// TODO: Consider preloading Samtools to improve responsiveness.
func (seqdb *SequenceDB) GetMultipleGene(genereqs []*request.GeneGetRequest, is_prot bool) ([]byte, error) {

	var geneInputBuffer bytes.Buffer
	var all_gene_file string

	// Input for samtools ( stdin )
	// The input is multiple lines of sequences id e.g. KCB09|contig000007|KCB09_00064:50-100
	for _, s := range genereqs {
		geneInputBuffer.WriteString(geneRequestToSAMrequest(s))
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

func (seqdb *SequenceDB) GetMultipleRegion(regreqs []*request.RegionGetRequest) ([]byte, error) {

	var contigInputBuffer bytes.Buffer

	// Make buffer for region
	for _, s := range regreqs {
		contigInputBuffer.WriteString(regionRequestToSAMrequest(s))
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

func geneRequestToSAMrequest(req *request.GeneGetRequest) string {
	return fmt.Sprintf("%s|%s|%s", req.Genome_ID, req.Contig_ID, req.Gene_ID)
}

func regionRequestToSAMrequest(req *request.RegionGetRequest) string {
	return fmt.Sprintf("%s|%s:%d-%d", req.Genome_ID, req.Contig_ID, req.Start, req.End)
}

// func (g GeneGetRequest) String() string {
// 	return fmt.Sprintf(
// 		"%s|%s|%s",
// 		g.Genome_ID, g.Contig_ID, g.Gene_ID,
// 	)
// }

// func (r RegionGetRequest) String() string {
// 	return fmt.Sprintf(
// 		"%s|%s:%d-%d",
// 		r.Genome_ID, r.Contig_ID, r.Start, r.End,
// 	)
// }

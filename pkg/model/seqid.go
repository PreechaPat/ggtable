package model

import (
	"fmt"
	"strings"
)

// SeqIDSep separates the parts of a BLAST sequence ID: GENOME//CONTIG//GENE
// for genes and GENOME//CONTIG for contigs. ggprep and ggorigin write the same
// separator when they build the BLAST databases.
const SeqIDSep = "//"

// GeneRef addresses one gene sequence in the BLAST gene databases.
type GeneRef struct {
	GenomeID string
	ContigID string
	GeneID   string
}

// SeqID returns the BLAST sequence ID, GENOME//CONTIG//GENE.
func (r GeneRef) SeqID() string {
	return r.GenomeID + SeqIDSep + r.ContigID + SeqIDSep + r.GeneID
}

// RegionRef addresses a stretch of a contig in the BLAST genome database.
// Start and End are 1-based and inclusive, as blastdbcmd -range expects.
type RegionRef struct {
	GenomeID string
	ContigID string
	Start    uint64
	End      uint64
}

// SeqID returns the BLAST sequence ID of the contig, GENOME//CONTIG.
func (r RegionRef) SeqID() string { return r.GenomeID + SeqIDSep + r.ContigID }

// Range returns the blastdbcmd range, START-END.
func (r RegionRef) Range() string { return fmt.Sprintf("%d-%d", r.Start, r.End) }

// Validate rejects a region blastdbcmd cannot fetch.
func (r RegionRef) Validate() error {
	if r.Start == 0 || r.End < r.Start {
		return fmt.Errorf("region %s:%s is invalid: start must be >= 1 and end >= start", r.SeqID(), r.Range())
	}
	return nil
}

// LabelFasta prefixes each FASTA header's genome ID with the genome's full
// name: ">G1//ctg//g" becomes ">Genome One-G1//ctg//g". Headers of unknown
// genomes and all other lines are left as they are.
func LabelFasta(fasta []byte, genomes GenomeCatalog) string {
	lines := strings.Split(string(fasta), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, ">") {
			continue
		}
		genomeID, rest, found := strings.Cut(strings.TrimPrefix(line, ">"), SeqIDSep)
		if name, ok := genomes.Name(genomeID); ok && found {
			lines[i] = ">" + name + "-" + genomeID + SeqIDSep + rest
		}
	}
	return strings.Join(lines, "\n")
}

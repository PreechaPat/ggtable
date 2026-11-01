package model

import (
	"bufio"
	"errors"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strings"
)

// BlastSearchRequest is a BLAST form submission.
type BlastSearchRequest struct {
	BlastType string `json:"blast_type"` // blastn or blastp
	Sequence  string `json:"sequence"`   // FASTA or bare sequence
}

// Validate checks the program and returns the cleaned query FASTA.
func (r BlastSearchRequest) Validate() (string, error) {
	if r.BlastType != "blastn" && r.BlastType != "blastp" {
		return "", fmt.Errorf("invalid BLAST type %q: use blastn or blastp", r.BlastType)
	}
	return CleanFasta(r.Sequence)
}

// CleanFasta trims the input and drops blank lines.
func CleanFasta(input string) (string, error) {
	var lines []string
	for _, line := range strings.Split(input, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			lines = append(lines, t)
		}
	}
	if len(lines) == 0 {
		return "", errors.New("sequence is empty")
	}
	return strings.Join(lines, "\n"), nil
}

var (
	escSep = regexp.QuoteMeta(SeqIDSep)
	// TOC line: "GENOME//CONTIG//GENE   score  evalue"
	blastTOCRegex = regexp.MustCompile(fmt.Sprintf(`^(\S+)%s(\S+)%s(\S+)`, escSep, escSep))
	// Alignment header, BLAST 2.17 with a -parse_seqids DB:
	//   >GENOME//CONTIG//GENE<a name=GENOME//CONTIG//GENE></a> unnamed protein product
	// Groups: genome, contig, gene, rest (anchor + title, kept so the TOC "#ID" jump still works).
	blastAlignRegex = regexp.MustCompile(fmt.Sprintf(`^>([^\s<]+?)%s([^\s<]+?)%s([^\s<]+?)(<a name=[^>]*></a>.*)$`, escSep, escSep))
	multiSpace      = regexp.MustCompile(`\s{2,}`)
)

type blastParseState int

const (
	stateHeader blastParseState = iota
	stateTOC
	stateAlignments
	stateFooter
)

// AnnotateBlastReport rewrites a BLAST -html report for display: hit IDs get
// the genome's full name, and each alignment gets a link to its cluster.
func AnnotateBlastReport(report io.Reader, genomes GenomeCatalog) (string, error) {
	var out strings.Builder
	reader := bufio.NewReader(report)
	state := stateHeader

	for {
		raw, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("read BLAST report: %w", err)
		}
		if raw == "" && err == io.EOF {
			break
		}
		line := strings.TrimSuffix(raw, "\n")
		trimmed := strings.TrimSpace(line)

		switch state {
		case stateHeader:
			if f := strings.Fields(line); len(f) == 2 && f[0] == "Score" && f[1] == "E" {
				line = fmt.Sprintf("%-90s %-10s %-5s", " ", "Score", "E")
			} else if strings.HasPrefix(line, "Sequences producing significant alignments:") {
				state = stateTOC
				line = fmt.Sprintf("%-90s %-10s %-5s", "Sequences producing significant alignments:", "(Bits)", "Value")
			}
		case stateTOC:
			if strings.HasPrefix(trimmed, ">") {
				state = stateAlignments
				line = annotateAlignmentHeader(line, genomes)
			} else {
				line = annotateTOCLine(line, genomes)
			}
		case stateAlignments:
			if strings.HasPrefix(line, "  Database:") || strings.HasPrefix(trimmed, "Lambda") {
				state = stateFooter
			} else if strings.HasPrefix(trimmed, ">") {
				line = annotateAlignmentHeader(line, genomes)
			}
		case stateFooter:
			// Multi-query input: each query repeats Query=/TOC/alignments/stats, so start over.
			if strings.HasPrefix(line, "<b>Query=</b>") {
				state = stateHeader
			}
		}
		out.WriteString(line + "\n")

		if err == io.EOF {
			break
		}
	}
	return out.String(), nil
}

func annotateTOCLine(line string, genomes GenomeCatalog) string {
	m := blastTOCRegex.FindStringSubmatch(line)
	if len(m) != 4 {
		return line
	}
	name, ok := genomes.Name(m[1])
	if !ok {
		return line
	}
	parts := multiSpace.Split(line, 3)
	if len(parts) < 3 {
		return line
	}
	id := GeneRef{GenomeID: m[1], ContigID: m[2], GeneID: m[3]}.SeqID()
	return fmt.Sprintf("%-90s %-10s %-5s", html.EscapeString(name)+"-"+id, parts[1], parts[2])
}

func annotateAlignmentHeader(line string, genomes GenomeCatalog) string {
	m := blastAlignRegex.FindStringSubmatch(line)
	if len(m) != 5 {
		return line
	}
	name, ok := genomes.Name(m[1])
	if !ok {
		return line
	}
	ref := GeneRef{GenomeID: m[1], ContigID: m[2], GeneID: m[3]}
	link := fmt.Sprintf("/cluster/heatmap/%s/%s/%s",
		url.PathEscape(ref.GenomeID), url.PathEscape(ref.ContigID), url.PathEscape(ref.GeneID))
	return fmt.Sprintf(">%s-%s%s <a href=\"%s\">View in gene table</a>",
		html.EscapeString(name), ref.SeqID(), m[4], link)
}

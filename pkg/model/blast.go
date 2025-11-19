package model

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
)

// cleanFasta validates and cleans the input FASTA string.
func cleanFasta(inputFasta string) (string, error) {
	cleaned := strings.TrimSpace(inputFasta)
	if cleaned == "" {
		return "", errors.New("input FASTA string is empty")
	}

	lines := strings.Split(cleaned, "\n")
	var validLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			validLines = append(validLines, trimmed)
		}
	}

	return strings.Join(validLines, "\n"), nil
}

// runBLASTCommand executes a BLAST command with the given parameters and input FASTA.
func runBLASTCommand(cmdName, db string, inputFasta string) (string, error) {
	cleanedFasta, err := cleanFasta(inputFasta)
	if err != nil {
		return "", fmt.Errorf("failed to clean FASTA: %w", err)
	}

	// Shows 500 alignments, that should be enough. Also, max_seq is already at 500
	cmd := exec.Command(cmdName, "-db", db, "-html", "-num_descriptions", "500", "-num_alignments", "500")
	cmd.Stdin = bytes.NewBufferString(cleanedFasta)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute %s: %w", cmdName, err)
	}

	result, err := parseAndAddLink(&out)
	if err != nil {
		return "", fmt.Errorf("failed to parse BLAST output: %w", err)
	}

	return result.String(), nil
}

// BLASTP runs a BLASTP search and returns the processed output.
func BLASTP(AADB, inputFasta string) (string, error) {
	return runBLASTCommand("blastp", AADB, inputFasta)
}

// BLASTN runs a BLASTN search and returns the processed output.
func BLASTN(NCDB, inputFasta string) (string, error) {
	return runBLASTCommand("blastn", NCDB, inputFasta)
}

// Parse BLAST result and add link to the
func parseAndAddLink(htmlContent *bytes.Buffer) (*bytes.Buffer, error) {
	var output bytes.Buffer

	// Genome id -> Genome name lookup
	genomeid_lookup := MAP_HEADER
	reader := bufio.NewReader(htmlContent)

	// Regex to capture genome, contig, and gene
	front_sequence_regex := regexp.MustCompile(`^(\S+)\|(\S+)\|(\S+)`)  // 36SW|contig000167|P36SW_07281           <a href="http://localhost:8080/blast#BL_ORD_ID:1277053">69.7</a>    2e-11
	sequence_name_regex := regexp.MustCompile(`\s(\S+)\|(\S+)\|(\S+)$`) // &gt;<a name="BL_ORD_ID:951843"></a> P36SW|contig000167|P36SW_07281
	space_detection := regexp.MustCompile(`\s{4,}`)

	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimSuffix(line, "\n")

		if err != nil && err != io.EOF {
			return nil, err
		}

		// Skip the problematic part. (BLASTN has several line with |||| which mess with regex)
		if strings.HasPrefix(line, "           ") {
			output.WriteString(line)
			output.WriteString("\n")
			continue
		}

		// doing match in both front and back kind
		table_matches := front_sequence_regex.FindStringSubmatch(line)
		header_matches := sequence_name_regex.FindStringSubmatch(line)

		// It is a table, I need a padding
		if len(table_matches) == 4 {
			genome_id := table_matches[1]
			contig := table_matches[2]
			gene := table_matches[3]

			genome_name, ok := genomeid_lookup[genome_id]
			if !ok {
				return &output, err
			}
			replacement := fmt.Sprintf("%s-%s|%s|%s", genome_name, genome_id, contig, gene)

			parts := space_detection.Split(line, 3)
			alink := parts[1]
			score := parts[2]

			newline := fmt.Sprintf("%-90s %-10s %-5s", replacement, alink, score)

			output.WriteString(newline)

		} else if len(header_matches) == 4 {
			// this is a header >.... no padding need
			genome_id := header_matches[1]
			contig := header_matches[2]
			gene := header_matches[3]

			genome_name, ok := genomeid_lookup[genome_id]
			if !ok {
				return &output, err
			}

			replacement := fmt.Sprintf("%s-%s|%s|%s", genome_name, genome_id, contig, gene)
			link := fmt.Sprintf("/cluster/heatmap/%s/%s/%s",
				url.PathEscape(genome_id),
				url.PathEscape(contig),
				url.PathEscape(gene),
			)
			link_html := fmt.Sprintf("<a href=\"%s\">View in gene table</a>", link)

			transformedLine := sequence_name_regex.ReplaceAllString(line, replacement)

			output.WriteString(transformedLine)
			output.WriteString(" ")
			output.WriteString(link_html)
		} else if strings.HasPrefix(line, "Sequences producing significant alignments:") {
			// Sequences producing significant alignments:                          (Bits)  Value
			s1 := "Sequences producing significant alignments:"
			s2 := "(Bits)"
			s3 := "Value"
			newline := fmt.Sprintf("%-90s %-10s %-5s", s1, s2, s3)
			output.WriteString(newline)

		} else if strings.HasSuffix(line, "Score     E") {
			// Score     E
			s1 := " "
			s2 := "Score"
			s3 := "E"
			newline := fmt.Sprintf("%-90s %-10s %-5s", s1, s2, s3)
			output.WriteString(newline)

		} else {
			output.WriteString(line)
		}
		output.WriteString("\n")

		if err == io.EOF {
			break
		}
	}

	return &output, nil

}

// parseAndAddLink processes BLAST HTML output and enriches it with genome links.
// func parseAndAddLink(htmlContent *bytes.Buffer) (*bytes.Buffer, error) {
// 	var output bytes.Buffer
// 	reader := bufio.NewReader(htmlContent)

// 	genomeIDLookup := MAP_HEADER // Assuming MAP_HEADER is predefined.
// 	frontRegex := regexp.MustCompile(`^(\S+)\|(\S+)\|(\S+)`)
// 	headerRegex := regexp.MustCompile(`\s(\S+)\|(\S+)\|(\S+)$`)
// 	spaceDetection := regexp.MustCompile(`\s{4,}`)

// 	for {
// 		line, err := reader.ReadString('\n')
// 		line = strings.TrimSpace(line)
// 		if err != nil && err != io.EOF {
// 			return nil, err
// 		}

// 		switch {
// 		case strings.HasPrefix(line, "           "): // Skip problematic lines
// 			output.WriteString(line)

// 		case frontRegex.MatchString(line): // Process table lines
// 			matches := frontRegex.FindStringSubmatch(line)
// 			genomeID, contig, gene := matches[1], matches[2], matches[3]

// 			genomeName, ok := genomeIDLookup[genomeID]
// 			if !ok {
// 				return nil, fmt.Errorf("unknown genome ID: %s", genomeID)
// 			}

// 			replacement := fmt.Sprintf("%s-%s|%s|%s", genomeName, genomeID, contig, gene)
// 			parts := spaceDetection.Split(line, 3)
// 			if len(parts) < 3 {
// 				return nil, errors.New("invalid table line format")
// 			}

// 			newline := fmt.Sprintf("%-90s %-10s %-5s", replacement, parts[1], parts[2])
// 			output.WriteString(newline)

// 		case headerRegex.MatchString(line): // Process header lines
// 			matches := headerRegex.FindStringSubmatch(line)
// 			genomeID, contig, gene := matches[1], matches[2], matches[3]

// 			genomeName, ok := genomeIDLookup[genomeID]
// 			if !ok {
// 				return nil, fmt.Errorf("unknown genome ID: %s", genomeID)
// 			}

// 			replacement := fmt.Sprintf("%s-%s|%s|%s", genomeName, genomeID, contig, gene)
// 			link := fmt.Sprintf("/cluster/heatmap/%s/%s/%s", url.PathEscape(genomeID), url.PathEscape(contig), url.PathEscape(gene))
// 			linkHTML := fmt.Sprintf("<a href=\"%s\">View in gene table</a>", link)

// 			transformedLine := headerRegex.ReplaceAllString(line, replacement)
// 			output.WriteString(fmt.Sprintf("%s %s", transformedLine, linkHTML))

// 		case strings.HasPrefix(line, "Sequences producing significant alignments:"):
// 			output.WriteString(fmt.Sprintf("%-90s %-10s %-5s", line, "(Bits)", "Value"))

// 		case strings.HasSuffix(line, "Score     E"):
// 			output.WriteString(fmt.Sprintf("%-90s %-10s %-5s", " ", "Score", "E"))

// 		default:
// 			output.WriteString(line)
// 		}
// 		output.WriteString("\n")

// 		if err == io.EOF {
// 			break
// 		}
// 	}

// 	return &output, nil
// }

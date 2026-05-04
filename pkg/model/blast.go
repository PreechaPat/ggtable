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

// BlastSearchRequest represents a BLAST request.
type BlastSearchRequest struct {
	BlastType string `json:"blast_type"`
	Sequence  string `json:"sequence"`
}

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

// Define the states for our parser
type ParseState int

const (
	StateHeader ParseState = iota
	StateTOC
	StateAlignments
	StateFooter
)

func parseAndAddLink(htmlContent *bytes.Buffer) (*bytes.Buffer, error) {
	var output bytes.Buffer
	reader := bufio.NewReader(htmlContent)

	// HACK: Assuming MAP_HEADER is available in your scope
	genomeid_lookup := MAP_HEADER

	// Regexes are now more specific and only applied in their respective states
	tocRegex := regexp.MustCompile(`^(\S+)\|(\S+)\|(\S+)`)
	alignHeaderRegex := regexp.MustCompile(`^(>.*?<a.*?></a>\s)(\S+)\|(\S+)\|(\S+)$`)
	spaceDetection := regexp.MustCompile(`\s{2,}`)

	state := StateHeader

	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, err
		}

		// Clean the line for easier processing, but we will write it with a newline later
		cleanLine := strings.TrimSuffix(line, "\n")

		switch state {
		case StateHeader:
			if strings.HasSuffix(cleanLine, "Score     E") {
				// Format the column headers
				newline := fmt.Sprintf("%-90s %-10s %-5s", " ", "Score", "E")
				output.WriteString(newline + "\n")
			} else if strings.HasPrefix(cleanLine, "Sequences producing significant alignments:") {
				// Transition to TOC state
				state = StateTOC
				newline := fmt.Sprintf("%-90s %-10s %-5s", "Sequences producing significant alignments:", "(Bits)", "Value")
				output.WriteString(newline + "\n")
			} else {
				output.WriteString(cleanLine + "\n")
			}

		case StateTOC:
			// If we hit a line starting with ">", we've entered the Alignments section
			if strings.HasPrefix(strings.TrimSpace(cleanLine), ">") {
				state = StateAlignments
				// Re-evaluate this line in the Alignments state
				processAlignmentLine(&output, cleanLine, alignHeaderRegex, genomeid_lookup)
			} else {
				// Process TOC rows
				matches := tocRegex.FindStringSubmatch(cleanLine)
				if len(matches) == 4 {
					genomeID, contig, gene := matches[1], matches[2], matches[3]

					if genomeName, ok := genomeid_lookup[genomeID]; ok {
						replacement := fmt.Sprintf("%s-%s|%s|%s", genomeName, genomeID, contig, gene)
						parts := spaceDetection.Split(cleanLine, 3)

						if len(parts) >= 3 {
							newline := fmt.Sprintf("%-90s %-10s %-5s", replacement, parts[1], parts[2])
							output.WriteString(newline + "\n")
							continue
						}
					}
				}
				// Default line handling if not a matching TOC row
				output.WriteString(cleanLine + "\n")
			}

		case StateAlignments:
			// If we hit the footer stats, transition to Footer
			if strings.HasPrefix(cleanLine, "  Database:") || strings.HasPrefix(strings.TrimSpace(cleanLine), "Lambda") {
				state = StateFooter
				output.WriteString(cleanLine + "\n")
			} else if strings.HasPrefix(strings.TrimSpace(cleanLine), ">") {
				processAlignmentLine(&output, cleanLine, alignHeaderRegex, genomeid_lookup)
			} else {
				// Regular alignment sequences (Query/Sbjct blocks)
				output.WriteString(cleanLine + "\n")
			}

		case StateFooter:
			// Dump the rest of the file without regex evaluation
			output.WriteString(cleanLine + "\n")
		}

		if err == io.EOF {
			break
		}
	}

	return &output, nil
}

// processAlignmentLine handles the replacement and link injection for the detail blocks
func processAlignmentLine(output *bytes.Buffer, line string, regex *regexp.Regexp, lookup map[string]string) {
	matches := regex.FindStringSubmatch(line)
	if len(matches) == 5 {
		prefix := matches[1] // Captures the `><a name=BL_ORD_ID:1165942></a> ` part
		genomeID := matches[2]
		contig := matches[3]
		gene := matches[4]

		if genomeName, ok := lookup[genomeID]; ok {
			replacement := fmt.Sprintf("%s-%s|%s|%s", genomeName, genomeID, contig, gene)
			link := fmt.Sprintf("/cluster/heatmap/%s/%s/%s",
				url.PathEscape(genomeID),
				url.PathEscape(contig),
				url.PathEscape(gene),
			)
			linkHTML := fmt.Sprintf("<a href=\"%s\">View in gene table</a>", link)

			// Reconstruct line: Prefix + Replaced String + Space + Link
			output.WriteString(fmt.Sprintf("%s%s %s\n", prefix, replacement, linkHTML))
			return
		}
	}
	// Fallback if lookup fails or regex doesn't match perfectly
	output.WriteString(line + "\n")
}

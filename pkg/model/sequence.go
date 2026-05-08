// Model for getting genomes, sequences, and genes

package model

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"

	ggdb "github.com/yumyai/ggtable/pkg/db"
)

// Get gene sequence
type GeneGetRequest struct {
	Genome_ID string `json:"genome_id"`
	Contig_ID string `json:"contig_id"`
	Gene_ID   string `json:"gene_id"`
	Is_Prot   bool   `json:"is_prot"`
}

// Get region sequence
type RegionGetRequest struct {
	Genome_ID string `json:"genome_id"`
	Contig_ID string `json:"contig_id"`
	Start     uint64 `json:"start"`
	End       uint64 `json:"end"`
	Is_Prot   bool   `json:"is_prot"`
}

// Get id name map of genome
func GetGenomes(db *sql.DB) (map[string]string, error) {

	ctx := context.TODO()

	qstring := `select genome_id, genome_fullname from genome_info;`

	stm, err := db.PrepareContext(ctx, qstring)
	if err != nil {
		return nil, err
	}
	defer stm.Close()

	// Search term, limit, offset
	rows, err := stm.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[string]string)

	for rows.Next() {

		var id, name string

		if err := rows.Scan(&id, &name); err != nil {
			panic(err)
		}

		results[id] = name
	}

	return results, nil
}

func GetGeneSequence(seqdb *ggdb.SequenceDB, req GeneGetRequest) (string, error) {

	raw_response, err := seqdb.GetGeneSequence(req.Genome_ID, req.Contig_ID, req.Gene_ID, req.Is_Prot)

	if err != nil {
		return "", err
	}

	ret := supplyFastaHeader(raw_response, MAP_HEADER).String()

	return ret, nil
}

func GetRegionSequence(seqdb *ggdb.SequenceDB, req RegionGetRequest) (string, error) {

	raw_response, err := seqdb.GetRegionSequence(req.Genome_ID, req.Contig_ID, req.Start, req.End)

	if err != nil {
		return "", err
	}

	ret := supplyFastaHeader(raw_response, MAP_HEADER).String()

	return ret, nil
}

func GetMultipleGenes(seqdb *ggdb.SequenceDB, req []*GeneGetRequest, is_prot bool) (string, error) {

	geneNames := make([]string, 0, len(req))
	for _, r := range req {
		geneNames = append(geneNames, fmt.Sprintf("%s//%s//%s", r.Genome_ID, r.Contig_ID, r.Gene_ID))
	}

	raw_output, err := seqdb.GetMultipleGene(geneNames, is_prot)

	if err != nil {
		return "", err
	}

	ret := supplyFastaHeader(raw_output, MAP_HEADER).String()

	return ret, nil
}

func GetMultipleRegions(seqdb *ggdb.SequenceDB, req []*RegionGetRequest) (string, error) {

	regionNames := make([]string, 0, len(req))
	for _, r := range req {
		regionNames = append(regionNames, fmt.Sprintf("%s//%s:%d-%d", r.Genome_ID, r.Contig_ID, r.Start, r.End))
	}

	raw_output, err := seqdb.GetMultipleRegion(regionNames)

	if err != nil {
		return "", err
	}

	ret := supplyFastaHeader(raw_output, MAP_HEADER).String()

	return ret, nil
}

func GetClusterID(db *sql.DB, gene_request GeneGetRequest) ([]string, error) {
	ctx := context.TODO()

	const q = `
		SELECT DISTINCT cluster_id
		FROM gene_matches
		WHERE genome_id = ? AND gene_id = ?
	`
	stm, err := db.PrepareContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer stm.Close()

	rows, err := stm.QueryContext(ctx, gene_request.Genome_ID, gene_request.Gene_ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var clusterID string
		if err := rows.Scan(&clusterID); err != nil {
			return nil, err
		}
		results = append(results, clusterID)
	}

	return results, nil
}

// Add name to fasta header
func supplyFastaHeader(input []byte, genomeMap map[string]string) *bytes.Buffer {
	var output bytes.Buffer

	// Split the input into lines
	lines := bytes.Split(input, []byte("\n"))

	for _, line := range lines {
		// Convert []byte to string for processing
		lineStr := string(line)

		if strings.HasPrefix(lineStr, ">") {
			// Process header line
			fields := strings.SplitN(lineStr, "//", 2) // Split the line into genome ID and the rest
			if len(fields) > 0 {
				genomeID := strings.TrimPrefix(fields[0], ">")
				if genomeName, ok := genomeMap[genomeID]; ok {
					// Replace genome ID with genome name-genomeID
					fields[0] = fmt.Sprintf(">%s-%s", genomeName, genomeID)
				}
				// Join fields back together and convert to []byte
				line = []byte(strings.Join(fields, "//"))
			}
		}

		// Write the line to the output (as []byte)
		output.Write(append(line, '\n'))
	}

	return &output
}

// Model for getting genomes, sequences, and genes

package model

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"

	ggdb "github.com/yumyai/ggtable/pkg/db"
	"github.com/yumyai/ggtable/pkg/handler/request"
)

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

func GetGeneSequence(seqdb *ggdb.SequenceDB, req request.GeneGetRequest) (string, error) {

	raw_response, err := seqdb.GetGeneSequence(req)

	if err != nil {
		return "", err
	}

	ret := supplyFastaHeader(raw_response, MAP_HEADER).String()

	return ret, nil
}

func GetRegionSequence(seqdb *ggdb.SequenceDB, req request.RegionGetRequest) (string, error) {

	raw_response, err := seqdb.GetRegionSequence(req)

	if err != nil {
		return "", err
	}

	ret := supplyFastaHeader(raw_response, MAP_HEADER).String()

	return ret, nil
}

func GetMultipleGenes(seqdb *ggdb.SequenceDB, req []*request.GeneGetRequest, is_prot bool) (string, error) {

	raw_output, err := seqdb.GetMultipleGene(req, is_prot)

	if err != nil {
		return "", err
	}

	ret := supplyFastaHeader(raw_output, MAP_HEADER).String()

	return ret, nil
}

func GetMultipleRegions(seqdb *ggdb.SequenceDB, req []*request.RegionGetRequest) (string, error) {

	raw_output, err := seqdb.GetMultipleRegion(req)

	if err != nil {
		return "", err
	}

	ret := supplyFastaHeader(raw_output, MAP_HEADER).String()

	return ret, nil
}

// Add genome name to fasta header
func supplyFastaHeader(input []byte, genomeMap map[string]string) *bytes.Buffer {
	var output bytes.Buffer

	// Split the input into lines
	lines := bytes.Split(input, []byte("\n"))

	for _, line := range lines {
		// Convert []byte to string for processing
		lineStr := string(line)

		if strings.HasPrefix(lineStr, ">") {
			// Process header line
			fields := strings.SplitN(lineStr, "|", 2) // Split the line into genome ID and the rest
			if len(fields) > 0 {
				genomeID := strings.TrimPrefix(fields[0], ">")
				if genomeName, ok := genomeMap[genomeID]; ok {
					// Replace genome ID with genome name-genomeID
					fields[0] = fmt.Sprintf(">%s-%s", genomeName, genomeID)
				}
				// Join fields back together and convert to []byte
				line = []byte(strings.Join(fields, "|"))
			}
		}

		// Write the line to the output (as []byte)
		output.Write(append(line, '\n'))
	}

	return &output
}

// From genome + gene, return all cluster that match.
func GetClusterID(db *sql.DB, gene_request request.GeneGetRequest) ([]string, error) {

	ctx := context.TODO()

	qstring := `select cluster_id from gene_matches where genome_id == ? and gene_id == ?`

	stm, err := db.PrepareContext(ctx, qstring)
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

		var r string
		if err := rows.Scan(&r); err != nil {
			panic(err)
		}

		results = append(results, r)
	}

	return results, nil
}

package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/model"
	"github.com/yumyai/ggtable/pkg/render"

	"github.com/yumyai/ggtable/pkg/handler/request"
)

func (dbctx *DBContext) BlastSearchPage(w http.ResponseWriter, r *http.Request) {

	var req request.BlastSearchRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Error(err.Error())
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.BlastType != "blastn" && req.BlastType != "blastp" {
		http.Error(w, "Invalid BLAST type", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Sequence) == "" {
		http.Error(w, "Sequence cannot be empty", http.StatusBadRequest)
		return
	}

	// TODO:Check if the sequence is compatible with blast type....

	// Process the BLAST search
	result := processBlastSearch(dbctx, req)

	if message, ok := result["message"].(string); ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		render.RenderBLASTPage(w, message, req.BlastType)
	}
}

func processBlastSearch(dbctx *DBContext, req request.BlastSearchRequest) map[string]interface{} {

	var result map[string]interface{}

	switch req.BlastType {
	case "blastn":
		output, _ := model.BLASTN(dbctx.NuclBLAST_DB, req.Sequence)
		result = map[string]interface{}{
			"message": output,
			"status":  "OK",
		}
	case "blastp":
		output, _ := model.BLASTP(dbctx.ProtBLAST_DB, req.Sequence)
		result = map[string]interface{}{
			"message": output,
			"status":  "OK",
		}
	default:
		result = map[string]interface{}{
			"message": "sumthing wrong",
			"status":  "error",
		}
	}

	return result

}

func (dbctx *DBContext) BlastNRedirectPage(w http.ResponseWriter, r *http.Request) {

	// Get result from get get gene sequence and

	genome_id := r.URL.Query().Get("genome_id")
	contig_id := r.URL.Query().Get("contig_id")
	start_loc, err_start := strconv.ParseUint(r.URL.Query().Get("start"), 10, 64)
	end_loc, err_end := strconv.ParseUint(r.URL.Query().Get("end"), 10, 64)

	// Check if genome_id or contig_id are missing
	if genome_id == "" || contig_id == "" {
		http.Error(w, "Missing genome_id or contig_id", http.StatusBadRequest)
		return
	}

	if err_start != nil || err_end != nil {
		http.Error(w, "Invalid start or end location", http.StatusBadRequest)
		return
	}

	// Create region request
	req := request.RegionGetRequest{
		Genome_ID: genome_id,
		Contig_ID: contig_id,
		Start:     start_loc,
		End:       end_loc,
		Is_Prot:   false,
	}

	seq, errq := model.GetRegionSequence(dbctx.Sequence_DB, req)

	if errq != nil {
		logger.Error("ERR")
	}

	baseURL := "https://blast.ncbi.nlm.nih.gov/Blast.cgi"
	params := url.Values{}
	params.Add("PROGRAM", "blastn")
	params.Add("PAGE_TYPE", "BlastSearch")
	params.Add("QUERY", seq)
	blastURL := baseURL + "?" + params.Encode()

	// Redirect the user to the BLAST URL
	http.Redirect(w, r, blastURL, http.StatusFound)

}

func (dbctx *DBContext) BlastPRedirectPage(w http.ResponseWriter, r *http.Request) {

	genome_id := r.URL.Query().Get("genome_id")
	contig_id := r.URL.Query().Get("contig_id")
	gene_id := r.URL.Query().Get("gene_id")

	// Check if genome_id or contig_id are missing
	if genome_id == "" || contig_id == "" || gene_id == "" {
		http.Error(w, "Missing genome_id or contig_id or gene_id", http.StatusBadRequest)
		return
	}

	req := request.GeneGetRequest{
		Genome_ID: genome_id,
		Contig_ID: contig_id,
		Gene_ID:   gene_id,
		Is_Prot:   true,
	}

	seq, errq := model.GetGeneSequence(dbctx.Sequence_DB, req)

	if errq != nil {
		logger.Error("ERR")
	}

	baseURL := "https://blast.ncbi.nlm.nih.gov/Blast.cgi"
	params := url.Values{}
	params.Add("PROGRAM", "blastp")
	params.Add("PAGE_TYPE", "BlastSearch")
	params.Add("QUERY", seq)
	blastURL := baseURL + "?" + params.Encode()

	// Redirect the user to the BLAST URL
	http.Redirect(w, r, blastURL, http.StatusFound)

}

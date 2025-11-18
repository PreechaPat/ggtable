package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/model"
	"github.com/yumyai/ggtable/pkg/render"
	"go.uber.org/zap"

	"github.com/yumyai/ggtable/pkg/handler/request"
)

func (dbctx *DBContext) BlastSearchPage(w http.ResponseWriter, r *http.Request) {
	if dbctx.BlastJobs == nil {
		http.Error(w, "BLAST service unavailable", http.StatusInternalServerError)
		return
	}

	var req request.BlastSearchRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Error("invalid BLAST request", zap.Error(err))
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

	job := dbctx.BlastJobs.NewJob(req.BlastType)
	go dbctx.runBlastJob(job.ID, req)

	if prefersJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"job_id": job.ID,
		}); err != nil {
			logger.Error("failed to encode BLAST response", zap.String("job_id", job.ID), zap.Error(err))
		}
		return
	}

	http.Redirect(w, r, "/blast/"+job.ID, http.StatusSeeOther)
}

func (dbctx *DBContext) runBlastJob(jobID string, req request.BlastSearchRequest) {
	dbctx.BlastJobs.SetRunning(jobID)

	var (
		output string
		err    error
	)

	switch req.BlastType {
	case "blastn":
		output, err = model.BLASTN(dbctx.NuclBLAST_DB, req.Sequence)
	case "blastp":
		output, err = model.BLASTP(dbctx.ProtBLAST_DB, req.Sequence)
	default:
		err = errors.New("unsupported BLAST type")
	}

	if err != nil {
		logger.Error("BLAST job failed", zap.String("job_id", jobID), zap.Error(err))
		dbctx.BlastJobs.FailJob(jobID, err)
		return
	}

	dbctx.BlastJobs.CompleteJob(jobID, output)
}

func (dbctx *DBContext) BlastStatusPage(w http.ResponseWriter, r *http.Request) {
	if dbctx.BlastJobs == nil {
		http.Error(w, "BLAST service unavailable", http.StatusInternalServerError)
		return
	}

	jobID := r.PathValue("job_id")
	if strings.TrimSpace(jobID) == "" {
		http.Error(w, "Missing job ID", http.StatusBadRequest)
		return
	}

	job, ok := dbctx.BlastJobs.GetJob(jobID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := render.BlastPageData{
		JobID:                  job.ID,
		BlastType:              job.BlastType,
		BlastReport:            job.Result,
		Status:                 string(job.Status),
		ErrorMessage:           job.Error,
		ShouldRefresh:          job.Status == BlastJobQueued || job.Status == BlastJobRunning,
		RefreshIntervalSeconds: 5,
	}

	if err := render.RenderBLASTPage(w, data); err != nil {
		logger.Error("failed to render BLAST page", zap.String("job_id", jobID), zap.Error(err))
	}
}

func prefersJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Requested-With"), "XMLHttpRequest")
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

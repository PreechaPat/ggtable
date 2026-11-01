package handler

import (
	"bytes"
	"context"
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
)

func (appConfig *AppContext) BlastSearchPage(w http.ResponseWriter, r *http.Request) {
	if appConfig.BlastManager == nil {
		http.Error(w, "BLAST service unavailable", http.StatusInternalServerError)
		return
	}

	var req model.BlastSearchRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Error("invalid BLAST request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	query, err := req.Validate()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	job := appConfig.BlastManager.NewJob(req.BlastType)
	go appConfig.runBlastJob(job.ID, req.BlastType, query)

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

// runBlastJob runs in its own goroutine, after the request has ended, so it
// does not use the request context.
func (appConfig *AppContext) runBlastJob(jobID, program, query string) {
	appConfig.BlastManager.SetRunning(jobID)

	report, err := appConfig.Seqs.Search(context.Background(), program, query)
	if err != nil {
		logger.Error("BLAST job failed", zap.String("job_id", jobID), zap.Error(err))
		appConfig.BlastManager.FailJob(jobID, errors.New("BLAST failed; check the query is a valid "+queryKind(program)+" sequence"))
		return
	}
	output, err := model.AnnotateBlastReport(bytes.NewReader(report), appConfig.Genomes)
	if err != nil {
		logger.Error("BLAST report rewrite failed", zap.String("job_id", jobID), zap.Error(err))
		appConfig.BlastManager.FailJob(jobID, errors.New("could not read the BLAST report"))
		return
	}
	appConfig.BlastManager.CompleteJob(jobID, output)
}

func queryKind(program string) string {
	if program == "blastp" {
		return "protein"
	}
	return "nucleotide"
}

func (appConfig *AppContext) BlastStatusPage(w http.ResponseWriter, r *http.Request) {
	if appConfig.BlastManager == nil {
		http.Error(w, "BLAST service unavailable", http.StatusInternalServerError)
		return
	}

	jobID := r.PathValue("job_id")
	if strings.TrimSpace(jobID) == "" {
		http.Error(w, "Missing job ID", http.StatusBadRequest)
		return
	}

	job, ok := appConfig.BlastManager.GetJob(jobID)
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
		ShouldRefresh:          job.Status == model.BlastJobQueued || job.Status == model.BlastJobRunning,
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

func (appConfig *AppContext) BlastNRedirectPage(w http.ResponseWriter, r *http.Request) {

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

	ref := model.RegionRef{GenomeID: genome_id, ContigID: contig_id, Start: start_loc, End: end_loc}
	if err := ref.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fasta, err := appConfig.Seqs.Region(r.Context(), ref)
	if err != nil {
		logger.Warn("region sequence not found", zap.String("seq_id", ref.SeqID()), zap.String("range", ref.Range()), zap.Error(err))
		http.Error(w, "Region sequence not found", http.StatusNotFound)
		return
	}
	seq := model.LabelFasta(fasta, appConfig.Genomes)

	baseURL := "https://blast.ncbi.nlm.nih.gov/Blast.cgi"
	params := url.Values{}
	params.Add("PROGRAM", "blastn")
	params.Add("PAGE_TYPE", "BlastSearch")
	params.Add("QUERY", seq)
	blastURL := baseURL + "?" + params.Encode()

	// Redirect the user to the BLAST URL
	http.Redirect(w, r, blastURL, http.StatusFound)

}

func (appConfig *AppContext) BlastPRedirectPage(w http.ResponseWriter, r *http.Request) {

	genome_id := r.URL.Query().Get("genome_id")
	contig_id := r.URL.Query().Get("contig_id")
	gene_id := r.URL.Query().Get("gene_id")

	// Check if genome_id or contig_id are missing
	if genome_id == "" || contig_id == "" || gene_id == "" {
		http.Error(w, "Missing genome_id or contig_id or gene_id", http.StatusBadRequest)
		return
	}

	ref := model.GeneRef{GenomeID: genome_id, ContigID: contig_id, GeneID: gene_id}
	fasta, err := appConfig.Seqs.Gene(r.Context(), ref, true)
	if err != nil {
		logger.Warn("gene sequence not found", zap.String("seq_id", ref.SeqID()), zap.Error(err))
		http.Error(w, "Gene sequence not found", http.StatusNotFound)
		return
	}
	seq := model.LabelFasta(fasta, appConfig.Genomes)

	baseURL := "https://blast.ncbi.nlm.nih.gov/Blast.cgi"
	params := url.Values{}
	params.Add("PROGRAM", "blastp")
	params.Add("PAGE_TYPE", "BlastSearch")
	params.Add("QUERY", seq)
	blastURL := baseURL + "?" + params.Encode()

	// Redirect the user to the BLAST URL
	http.Redirect(w, r, blastURL, http.StatusFound)

}

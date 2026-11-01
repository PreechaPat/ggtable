package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/model"
	"go.uber.org/zap"
)

// SequenceReponse is the JSON error body of /sequence/by-cluster.
type SequenceReponse struct {
	Status          string `json:"success"`
	SequencePayload string `json:"payload"`
	Error           string `json:"error"`
}

func (appConfig *AppContext) GetGeneSequenceHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	isProt, err := strconv.ParseBool(q.Get("is_prot"))
	if err != nil {
		http.Error(w, "is_prot need to be bool-like string", http.StatusBadRequest)
		return
	}
	ref := model.GeneRef{GenomeID: q.Get("genome_id"), ContigID: q.Get("contig_id"), GeneID: q.Get("gene_id")}

	fasta, err := appConfig.Seqs.Gene(r.Context(), ref, isProt)
	if err != nil {
		logger.Warn("gene sequence not found", zap.String("seq_id", ref.SeqID()), zap.Error(err))
		http.Error(w, "Not found (maybe sequence database isn't available?)", http.StatusBadRequest)
		return
	}
	fmt.Fprint(w, model.LabelFasta(fasta, appConfig.Genomes))
}

func (appConfig *AppContext) GetRegionSequenceHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	start, errStart := strconv.ParseUint(q.Get("start"), 10, 64)
	end, errEnd := strconv.ParseUint(q.Get("end"), 10, 64)

	var errorMessages []string
	if errStart != nil {
		errorMessages = append(errorMessages, "Invalid start value")
	}
	if errEnd != nil {
		errorMessages = append(errorMessages, "Invalid end value")
	}
	if len(errorMessages) > 0 {
		http.Error(w, strings.Join(errorMessages, "; "), http.StatusBadRequest)
		return
	}
	ref := model.RegionRef{GenomeID: q.Get("genome_id"), ContigID: q.Get("contig_id"), Start: start, End: end}
	if err := ref.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fasta, err := appConfig.Seqs.Region(r.Context(), ref)
	if err != nil {
		logger.Warn("region sequence not found", zap.String("seq_id", ref.SeqID()), zap.String("range", ref.Range()), zap.Error(err))
		http.Error(w, "Region not found (maybe sequence database isn't available?)", http.StatusBadRequest)
		return
	}
	fmt.Fprint(w, model.LabelFasta(fasta, appConfig.Genomes))
}

// GetSequenceByClusterIDHandler returns the FASTA of every gene in a cluster,
// plus, for nucleotides, every region.
func (appConfig *AppContext) GetSequenceByClusterIDHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clusterID := q.Get("cluster_id")
	isProt, err := strconv.ParseBool(q.Get("is_prot"))
	if err != nil {
		http.Error(w, "is_prot need to be bool-like string", http.StatusBadRequest)
		return
	}

	cluster, err := appConfig.Clusters.Cluster(r.Context(), clusterID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			writeSequenceError(w, fmt.Sprintf("cluster %q not found", clusterID))
			return
		}
		logger.Error("failed to load cluster", zap.String("cluster_id", clusterID), zap.Error(err))
		writeSequenceError(w, "failed to load cluster")
		return
	}

	genes, err := appConfig.Seqs.Genes(r.Context(), cluster.GeneRefs(), isProt)
	if err != nil {
		logger.Error("failed to fetch cluster genes", zap.String("cluster_id", clusterID), zap.Error(err))
		writeSequenceError(w, "failed to fetch gene sequences")
		return
	}
	var regions []byte
	if !isProt {
		regions, err = appConfig.Seqs.Regions(r.Context(), cluster.RegionRefs())
		if err != nil {
			logger.Error("failed to fetch cluster regions", zap.String("cluster_id", clusterID), zap.Error(err))
			writeSequenceError(w, "failed to fetch region sequences")
			return
		}
	}

	fmt.Fprint(w, model.LabelFasta(genes, appConfig.Genomes))
	fmt.Fprint(w, model.LabelFasta(regions, appConfig.Genomes))
}

// writeSequenceError sends msg as JSON; details stay in the server log.
func writeSequenceError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if encErr := json.NewEncoder(w).Encode(SequenceReponse{Status: "error", Error: msg}); encErr != nil {
		logger.Error("failed to encode sequence error", zap.Error(encErr))
	}
}

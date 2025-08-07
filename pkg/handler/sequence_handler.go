package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/yumyai/ggtable/pkg/handler/types"
	"github.com/yumyai/ggtable/pkg/model"
)

// Response struct to hold the payload and page number
type SequencePayload struct {
	Sequence interface{} `json:"sequence"`
}

type SequenceReponse struct {
	Status          string `json:"success"`
	SequencePayload string `json:"payload"`
	Error           string `json:"error"`
}

func (dbctx *DBContext) GetGeneSequenceHandler(w http.ResponseWriter, r *http.Request) {

	genome_id := r.URL.Query().Get("genome_id")
	contig_id := r.URL.Query().Get("contig_id")
	gene_id := r.URL.Query().Get("gene_id")
	is_prot_str := r.URL.Query().Get("is_prot")
	is_prot, err := strconv.ParseBool(is_prot_str)

	if err != nil {
		http.Error(w, "is_prot need to be bool-like string", http.StatusBadRequest)
	}

	genome_gene_param := types.GeneRequest{
		Genome_ID: genome_id,
		Contig_ID: contig_id,
		Gene_ID:   gene_id,
		Is_Prot:   is_prot,
	}

	respons, err := model.GetGeneSequence(dbctx.Sequence_DB, genome_gene_param)

	if err != nil {
		http.Error(w, "Not found (maybe Samtools isn't available?)", http.StatusBadRequest)

	} else {
		fmt.Fprint(w, respons)
	}
}

func (dbctx *DBContext) GetRegionSequenceHandler(w http.ResponseWriter, r *http.Request) {

	genome_id := r.URL.Query().Get("genome_id")
	contig_id := r.URL.Query().Get("contig_id")
	start, err_start := strconv.ParseUint(r.URL.Query().Get("start"), 10, 64)
	end, err_end := strconv.ParseUint(r.URL.Query().Get("end"), 10, 64)

	var errorMessages []string

	if err_start != nil {
		errorMessages = append(errorMessages, "Invalid start value")
	}
	if err_end != nil {
		errorMessages = append(errorMessages, "Invalid end value")
	}

	if len(errorMessages) > 0 {
		// Join all error messages into a single response
		http.Error(w, strings.Join(errorMessages, "; "), http.StatusBadRequest)
		return
	}

	region_gene_param := types.RegionRequest{
		Genome_ID: genome_id,
		Contig_ID: contig_id,
		Start:     start,
		End:       end,
		Is_Prot:   false,
	}

	respons, err := model.GetRegionSequence(dbctx.Sequence_DB, region_gene_param)

	if err != nil {
		http.Error(w, "No", http.StatusBadRequest)

	} else {
		fmt.Fprint(w, respons)
	}
}

func (dbctx *DBContext) GetSequenceByClusterIDHandler(w http.ResponseWriter, r *http.Request) {

	cluster_id := r.URL.Query().Get("cluster_id")
	is_prot_str := r.URL.Query().Get("is_prot")
	is_prot, errgenome := strconv.ParseBool(is_prot_str)

	if errgenome != nil {
		http.Error(w, "is_prot need to be bool-like string", http.StatusBadRequest)
	}

	// Get cluster info
	cluster_info, errgenome := model.GetCluster(dbctx.DB, cluster_id)

	if errgenome != nil {
		// Build error string
		response := SequenceReponse{
			Status:          "error",
			SequencePayload: "",
			Error:           errgenome.Error(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Build request
	gene_request := make([]*types.GeneRequest, 0, 20)
	// is_prot doesn't use here.
	for _, genome := range cluster_info.Genomes {
		for _, gene := range genome.Genes {
			req := &types.GeneRequest{
				Genome_ID: gene.Region.GenomeID,
				Contig_ID: gene.Region.ContigID,
				Gene_ID:   gene.GeneID,
				Is_Prot:   is_prot,
			}
			gene_request = append(gene_request, req)
		}
	}

	gene, gene_err := model.GetMultipleGenes(dbctx.Sequence_DB, gene_request, is_prot)

	var region string
	var region_err error

	// Only call this if it is not protein
	if !is_prot {
		region_request := make([]*types.RegionRequest, 0, 50)

		// is_prot doesn't get used here.
		for _, genome := range cluster_info.Genomes {
			for _, region := range genome.Regions {
				// TODO: Some region doesn't have define location, filter that out
				req := &types.RegionRequest{
					Genome_ID: region.GenomeID,
					Contig_ID: region.ContigID,
					Start:     uint64(region.Start),
					End:       uint64(region.End),
					Is_Prot:   is_prot,
				}
				region_request = append(region_request, req)
			}
		}

		region, region_err = model.GetMultipleRegions(dbctx.Sequence_DB, region_request)
	}

	// Handle error
	if gene_err != nil || region_err != nil {

		var erroMessg string
		erroMessg += ""

		if gene_err != nil {
			erroMessg += gene_err.Error()
		}
		if region_err != nil {
			erroMessg += region_err.Error()
		}

		response := SequenceReponse{
			Status:          "error",
			SequencePayload: "",
			Error:           erroMessg,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Search sequence
	fmt.Fprint(w, gene)

	if !is_prot {
		fmt.Fprint(w, region)
	}
}

package handler

import (
	"fmt"
	"net/http"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/handler/request"
	"github.com/yumyai/ggtable/pkg/model"
	"github.com/yumyai/ggtable/pkg/render"
	"go.uber.org/zap"
)

// ClusterHeatmapPage renders the search-style heatmap for a single cluster resolved via genome/contig/gene path params.
func (dbctx *DBContext) ClusterHeatmapPage(w http.ResponseWriter, r *http.Request) {

	genome := r.PathValue("genome_id")
	contig := r.PathValue("contig_id")
	gene := r.PathValue("gene_id")

	if genome == "" || gene == "" {
		fmt.Fprint(w, "ERROR")
		return
	}

	genome_gene_param := request.GeneGetRequest{
		Genome_ID: genome,
		Contig_ID: contig,
		Gene_ID:   gene,
	}

	logger.Debug("Searching for",
		zap.String("genome", genome),
		zap.String("contig", contig),
		zap.String("gene", gene),
	)

	cluster_ids, err := model.GetClusterID(dbctx.DB, genome_gene_param)

	// Check for all possible errors
	if err != nil {
		// Cluster not found
		fmt.Fprint(w, "ERROR")
		return
	} else if len(cluster_ids) != 1 {
		fmt.Fprint(w, "ERROR")
		return
	}

	cluster_prob, err2 := model.GetCluster(dbctx.DB, cluster_ids[0])

	// Should be possible, massive error
	if err2 != nil {
		fmt.Fprint(w, "ERROR")
		return
	}

	// Search request is used for rendering only, no query involve here.
	// Allow optional color mode from query with canonicalization
	colorByRaw := r.URL.Query().Get("color_by")
	switch colorByRaw {
	case "gene_copy_number", "copy_number", "copies", "copy", "":
		colorByRaw = "gene_copy_number"
	case "max_gene_completeness", "max_completeness", "completeness":
		colorByRaw = "max_gene_completeness"
	default:
		colorByRaw = "gene_copy_number"
	}
	colorBy := colorByRaw

	var search_request = request.ClusterSearchRequest{
		Search_For:   "",
		Search_Field: request.NewClusterField(""),
		Order_Dir:    defaultOrderDir,
		Page:         1,
		Page_Size:    1,
		Genome_IDs:   model.ALL_GENOME_ID,
		Color_By:     colorBy,
		// RequireGenesFromGenomes: reqGeneFromGenome,
	}

	err3 := render.RenderClusterHeatmapPage(w, []*model.Cluster{cluster_prob}, search_request, 1)

	if err3 != nil {
		fmt.Fprint(w, "ERROR")
		return
	}
}

// ClusterDetailPage renders the dedicated table view for a cluster addressed by ID.
func (dbctx *DBContext) ClusterDetailPage(w http.ResponseWriter, r *http.Request) {

	cluster_id := r.PathValue("cluster_id")

	res, err_query := model.GetCluster(dbctx.DB, cluster_id)

	if err_query != nil {
		panic(err_query)
	}

	err := render.RenderClusterTablePage(w, res)

	if err != nil {
		panic(err)
	}
}

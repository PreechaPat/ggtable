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

// Get cluster by genome + gene ID
func (dbctx *DBContext) GetClusterByGeneHandler(w http.ResponseWriter, r *http.Request) {

	genome := r.URL.Query().Get("genome_id")
	gene := r.URL.Query().Get("gene_id")

    genome_gene_param := request.GeneGetRequest{
        Genome_ID: genome,
        Gene_ID:   gene,
    }

	logger.Debug("Searching for", zap.String("genome", genome), zap.String("gene", gene))

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
        Page:         1,
        Page_Size:    1,
        Genome_IDs:   model.ALL_GENOME_ID,
        Color_By:     colorBy,
        // RequireGenesFromGenomes: reqGeneFromGenome,
    }

	err3 := render.RenderClustersAsTable(w, []*model.Cluster{cluster_prob}, search_request, 1)

	if err3 != nil {
		fmt.Fprint(w, "ERROR")
		return
	}
}

func (dbctx *DBContext) ClusterPage(w http.ResponseWriter, r *http.Request) {

	cluster_id := r.PathValue("cluster_id")

	res, err_query := model.GetCluster(dbctx.DB, cluster_id)

	if err_query != nil {
		panic(err_query)
	}

	err := render.RenderClusterPage(w, res)

	if err != nil {
		panic(err)
	}
}

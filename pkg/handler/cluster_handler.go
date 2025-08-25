package handler

import (
	"fmt"
	"net/http"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/handler/types"
	"github.com/yumyai/ggtable/pkg/model"
	"go.uber.org/zap"
)

func (dbctx *DBContext) GetClusterByGeneHandler(w http.ResponseWriter, r *http.Request) {

	genome := r.URL.Query().Get("genome_id")
	gene := r.URL.Query().Get("gene_id")

	genome_gene_param := types.GeneGetRequest{
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

	// RenderingPage
	err3 := model.RenderClustersAsTable(w, []*model.Cluster{cluster_prob}, model.ALL_GENOME_ID, 1, 1, 1)

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

	err := model.RenderClusterPage(w, res)

	if err != nil {
		panic(err)
	}
}

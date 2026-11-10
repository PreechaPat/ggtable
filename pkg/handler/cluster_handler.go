package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/model"
	"github.com/yumyai/ggtable/pkg/render"
	"go.uber.org/zap"
)

// ClusterHeatmapPage renders the search-style heatmap for a single cluster resolved via genome/contig/gene path params.
func (appConfig *AppContext) ClusterHeatmapPage(w http.ResponseWriter, r *http.Request) {

	genome := r.PathValue("genome_id")
	contig := r.PathValue("contig_id")
	gene := r.PathValue("gene_id")

	if genome == "" || gene == "" {
		fmt.Fprint(w, "ERROR")
		return
	}

	logger.Debug("Searching for",
		zap.String("genome", genome),
		zap.String("contig", contig),
		zap.String("gene", gene),
	)

	cluster_ids, err := appConfig.Clusters.ClusterIDsForGene(r.Context(), genome, gene)
	if err != nil {
		logger.Error("failed to find cluster of gene", zap.String("genome", genome), zap.String("gene", gene), zap.Error(err))
		fmt.Fprint(w, "ERROR")
		return
	} else if len(cluster_ids) != 1 {
		fmt.Fprint(w, "ERROR")
		return
	}

	cluster_prob, err2 := appConfig.Clusters.Cluster(r.Context(), cluster_ids[0])
	if err2 != nil {
		logger.Error("failed to load cluster", zap.String("cluster_id", cluster_ids[0]), zap.Error(err2))
		fmt.Fprint(w, "ERROR")
		return
	}

	// Search request is used for rendering only, no query involve here.
	// Allow optional color mode from query with canonicalization
	colorBy := parseColorBy(r.URL.Query().Get("color_by"))

	// Allow only selected genome IDs (defaults to all)
	includeGenome := []string{}
	for key := range r.URL.Query() {
		if strings.HasPrefix(key, "gm_") {
			includeGenome = append(includeGenome, strings.TrimPrefix(key, "gm_"))
		}
	}
	if len(includeGenome) == 0 {
		includeGenome = appConfig.Genomes.IDs()
	}

	var search_request = model.ClusterSearchRequest{
		Search_For:   "",
		Search_Field: model.ParseClusterField(""),
		Order_Dir:    defaultOrderDir,
		Page:         1,
		Page_Size:    1,
		Genome_IDs:   includeGenome,
		Color_By:     colorBy,
		// RequireGenesFromGenomes: reqGeneFromGenome,
	}

	err3 := render.RenderClusterStandaloneHeatmapPage(w, appConfig.Site, appConfig.Genomes, []*model.Cluster{cluster_prob}, search_request, 1)

	if err3 != nil {
		fmt.Fprint(w, "ERROR")
		return
	}
}

// ClusterDetailPage renders the dedicated table view for a cluster addressed by ID.
func (appConfig *AppContext) ClusterDetailPage(w http.ResponseWriter, r *http.Request) {

	cluster_id := r.PathValue("cluster_id")

	res, err := appConfig.Clusters.Cluster(r.Context(), cluster_id)
	if errors.Is(err, model.ErrNotFound) {
		http.Error(w, fmt.Sprintf("Cluster %q not found", cluster_id), http.StatusNotFound)
		return
	}
	if err != nil {
		logger.Error("failed to load cluster", zap.String("cluster_id", cluster_id), zap.Error(err))
		http.Error(w, "Failed to load cluster", http.StatusInternalServerError)
		return
	}

	// The template streams straight to w, so a status can't be changed once it
	// has started writing; log the failure instead.
	if err := render.RenderClusterTablePage(w, appConfig.Genomes, res); err != nil {
		logger.Error("failed to render cluster table", zap.String("cluster_id", cluster_id), zap.Error(err))
	}
}

package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/data/sqldb"
	"github.com/yumyai/ggtable/pkg/model"
	"github.com/yumyai/ggtable/pkg/render"
	"go.uber.org/zap"
)

// TODO:
// Doing a full json validations
// https://www.alexedwards.net/blog/how-to-properly-parse-a-json-request-body
// Organize the DB connection
// https://www.alexedwards.net/blog/organising-database-access

const (
	defaultPageSize   = 100
	defaultPageNumber = 1
	defaultOrderBy    = "cluster_id"
	defaultOrderDir   = "asc"
)

// Response struct to hold the payload and page number
type ClustersPayload struct {
	Cluster   interface{} `json:"clusters"`
	TotalPage int         `json:"pageNumber"`
}

type ClusterResponse struct {
	Success bool
	Payload ClustersPayload `json:"payload"`
	Error   bool
}

func parsePositiveIntFallback(v string, fallback int) int {
	num, err := strconv.Atoi(v)
	if err != nil || num <= 0 {
		return fallback
	}
	return num
}

func normalizeOrderDir(raw string) string {
	switch strings.ToLower(raw) {
	case "desc":
		return "desc"
	default:
		return defaultOrderDir
	}
}

// parseSort reads order_by and order_dir. A missing or unsortable field
// (e.g. gene_id) falls back to cluster ID, so a bad link still shows a page.
func parseSort(q url.Values) (model.ClusterField, string) {
	field := model.ParseClusterField(q.Get("order_by"))
	switch field {
	case model.ClusterFieldFunction, model.ClusterFieldCOGID, model.ClusterFieldClusterID:
	default:
		field = model.ParseClusterField(defaultOrderBy)
	}
	return field, normalizeOrderDir(q.Get("order_dir"))
}

// Search page
func (appConfig *AppContext) ClusterSearchPage(w http.ResponseWriter, r *http.Request) {

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	searchTerm := r.URL.Query().Get("search")
	searchBy := r.URL.Query().Get("search_by")
	searchByF := model.ParseClusterField(searchBy)

	orderByF, orderDir := parseSort(r.URL.Query())

	currentPage := parsePositiveIntFallback(r.URL.Query().Get("page"), defaultPageNumber)
	pageSize := parsePositiveIntFallback(r.URL.Query().Get("page_size"), defaultPageSize)

	// Color selection canonicalization
	// Accepted: gene_copy_number | max_gene_completeness
	// Backward-compat: copy -> gene_copy_number, completeness -> max_gene_completeness
	colorByRaw := r.URL.Query().Get("color_by")
	switch colorByRaw {
	case "gene_copy_number", "copy_number", "copies", "copy", "":
		// default to gene_copy_number if empty
		colorByRaw = "gene_copy_number"
	case "max_gene_completeness", "max_completeness", "completeness":
		colorByRaw = "max_gene_completeness"
	default:
		colorByRaw = "gene_copy_number"
	}
	colorBy := colorByRaw

	// Include the following genome only
	var includeGenome []string
	for key := range r.URL.Query() {
		if strings.HasPrefix(key, "gm_") {
			// Strip "gn_" prefix and append to the array
			strippedKey := strings.TrimPrefix(key, "gm_")
			includeGenome = append(includeGenome, strippedKey)
		}
	}

	// // Only include those cluster with following genes
	// var reqGeneFromGenome []string
	// // Iterate over query parameters
	// for key := range r.URL.Query() {
	// 	if strings.HasPrefix(key, "gn_") {
	// 		// Strip "gn_" prefix and append to the array
	// 		strippedKey := strings.TrimPrefix(key, "gn_")
	// 		reqGeneFromGenome = append(reqGeneFromGenome, strippedKey)
	// 	}
	// }

	logger.Info("Running searchpage",
		zap.String("searchterm", searchTerm),
		zap.String("url", r.URL.Path),
		zap.Int("Page", currentPage),
		zap.Int("Pagesize", pageSize),
		zap.String("order_by", orderByF.String()),
		zap.String("order_dir", orderDir),
		zap.String("color_by", colorBy),
	)

	var search_request = model.ClusterSearchRequest{
		Search_For:   searchTerm,
		Search_Field: searchByF,
		Order_By:     orderByF,
		Order_Dir:    orderDir,
		Page:         currentPage,
		Page_Size:    pageSize,
		Genome_IDs:   includeGenome,
		Color_By:     colorBy,
		// RequireGenesFromGenomes: reqGeneFromGenome,
	}

	// A failed search still renders the page (with no rows), so the user can
	// fix the query; only database failures are logged as errors.
	rows, err := appConfig.Clusters.Search(r.Context(), search_request)
	rowNum := 0
	if err == nil {
		rowNum, err = appConfig.Clusters.CountSearch(r.Context(), search_request)
	}
	if errors.Is(err, sqldb.ErrInvalidSearch) {
		logger.Debug("invalid search", zap.Error(err))
		rows = nil
	} else if err != nil {
		logger.Error("search failed", zap.Any("search_request", search_request), zap.Error(err))
		rows = nil
	}

	totalPageNum := (rowNum + pageSize - 1) / pageSize // Rounding up

	err = render.RenderClusterHeatmapPage(w, appConfig.Site, appConfig.Genomes, rows, search_request, totalPageNum)

	if err != nil {
		logger.Error(err.Error())
		http.Error(w, "Failed to render table", http.StatusInternalServerError)
	}
}

// Main page.
func (appConfig *AppContext) MainPage(w http.ResponseWriter, r *http.Request) {

	const PAGE_SIZE = defaultPageSize

	pageNum := parsePositiveIntFallback(r.URL.Query().Get("page"), defaultPageNumber)

	orderByF, orderDir := parseSort(r.URL.Query())

	// Color selection canonicalization
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

	logger.Info("Running mainpage",
		zap.String("url", r.URL.Path),
		zap.Int("page", pageNum),
		zap.Int("page_size", PAGE_SIZE),
		zap.String("order_by", orderByF.String()),
		zap.String("order_dir", orderDir),
		zap.String("color_by", colorBy),
	)

	var search_request = model.ClusterSearchRequest{
		Search_For:   "",
		Search_Field: model.ClusterFieldFunction,
		Order_By:     orderByF,
		Order_Dir:    orderDir,
		Page:         pageNum,
		Page_Size:    PAGE_SIZE,
		Genome_IDs:   appConfig.Genomes.IDs(), // Default to all genomes
		Color_By:     colorBy,
	}

	rows, err := appConfig.Clusters.MainPage(r.Context(), search_request)
	if err != nil {
		logger.Error("Failed to get main page data from model",
			zap.String("url", r.URL.Path),
			zap.Any("search_request", search_request), // Log the request details
			zap.Error(err))                            // Log the error
		http.Error(w, "Failed to retrieve data", http.StatusInternalServerError)
		return // Important: stop execution after sending error
	}

	rowNum, err := appConfig.Clusters.CountClusters(r.Context())
	if err != nil {
		logger.Error("Failed to count rows by query",
			zap.String("url", r.URL.Path),
			zap.Any("search_request", search_request), // Log the request details
			zap.Error(err))                            // Log the error
		http.Error(w, "Failed to count total items", http.StatusInternalServerError)
		return // Important: stop execution after sending error
	}

	totalPageNum := (rowNum + PAGE_SIZE - 1) / PAGE_SIZE // To round it up instead

	err = render.RenderClusterHeatmapPage(w, appConfig.Site, appConfig.Genomes, rows, search_request, totalPageNum)

	if err != nil {
		logger.Error(err.Error()) // Already logging the error message
		http.Error(w, "Failed to render table", http.StatusInternalServerError)
		// No return needed here as it's the last statement in the function
	}
}

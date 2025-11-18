package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/handler/request"
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

// Search page
func (dbctx *DBContext) ClusterSearchPage(w http.ResponseWriter, r *http.Request) {

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	searchTerm := r.URL.Query().Get("search")
	searchBy := r.URL.Query().Get("search_by")
	searchByF := request.NewClusterField(searchBy)

	orderBy := r.URL.Query().Get("order_by")
	if orderBy == "" {
		orderBy = defaultOrderBy
	}
	orderByF := request.NewClusterField(orderBy)

	currentPage := parsePositiveIntFallback(r.URL.Query().Get("page"), defaultPageNumber)
	pageSize := parsePositiveIntFallback(r.URL.Query().Get("page_size"), defaultPageSize)
	orderDir := normalizeOrderDir(r.URL.Query().Get("order_dir"))

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

	var search_request = request.ClusterSearchRequest{
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

	rows, _ := model.SearchGeneCluster(dbctx.DB, search_request)
	rowNum, _ := model.CountSearchRow(dbctx.DB, search_request)

	totalPageNum := (rowNum + pageSize - 1) / pageSize // Rounding up

	err := render.RenderClustersAsTable(w, rows, search_request, totalPageNum)

	if err != nil {
		logger.Error(err.Error())
		http.Error(w, "Failed to render table", http.StatusInternalServerError)
	}
}

// Main page.
func (dbctx *DBContext) MainPage(w http.ResponseWriter, r *http.Request) {

	const PAGE_SIZE = defaultPageSize

	pageNum := parsePositiveIntFallback(r.URL.Query().Get("page"), defaultPageNumber)

	orderBy := r.URL.Query().Get("order_by")
	if orderBy == "" {
		orderBy = defaultOrderBy
	}
	orderByF := request.NewClusterField(orderBy)
	orderDir := normalizeOrderDir(r.URL.Query().Get("order_dir"))

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

	var search_request = request.ClusterSearchRequest{
		Search_For:   "",
		Search_Field: request.ClusterFieldFunction,
		Order_By:     orderByF,
		Order_Dir:    orderDir,
		Page:         pageNum,
		Page_Size:    PAGE_SIZE,
		Genome_IDs:   model.ALL_GENOME_ID, // Default to all genomes
		Color_By:     colorBy,
	}

	rows, err := model.GetMainPage(dbctx.DB, search_request) // Capture the error here
	if err != nil {
		logger.Error("Failed to get main page data from model",
			zap.String("url", r.URL.Path),
			zap.Any("search_request", search_request), // Log the request details
			zap.Error(err))                            // Log the error
		http.Error(w, "Failed to retrieve data", http.StatusInternalServerError)
		return // Important: stop execution after sending error
	}

	rowNum, err := model.CountAllRow(dbctx.DB)

	// rowNum, err := model.CountRowByQuery(, ) // Capture the error here
	if err != nil {
		logger.Error("Failed to count rows by query",
			zap.String("url", r.URL.Path),
			zap.Any("search_request", search_request), // Log the request details
			zap.Error(err))                            // Log the error
		http.Error(w, "Failed to count total items", http.StatusInternalServerError)
		return // Important: stop execution after sending error
	}

	totalPageNum := (rowNum + PAGE_SIZE - 1) / PAGE_SIZE // To round it up instead

	err = render.RenderClustersAsTable(w, rows, search_request, totalPageNum)

	if err != nil {
		logger.Error(err.Error()) // Already logging the error message
		http.Error(w, "Failed to render table", http.StatusInternalServerError)
		// No return needed here as it's the last statement in the function
	}
}

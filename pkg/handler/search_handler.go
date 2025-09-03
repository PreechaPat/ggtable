package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/handler/request"
	"github.com/yumyai/ggtable/pkg/model"
	"go.uber.org/zap"
)

// TODO:
// Doing a full json validations
// https://www.alexedwards.net/blog/how-to-properly-parse-a-json-request-body
// Organize the DB connection
// https://www.alexedwards.net/blog/organising-database-access

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
		orderBy = "cluster_id"
	}

	orderByF := request.NewClusterField(orderBy)
	pageNumStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")
	currentPage, _ := strconv.Atoi(pageNumStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)

	if pageSizeStr == "" || pageSize <= 0 {
		logger.Error("Invalid page size, defaulting to 100")
		pageSize = 100 // Default page size
	}

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
	)

	var search_request = request.ClusterSearchRequest{
		Search_For:   searchTerm,
		Search_Field: searchByF,
		Order_By:     orderByF,
		Page:         currentPage,
		Page_Size:    pageSize,
		Genome_IDs:   includeGenome,
		// RequireGenesFromGenomes: reqGeneFromGenome,
	}

	rows, _ := model.SearchGeneCluster(dbctx.DB, search_request)
	rowNum, _ := model.CountRowByQuery(dbctx.DB, search_request)
	totalPageNum := (rowNum + pageSize - 1) / pageSize // Rounding up

	err := model.RenderClustersAsTable(w, rows, search_request, totalPageNum)

	if err != nil {
		logger.Error(err.Error())
		http.Error(w, "Failed to render table", http.StatusInternalServerError)
	}
}

// Main page queries everything.
func (dbctx *DBContext) MainPage(w http.ResponseWriter, r *http.Request) {

	PAGE_SIZE := 100

	pageNumStr := r.URL.Query().Get("page")
	pageNum, _ := strconv.Atoi(pageNumStr)

	orderBy := r.URL.Query().Get("order_by")
	if orderBy == "" {
		orderBy = "cluster_id"
	}

	orderByF := request.NewClusterField(orderBy)

	if pageNum == 0 {
		pageNum = 1
	}

	logger.Info("Running mainpage",
		zap.String("url", r.URL.Path),
		zap.Int("page", pageNum),
		zap.Int("page_size", PAGE_SIZE),
		zap.String("order_by", orderByF.String()),
	)

	var search_request = request.ClusterSearchRequest{
		Search_For:   "",
		Search_Field: request.ClusterFieldFunction,
		Order_By:     orderByF,
		Page:         pageNum,
		Page_Size:    PAGE_SIZE,
		Genome_IDs:   model.ALL_GENOME_ID, // Default to all genomes
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

	rowNum, err := model.CountRowByQuery(dbctx.DB, search_request) // Capture the error here
	if err != nil {
		logger.Error("Failed to count rows by query",
			zap.String("url", r.URL.Path),
			zap.Any("search_request", search_request), // Log the request details
			zap.Error(err))                            // Log the error
		http.Error(w, "Failed to count total items", http.StatusInternalServerError)
		return // Important: stop execution after sending error
	}

	totalPageNum := (rowNum + PAGE_SIZE - 1) / PAGE_SIZE // To round it up instead

	err = model.RenderClustersAsTable(w, rows, search_request, totalPageNum)

	if err != nil {
		logger.Error(err.Error()) // Already logging the error message
		http.Error(w, "Failed to render table", http.StatusInternalServerError)
		// No return needed here as it's the last statement in the function
	}
}

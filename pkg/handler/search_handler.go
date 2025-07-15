package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/handler/types"
	"github.com/yumyai/ggtable/pkg/model"
	"go.uber.org/zap"
)

// TODO:
// Doing a full json validations
// https://www.alexedwards.net/blog/how-to-properly-parse-a-json-request-body
// Organize the DB connection
// https://www.alexedwards.net/blog/organising-database-access

// var (
// 	HEADER []string = []string{
// 		"CBS57885", "CBS57985", "CBS57785", "CBS57585", "CBS57385", "CBS57385m",
// 		"EQ25", "EQ04", "CAO", "EQ10", "EQ09", "EQ05",
// 		"ATCC200269", "P45BR", "CBS101555", "PINS", "PIS", "PINSPB",
// 		"P41NK", "CBS67385", "P44TW", "46P211CM", "P47ZG", "46P213L8",
// 		"RT01", "RT02", "SIMI2989", "SIMI7873", "KCB07", "CBS101039",
// 		"P16PC", "KCB02", "P36SW", "P53LD", "P40KJ", "CU43150",
// 		"SIMI91646", "M29", "P39KP", "MCC18", "59P211AT", "CR02",
// 		"SIMI452345", "46P214L10", "P50PR", "KCB05", "KCB06", "P42PT",
// 		"SIMI8727", "P15ON", "P34UM", "RM902", "MCC5", "SIMI18093",
// 		"P38WA", "ATCC28251", "ATCC64221", "46P212L4", "SIMI769548", "P52WN",
// 		"P46EP", "P211", "P48DZ", "MCC13", "MCC13m", "MCC17",
// 		"SIMI4763", "KCB01", "KCB03", "KCB08", "KCB09", "P43SY",
// 		"SIMI330644", "SIMI292145", "ATCC90586", "PARR", "RM906", "RCB01",
// 		"ATCC32230", "PAPH", "PINF", "PPAR", "PCAP", "PRAM",
// 		"PCIN", "PSOJ", "HARA", "PVEX", "PIRR", "PIWA",
// 		"PULT", "LGIG", "SDEC", "SPAR", "AAST", "AINV",
// 		"CBS134681", "ACAN", "ALAI", "PTRI", "TPSE",
// 	}
// )

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

	// I can parse form here, but why though?
	if err := r.ParseForm(); err != nil {

		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	searchTerm := r.URL.Query().Get("search")
	searchBy := r.URL.Query().Get("search_by")
	pageNumStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")
	currentPage, _ := strconv.Atoi(pageNumStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)

	// Array to hold filtered genome
	var genomeIDs []string

	// Iterate over query parameters
	for key := range r.URL.Query() {
		if strings.HasPrefix(key, "gm_") {
			// Strip "gn_" prefix and append to the array
			strippedKey := strings.TrimPrefix(key, "gm_")
			genomeIDs = append(genomeIDs, strippedKey)
		}
	}

	// Only include those cluster with following genes
	var geneIn []string
	// Iterate over query parameters
	for key := range r.URL.Query() {
		if strings.HasPrefix(key, "gn_") {
			// Strip "gn_" prefix and append to the array
			strippedKey := strings.TrimPrefix(key, "gn_")
			geneIn = append(geneIn, strippedKey)
		}
	}

	logger.Debug("Running search",
		zap.String("searchterm", searchTerm),
		zap.String("url", r.URL.Path),
		zap.Int("Pagesize", pageSize))

	var search_request = types.SearchRequest{
		Search_for:   searchTerm,
		Search_field: searchBy,
		Page:         currentPage,
		Page_size:    pageSize,
		GenomeIDs:    genomeIDs,
	}

	rows, _ := model.SearchGeneCluster(dbctx.DB, search_request)
	rowNum, _ := model.CountRowByQuery(dbctx.DB, search_request)
	totalPageNum := (rowNum + pageSize - 1) / pageSize // To round it up instead

	err := model.RenderClustersAsTable(w, rows, genomeIDs, currentPage, totalPageNum, pageSize)

	if err != nil {
		logger.Error(err.Error())
		http.Error(w, "Failed to render table", http.StatusInternalServerError)
	}
}

// Main page query everything.
func (dbctx *DBContext) MainPage(w http.ResponseWriter, r *http.Request) {

	PAGE_SIZE := 50

	pageNumStr := r.URL.Query().Get("page")
	pageNum, _ := strconv.Atoi(pageNumStr)

	if pageNum == 0 {
		pageNum = 1
	}

	logger.Info("Running mainpage",
		zap.String("url", r.URL.Path),
		zap.Int("page", pageNum))

	var search_request = types.SearchRequest{
		Search_for:   "",
		Search_field: "function",
		Page:         pageNum,
		Page_size:    PAGE_SIZE,
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

	err = model.RenderClustersAsTable(w, rows, model.ALL_GENOME_ID, pageNum, totalPageNum, PAGE_SIZE)

	if err != nil {
		logger.Error(err.Error()) // Already logging the error message
		http.Error(w, "Failed to render table", http.StatusInternalServerError)
		// No return needed here as it's the last statement in the function
	}
}

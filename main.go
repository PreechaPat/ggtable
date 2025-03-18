package main

import (
	"database/sql"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/joho/godotenv"

	"github.com/yumyai/ggtable/logger"
	mydb "github.com/yumyai/ggtable/pkg/db"
	"github.com/yumyai/ggtable/pkg/handler"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	_ "modernc.org/sqlite"
)

var (
	ggtable_data string
)

func main() {

	// Establish logger
	VERSION := "0.0.3"
	LOG_LEVEL := zapcore.InfoLevel

	if err := logger.InitLogger(LOG_LEVEL); err != nil {
		panic(err)
	}

	// Try load env
	dotenvErr := godotenv.Load()

	if dotenvErr != nil {
		logger.Warn("No .env found, using local environment")
	}

	defer logger.Sync() // Make sure that the buffered is flushed.

	ggtable_data = os.Getenv("GGTABLE_DATA")

	if ggtable_data == "" {
		logger.Warn("No local environment (GGTABLE_DATA), using default value (./data)")
		ggtable_data = "./data" // Replace "default_value" with your desired fallback value
	}

	ggtable_sqlite := path.Join(ggtable_data, "db/gene_table.db")
	seq_db := path.Join(ggtable_data, "db/sequence_db")
	prot_db := path.Join(ggtable_data, "db/blastdb/pythium_prot_v3")
	nucl_db := path.Join(ggtable_data, "db/blastdb/pythium_nucl_v3")

	// Connect to db
	db, _ := sql.Open("sqlite", ggtable_sqlite)

	dbctx := &handler.DBContext{
		DB:           db,
		Sequence_DB:  &mydb.SequenceDB{Dir: seq_db},
		ProtBLAST_DB: prot_db,
		NuclBLAST_DB: nucl_db,
	}

	logger.Info("Start:", zap.String("Version", VERSION))
	logger.Info("Open database on", zap.String("DB_LOC", ggtable_sqlite))

	mux := NewRouter(dbctx)

	// Apply middleware
	// m := middle.LoggingMiddleware(middle.CreateMiddlewareLogger(zapcore.DebugLevel))
	// newmux := m(mux)

	logger.Info("Server starting on :8080...")
	httpErr := http.ListenAndServe("0.0.0.0:8080", mux)
	if httpErr != nil {
		logger.Error("Error starting server:", zap.String("error message", httpErr.Error()))
	}
}

// Move to router.go in the next iteration
func NewRouter(dbctx *handler.DBContext) *http.ServeMux {
	mux := http.NewServeMux()

	// Error route
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	// Main routes
	mux.HandleFunc("GET /", dbctx.MainPage)
	mux.HandleFunc("GET /search", dbctx.ClusterSearchPage)
	mux.HandleFunc("POST /blast", dbctx.BlastSearchPage)
	mux.HandleFunc("GET /cluster/{cluster_id}", dbctx.ClusterPage) // Go to cluster-page, difference from below TODO: change name to avoide future confusion
	mux.HandleFunc("GET /cluster/", dbctx.GetClusterByGeneHandler) // Use by in BLAST result, TODO: Change name later
	mux.HandleFunc("GET /redirect/blastn/", dbctx.BlastNRedirectPage)
	mux.HandleFunc("GET /redirect/blastp/", dbctx.BlastPRedirectPage)

	// API routes
	// mux.HandleFunc("GET /api/v1/search", dbctx.ClusterSearchAPI)
	mux.HandleFunc("GET /api/v1/health", handler.HealthCheck)
	mux.HandleFunc("GET /api/v1/cluster/{cluster_id}", dbctx.ClusterPage)

	// Get sequences
	mux.HandleFunc("GET /sequence/by-gene", dbctx.GetGeneSequenceHandler)
	mux.HandleFunc("GET /sequence/by-region", dbctx.GetRegionSequenceHandler)
	mux.HandleFunc("GET /sequence/by-cluster", dbctx.GetSequenceByClusterIDHandler)

	// Static files
	setupStaticFiles(mux)

	// SPA
	// Not working ATM.
	// setupSPA(mux)

	return mux
}

// Manually add static for all route that use this
func setupStaticFiles(mux *http.ServeMux) {
	_ = mime.AddExtensionType(".js", "text/javascript")
	_ = mime.AddExtensionType(".css", "text/css")
	fs := http.FileServer(http.Dir("./static/"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	// Additional route that also use it
	// Maybe there will be a better way...
	// mux.Handle("GET /cluster/static/", http.StripPrefix("/cluster/static/", fs))
}

func setupSPA(mux *http.ServeMux) {

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		logger.Error("Unable to get caller information")
		return
	}

	projectPath := filepath.Dir(filename)
	logger.Info("SPA is available in:", zap.String("Loc", projectPath))
	// distDir := filepath.Join(projectPath, "../ggapp-react/dist")
	distDir := "/workspaces/ggapp-react/dist"
	logger.Info("Serving SPA from", zap.String("dir", distDir))
	spa := http.FileServer(http.Dir(distDir))
	mux.Handle("GET /v1/", http.StripPrefix("/v1/", spa))
}

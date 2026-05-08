package main

import (
	"database/sql"
	"flag"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/db"
	"github.com/yumyai/ggtable/pkg/handler"
	"github.com/yumyai/ggtable/pkg/model"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	_ "modernc.org/sqlite"
)

var (
	ggtable_data     string
	ggtable_title    string
	ggtable_subtitle string
)

// App configuration parsed from flags/env
type AppConfig struct {
	Version  string
	DataDir  string // GGTABLE_DATA
	Title    string // GGTITLE
	Subtitle string // GGSUBTITLE
	Addr     string // listen addr, default 0.0.0.0:8080
	Verbose  bool   // -v
	Sorted   string
}

// ParseConfig loads .env (if present), uses env as defaults, and then parses flags.
func ParseConfig() AppConfig {
	_ = godotenv.Load() // best-effort; env wins if present

	cfg := AppConfig{
		Version:  "3.0.3",
		DataDir:  getenv("GGTABLE_DATA", "./data"),
		Title:    getenv("GGTITLE", ""),
		Subtitle: getenv("GGSUBTITLE", ""),
		Addr:     getenv("GGTABLE_ADDR", "0.0.0.0:8080"),
		Sorted:   getenv("GGSORTED", ""),
	}

	flag.BoolVar(&cfg.Verbose, "v", false, "Enable verbose (debug) logging")
	flag.StringVar(&cfg.DataDir, "data", cfg.DataDir, "Path to data directory (default from $GGTABLE_DATA)")
	flag.StringVar(&cfg.Title, "title", cfg.Title, "Application title (default from $GGTITLE)")
	flag.StringVar(&cfg.Subtitle, "subtitle", cfg.Subtitle, "Application subtitle (default from $GGSUBTITLE)")
	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "HTTP listen address")

	flag.Parse()
	return cfg
}

func main() {
	cfg := ParseConfig()

	// Optional: export flags back to env for downstream packages that read env
	_ = os.Setenv("GGTABLE_DATA", cfg.DataDir)
	_ = os.Setenv("GGTITLE", cfg.Title)
	_ = os.Setenv("GGSUBTITLE", cfg.Subtitle)

	if err := run(cfg); err != nil {
		// If logger isn't up yet, print; else log and exit.
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

// run wires everything together using the parsed config.
func run(cfg AppConfig) error {
	// Logger
	logLevel := zapcore.InfoLevel
	if cfg.Verbose {
		logLevel = zapcore.DebugLevel
	}
	if err := logger.InitLogger(logLevel); err != nil {
		return err
	}
	defer logger.Sync()

	// Data paths
	if cfg.DataDir == "" {
		logger.Warn("No data dir provided; falling back to ./data")
		cfg.DataDir = "./data"
	}
	sqlitePath := path.Join(cfg.DataDir, "db/gene_table.db")
	protDB := path.Join(cfg.DataDir, "db/blastdb/genetable_genes_prot")
	nuclDB := path.Join(cfg.DataDir, "db/blastdb/genetable_genes_nucl")
	seqDB := path.Join(cfg.DataDir, "db/sequence_db")

	// DB connect
	// FIX: This create a new sqlite3 database if it does not exists.
	// it should be an error instead.

	// NOTE: Also, this part should be move into NewGeneClusterDB
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=synchronous(NORMAL)", sqlitePath)
	dbConn, err := sql.Open("sqlite", dsn)
	if err != nil {
		logger.Fatal("Cannot connect to database", zap.String("DB_LOC", sqlitePath), zap.Error(err))
		return err
	}

	dbConn.SetMaxOpenConns(10)
	dbConn.SetMaxIdleConns(5)
	dbConn.SetConnMaxLifetime(3 * time.Minute)

	gcdb := db.NewGeneClusterDB(dbConn, &db.SequenceDB{Dir: seqDB})

	appConfig := &handler.AppContext{
		GCDB:         gcdb,
		BlastManager: db.NewBlastManager(),
		ProtBLASTDB:  protDB,
		NuclBLASTDB:  nuclDB,
	}

	logger.Info("Start", zap.String("Version", cfg.Version))
	logger.Info("Open database", zap.String("DB_LOC", sqlitePath))
	if cfg.Title != "" {
		logger.Info("App title", zap.String("title", cfg.Title), zap.String("subtitle", cfg.Subtitle))
	}

	// Router
	mux := NewRouter(appConfig)

	// Initialize header map from genome_id to full name
	if err := model.InitMapHeader(dbConn); err != nil {
		logger.Fatal("Cannot init header", zap.String("MAP_HEADER_ERR", err.Error()))
		return err
	}
	if cfg.Sorted != "" {
		// Split by comma
		sortedIDs := []string{}
		sortedIDs = append(sortedIDs, strings.Split(cfg.Sorted, ",")...)
		model.SetGenomeID(sortedIDs)
		logger.Info("Using manually sorted HEADER with length", zap.Int("len", len(sortedIDs)))
	}
	// Serve
	logger.Info("Server starting", zap.String("addr", cfg.Addr))
	if httpErr := http.ListenAndServe(cfg.Addr, mux); httpErr != nil {
		logger.Error("Error starting server", zap.String("error", httpErr.Error()))
		return httpErr
	}
	return nil
}

// Move to router.go in the next iteration
func NewRouter(appConfig *handler.AppContext) *http.ServeMux {
	mux := http.NewServeMux()

	// Error route
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	// Main routes
	mux.HandleFunc("GET /", appConfig.MainPage)
	mux.HandleFunc("GET /search", appConfig.ClusterSearchPage)
	mux.HandleFunc("POST /blast", appConfig.BlastSearchPage)
	mux.HandleFunc("GET /blast/{job_id}", appConfig.BlastStatusPage)
	mux.HandleFunc("GET /cluster/table/{cluster_id}", appConfig.ClusterDetailPage) // Dedicated cluster table page.
	mux.HandleFunc("GET /cluster/heatmap/{genome_id}/{contig_id}/{gene_id}", appConfig.ClusterHeatmapPage)
	mux.HandleFunc("GET /redirect/blastn/", appConfig.BlastNRedirectPage)
	mux.HandleFunc("GET /redirect/blastp/", appConfig.BlastPRedirectPage)

	// API routes
	// mux.HandleFunc("GET /api/v1/search", appConfig.ClusterSearchAPI)
	mux.HandleFunc("GET /api/v1/health", handler.HealthCheck)
	mux.HandleFunc("GET /api/v1/cluster/{cluster_id}", appConfig.ClusterDetailPage)

	// Get sequences
	mux.HandleFunc("GET /sequence/by-gene", appConfig.GetGeneSequenceHandler)
	mux.HandleFunc("GET /sequence/by-region", appConfig.GetRegionSequenceHandler)
	mux.HandleFunc("GET /sequence/by-cluster", appConfig.GetSequenceByClusterIDHandler)

	// Static
	setupStaticFiles(mux)
	// setupSPA(mux) // optional

	return mux
}

// Manually add static for all route that use this
func setupStaticFiles(mux *http.ServeMux) {
	_ = mime.AddExtensionType(".js", "text/javascript")
	_ = mime.AddExtensionType(".css", "text/css")
	fs := http.FileServer(http.Dir("./static/"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))
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

// small helper
func getenv(k, default_val string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return default_val
}

package handler

import (
	"io/fs"
	"net/http"

	"github.com/yumyai/ggtable/pkg/middle"
)

// Routes returns the full HTTP handler: every route, /static/ served from
// static, wrapped in request-ID and logging middleware.
func (app *AppContext) Routes(static fs.FS) http.Handler {
	mux := http.NewServeMux()

	// Pages
	mux.HandleFunc("GET /{$}", app.MainPage)
	mux.HandleFunc("GET /search", app.ClusterSearchPage)
	mux.HandleFunc("GET /cluster/table/{cluster_id}", app.ClusterDetailPage)
	mux.HandleFunc("GET /cluster/heatmap/{genome_id}/{contig_id}/{gene_id}", app.ClusterHeatmapPage)

	// BLAST
	mux.HandleFunc("POST /blast", app.BlastSearchPage)
	mux.HandleFunc("GET /blast/{job_id}", app.BlastStatusPage)
	mux.HandleFunc("GET /redirect/blastn/", app.BlastNRedirectPage)
	mux.HandleFunc("GET /redirect/blastp/", app.BlastPRedirectPage)

	// Sequences (FASTA)
	mux.HandleFunc("GET /sequence/by-gene", app.GetGeneSequenceHandler)
	mux.HandleFunc("GET /sequence/by-region", app.GetRegionSequenceHandler)
	mux.HandleFunc("GET /sequence/by-cluster", app.GetSequenceByClusterIDHandler)

	// API
	mux.HandleFunc("GET /api/v1/health", HealthCheck)
	mux.HandleFunc("GET /api/v1/cluster/{cluster_id}", app.ClusterDetailPage) // HTML for now, same as /cluster/table

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))

	return middle.RequestID(middle.Logging(mux))
}

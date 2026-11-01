package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/data"
	"github.com/yumyai/ggtable/pkg/handler"
	"github.com/yumyai/ggtable/pkg/model"
	"github.com/yumyai/ggtable/pkg/render"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// version is set at build time: go build -ldflags "-X main.version=3.0.5".
var version = "dev"

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
	// StaticDir serves /static/ from disk instead of the embedded copy (dev only).
	StaticDir string // GGTABLE_STATIC
}

// ParseConfig loads .env (if present), uses env as defaults, and then parses flags.
func ParseConfig() AppConfig {
	_ = godotenv.Load() // best-effort; env wins if present

	cfg := AppConfig{
		Version:   version,
		DataDir:   getenv("GGTABLE_DATA", "./data"),
		Title:     getenv("GGTITLE", render.DefaultTitle),
		Subtitle:  getenv("GGSUBTITLE", ""),
		Addr:      getenv("GGTABLE_ADDR", "0.0.0.0:8080"),
		Sorted:    getenv("GGSORTED", ""),
		StaticDir: getenv("GGTABLE_STATIC", ""),
	}

	flag.BoolVar(&cfg.Verbose, "v", false, "Enable verbose (debug) logging")
	flag.StringVar(&cfg.DataDir, "data", cfg.DataDir, "Path to data directory (default from $GGTABLE_DATA)")
	flag.StringVar(&cfg.Title, "title", cfg.Title, "Application title (default from $GGTITLE)")
	flag.StringVar(&cfg.Subtitle, "subtitle", cfg.Subtitle, "Application subtitle (default from $GGSUBTITLE)")
	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "HTTP listen address")
	flag.StringVar(&cfg.StaticDir, "static", cfg.StaticDir, "Serve /static/ from this directory instead of the embedded files, for editing JS/CSS without rebuilding (default from $GGTABLE_STATIC)")

	flag.Parse()
	return cfg
}

func main() {
	cfg := ParseConfig()

	if err := run(cfg); err != nil {
		// The logger may not be up yet, and this is the one place the error is reported.
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

	// Data: fails here, with the reason, if the data directory is incomplete or malformed.
	if cfg.DataDir == "" {
		return errors.New("no data directory: pass -data or set GGTABLE_DATA")
	}
	store, err := data.Open(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("cannot use data directory %s: %w", cfg.DataDir, err)
	}
	defer store.Close()

	genomes := store.Genomes
	if cfg.Sorted != "" {
		var unknown []string
		genomes, unknown = genomes.Reorder(strings.Split(cfg.Sorted, ","))
		if len(unknown) > 0 {
			logger.Warn("GGSORTED lists genomes that are not in genome_info; ignoring them", zap.Strings("unknown", unknown))
		}
		logger.Info("Using GGSORTED genome order", zap.Int("genomes", len(genomes.IDs())))
	}

	appConfig := &handler.AppContext{
		Clusters:     store.Clusters,
		Seqs:         store.Seqs,
		Genomes:      genomes,
		BlastManager: model.NewBlastManager(),
		Site:         render.SiteInfo{Title: cfg.Title, Subtitle: cfg.Subtitle},
	}

	logger.Info("Start", zap.String("Version", cfg.Version))
	logger.Info("Opened data directory", zap.String("dir", cfg.DataDir), zap.Int("genomes", len(genomes.IDs())))
	logger.Info("App title", zap.String("title", cfg.Title), zap.String("subtitle", cfg.Subtitle))

	static, err := staticFiles(cfg.StaticDir)
	if err != nil {
		return err
	}

	logger.Info("Server starting", zap.String("addr", cfg.Addr))
	if err := http.ListenAndServe(cfg.Addr, appConfig.Routes(static)); err != nil {
		return fmt.Errorf("serve on %s: %w", cfg.Addr, err)
	}
	return nil
}

// staticFS holds static/ compiled into the binary, so it runs from any directory.
//
//go:embed static
var staticFS embed.FS

// staticFiles returns the files served at /static/: the embedded copy, or
// staticDir on disk when set (to edit JS/CSS without rebuilding).
func staticFiles(staticDir string) (fs.FS, error) {
	// Some minimal containers lack /etc/mime.types; these must be right for browsers
	// to run the scripts. An error here only means the type was already registered.
	_ = mime.AddExtensionType(".js", "text/javascript")
	_ = mime.AddExtensionType(".css", "text/css")

	if staticDir == "" {
		sub, err := fs.Sub(staticFS, "static")
		if err != nil {
			return nil, fmt.Errorf("open embedded static files: %w", err)
		}
		return sub, nil
	}
	info, err := os.Stat(staticDir)
	if err != nil {
		return nil, fmt.Errorf("cannot use static dir (omit -static to use embedded files): %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("static dir %q is not a directory (omit -static to use embedded files)", staticDir)
	}
	logger.Info("Serving static files from disk", zap.String("dir", staticDir))
	return os.DirFS(staticDir), nil
}

// small helper
func getenv(k, default_val string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return default_val
}

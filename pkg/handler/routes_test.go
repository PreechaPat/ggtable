package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/yumyai/ggtable/internal/testdb"
	"github.com/yumyai/ggtable/pkg/data/sqldb"
	"github.com/yumyai/ggtable/pkg/middle"
	"github.com/yumyai/ggtable/pkg/model"
)

// sqlOnlyRoutes serves the routes backed by a SQLite fixture; routes that
// need BLAST are tested with newTestApp.
func sqlOnlyRoutes(t *testing.T) http.Handler {
	t.Helper()
	app := &AppContext{
		Clusters:     sqldb.New(testdb.New(t)),
		Genomes:      model.NewGenomeCatalog([]string{"G1", "G2", "G3"}, map[string]string{"G1": "Genome One", "G2": "Genome Two", "G3": "Genome Three"}),
		BlastManager: model.NewBlastManager(),
	}
	return app.Routes(fstest.MapFS{"gene-table.js": {Data: []byte("// js")}})
}

func get(t *testing.T, h http.Handler, method, url string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(method, url, nil))
	return rr
}

func TestRoutes(t *testing.T) {
	h := sqlOnlyRoutes(t)
	tests := []struct {
		method, url string
		status      int
		body        string // substring, optional
	}{
		{"GET", "/", 200, "ATP synthase"},
		{"GET", "/?order_by=function&order_dir=desc", 200, ""},
		{"GET", "/search?search=ATP&search_by=function", 200, "ATP binding"},
		{"GET", "/search?search=x&search_by=bogus", 200, ""},
		{"GET", "/cluster/table/c1", 200, "ATP synthase"},
		{"GET", "/cluster/table/nope", 404, "not found"},
		{"GET", "/api/v1/cluster/c1", 200, "ATP synthase"},
		{"GET", "/cluster/heatmap/G2/ctgB/g2a", 200, "ATP synthase"},
		{"GET", "/api/v1/health", 200, `"health":"ok"`},
		{"GET", "/blast/unknown-job", 404, ""},
		{"GET", "/static/gene-table.js", 200, "// js"},
		{"GET", "/static/missing.js", 404, ""},
		{"GET", "/favicon.ico", 404, ""},
		{"GET", "/no/such/page", 404, ""},
		{"POST", "/", 405, ""},
	}
	for _, tt := range tests {
		rr := get(t, h, tt.method, tt.url)
		if rr.Code != tt.status {
			t.Errorf("%s %s: status %d, want %d", tt.method, tt.url, rr.Code, tt.status)
		}
		if tt.body != "" && !strings.Contains(rr.Body.String(), tt.body) {
			t.Errorf("%s %s: body does not contain %q", tt.method, tt.url, tt.body)
		}
	}
}

func TestRoutes_RequestID(t *testing.T) {
	h := sqlOnlyRoutes(t)
	a := get(t, h, "GET", "/api/v1/health").Header().Get(middle.RequestIDHeader)
	b := get(t, h, "GET", "/api/v1/health").Header().Get(middle.RequestIDHeader)
	if !strings.HasPrefix(a, "req-") || a == b {
		t.Errorf("request IDs %q, %q: want unique req- prefixed IDs", a, b)
	}
}

func TestRoutes_BlastSubmit(t *testing.T) {
	h := sqlOnlyRoutes(t)
	for _, body := range []string{`not json`, `{"blast_type":"tblastx","sequence":"A"}`, `{"blast_type":"blastn","sequence":"  "}`} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("POST", "/blast", strings.NewReader(body)))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("POST /blast %s: status %d, want 400", body, rr.Code)
		}
	}
}

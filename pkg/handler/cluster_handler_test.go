package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yumyai/ggtable/internal/testdb"
	"github.com/yumyai/ggtable/pkg/data"
	"github.com/yumyai/ggtable/pkg/data/sqldb"
	"github.com/yumyai/ggtable/pkg/model"
)

func newTestApp(t *testing.T) *AppContext {
	t.Helper()
	store, err := data.Open(testdb.DataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return &AppContext{Clusters: store.Clusters, Seqs: store.Seqs, Genomes: store.Genomes, BlastManager: model.NewBlastManager()}
}

func TestClusterDetailPage(t *testing.T) {
	app := &AppContext{
		Clusters: sqldb.New(testdb.New(t)),
		Genomes:  model.NewGenomeCatalog([]string{"G1"}, map[string]string{"G1": "Genome One"}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /cluster/table/{cluster_id}", app.ClusterDetailPage)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/cluster/table/c1", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "ATP synthase") {
		t.Errorf("c1: status %d, body contains cluster = %v", rr.Code, strings.Contains(rr.Body.String(), "ATP synthase"))
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/cluster/table/nope", nil))
	if rr.Code != http.StatusNotFound {
		t.Errorf("unknown cluster: status %d, want 404", rr.Code)
	}
}

func TestMainAndSearchPages(t *testing.T) {
	app := &AppContext{
		Clusters: sqldb.New(testdb.New(t)),
		Genomes:  model.NewGenomeCatalog([]string{"G1", "G2"}, map[string]string{"G1": "Genome One", "G2": "Genome Two"}),
	}
	for _, url := range []string{"/", "/search?search=ATP&search_by=function", "/search?search=x&search_by=bogus"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, url, nil)
		if url == "/" {
			app.MainPage(rr, req)
		} else {
			app.ClusterSearchPage(rr, req)
		}
		if rr.Code != http.StatusOK {
			t.Errorf("%s: status %d", url, rr.Code)
		}
	}
}

func TestGetSequenceByClusterIDHandler(t *testing.T) {
	app := newTestApp(t)

	rr := httptest.NewRecorder()
	app.GetSequenceByClusterIDHandler(rr, httptest.NewRequest(http.MethodGet, "/sequence/by-cluster?cluster_id=c1&is_prot=false", nil))
	body := rr.Body.String()
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, body)
	}
	// 3 genes + 1 region, gene headers labelled with genome names.
	if n := strings.Count(body, ">"); n != 4 {
		t.Errorf("got %d FASTA records, want 4:\n%s", n, body)
	}
	for _, want := range []string{">Genome One-G1//ctgA//g1a", ">Genome Two-G2//ctgB//g2a", ">Genome Three-G3//ctgC:100-900"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}

	rr = httptest.NewRecorder()
	app.GetSequenceByClusterIDHandler(rr, httptest.NewRequest(http.MethodGet, "/sequence/by-cluster?cluster_id=nope&is_prot=true", nil))
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), `cluster \"nope\" not found`) {
		t.Errorf("unknown cluster: %d %s", rr.Code, rr.Body.String())
	}
}

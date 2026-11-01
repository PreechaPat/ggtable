package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yumyai/ggtable/internal/testdb"
	"github.com/yumyai/ggtable/pkg/data"
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

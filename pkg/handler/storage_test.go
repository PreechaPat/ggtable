package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yumyai/ggtable/pkg/model"
)

type failingClusters struct {
	model.ClusterStore
	err error
}

func (s failingClusters) Cluster(context.Context, string) (*model.Cluster, error) { return nil, s.err }

// Exercise HTTP error translation without SQLite fixtures or BLAST executables.
func TestClusterStorageErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"missing", model.ErrNotFound, http.StatusNotFound},
		{"unavailable", errors.New("private storage details"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &AppContext{Clusters: failingClusters{err: tc.err}}
			req := httptest.NewRequest("GET", "/cluster/table/c1", nil)
			req.SetPathValue("cluster_id", "c1")
			rr := httptest.NewRecorder()
			app.ClusterDetailPage(rr, req)
			if rr.Code != tc.status {
				t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
			}
			if strings.Contains(rr.Body.String(), "private storage details") {
				t.Fatal("storage details leaked")
			}
		})
	}
}

func TestClusterSequenceStorageFailure(t *testing.T) {
	app := &AppContext{Clusters: failingClusters{err: errors.New("private storage details")}}
	rr := httptest.NewRecorder()
	app.GetSequenceByClusterIDHandler(rr, httptest.NewRequest("GET", "/sequence/by-cluster?cluster_id=c1&is_prot=false", nil))
	if rr.Code != http.StatusBadRequest || rr.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("%d %v", rr.Code, rr.Header())
	}
	if strings.Contains(rr.Body.String(), "private storage details") {
		t.Fatal("storage details leaked")
	}
}

func TestHeatmapColorAliases(t *testing.T) {
	for _, raw := range []string{"max_gene_completeness", "max_completeness", "completeness"} {
		if got := parseColorBy(raw); got != "max_gene_completeness" {
			t.Errorf("%q: %q", raw, got)
		}
	}
	for _, raw := range []string{"", "gene_copy_number", "copy_number", "copies", "copy", "bogus"} {
		if got := parseColorBy(raw); got != "gene_copy_number" {
			t.Errorf("%q: %q", raw, got)
		}
	}
}

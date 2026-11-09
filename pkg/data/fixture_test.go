package data

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yumyai/ggtable/internal/testdb"
	"github.com/yumyai/ggtable/pkg/model"
)

// TestRealData runs the data layer against a real ggprep/ggorigin build: the
// directory in $GGTABLE_TEST_DATA, else test_data/pins6/output (gitignored,
// built by scripts/build-test-data.sh). It skips when neither exists.
func TestRealData(t *testing.T) {
	testdb.RequireBlast(t)
	dir := os.Getenv("GGTABLE_TEST_DATA")
	if dir == "" {
		dir = filepath.Join("..", "..", "test_data", "pins6", "output")
	}
	if _, err := os.Stat(filepath.Join(dir, SQLiteFile)); err != nil {
		t.Skipf("no real data at %s (set GGTABLE_TEST_DATA)", dir)
	}

	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()

	ids := store.Genomes.IDs()
	if len(ids) == 0 {
		t.Fatal("no genomes")
	}
	t.Logf("%d genomes: %v", len(ids), ids)

	page, err := store.Clusters.MainPage(ctx, model.ClusterSearchRequest{Page: 1, Page_Size: 50, Genome_IDs: ids})
	if err != nil || len(page) == 0 {
		t.Fatalf("MainPage() = %d clusters, %v", len(page), err)
	}
	total, err := store.Clusters.CountClusters(ctx)
	if err != nil || total < len(page) {
		t.Fatalf("CountClusters() = %d, %v", total, err)
	}

	// Every gene and region on the first page must be fetchable from BLAST.
	var genes []model.GeneRef
	var regions []model.RegionRef
	for _, c := range page {
		genes = append(genes, c.GeneRefs()...)
		regions = append(regions, c.RegionRefs()...)
	}
	for _, prot := range []bool{true, false} {
		if _, err := store.Seqs.Genes(ctx, genes, prot); err != nil {
			t.Errorf("Genes(prot=%v) for %d genes: %v", prot, len(genes), err)
		}
	}
	if _, err := store.Seqs.Regions(ctx, regions); err != nil {
		t.Errorf("Regions() for %d regions: %v", len(regions), err)
	}

	// Search by the first cluster's own fields finds it again.
	first := page[0]
	byID, err := store.Clusters.Search(ctx, model.ClusterSearchRequest{
		Search_For: first.ClusterProperty.ClusterID, Search_Field: model.ClusterFieldClusterID,
		Order_By: model.ClusterFieldClusterID, Page: 1, Page_Size: 10})
	if err != nil || len(byID) == 0 || byID[0].ClusterProperty.ClusterID != first.ClusterProperty.ClusterID {
		t.Errorf("Search(cluster_id=%s) = %d clusters, %v", first.ClusterProperty.ClusterID, len(byID), err)
	}
	if len(genes) > 0 {
		g := genes[0]
		ids, err := store.Clusters.ClusterIDsForGene(ctx, g.GenomeID, g.GeneID)
		if err != nil || len(ids) == 0 {
			t.Errorf("ClusterIDsForGene(%s) = %v, %v", g.SeqID(), ids, err)
		}
	}
}

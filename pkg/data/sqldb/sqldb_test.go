package sqldb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yumyai/ggtable/internal/testdb"
	"github.com/yumyai/ggtable/pkg/model"
)

var ctx = context.Background()

func clusterIDs(cs []*model.Cluster) []string {
	ids := []string{}
	for _, c := range cs {
		ids = append(ids, c.ClusterProperty.ClusterID)
	}
	return ids
}

func geneIDs(g *model.Genome) []string {
	ids := []string{}
	if g == nil {
		return ids
	}
	for _, gene := range g.Genes {
		ids = append(ids, gene.GeneID)
	}
	return ids
}

func TestOpen(t *testing.T) {
	t.Run("valid file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "gene_table.db")
		testdb.WriteSQLite(t, path)
		db, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		db.Close()
	})
	t.Run("missing file is not created", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "gene_table.db")
		if _, err := Open(path); err == nil {
			t.Fatal("Open() of a missing file succeeded")
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("Open() created %s", path)
		}
	})
	t.Run("directory", func(t *testing.T) {
		_, err := Open(t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "is a directory") {
			t.Errorf("Open(dir) = %v", err)
		}
	})
	t.Run("not sqlite", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "gene_table.db")
		os.WriteFile(path, []byte("cluster_id\tcog\nc1\tCOG1\n"), 0o644)
		_, err := Open(path)
		if err == nil || !strings.Contains(err.Error(), "is this a SQLite file?") {
			t.Errorf("Open(text file) = %v", err)
		}
	})
}

func TestCheckSchema(t *testing.T) {
	if err := New(testdb.New(t)).CheckSchema(ctx); err != nil {
		t.Errorf("CheckSchema() on valid data = %v", err)
	}

	conn := testdb.New(t)
	conn.Exec(`DROP TABLE region_matches; ALTER TABLE gene_info DROP COLUMN gene_length`)
	err := New(conn).CheckSchema(ctx)
	for _, want := range []string{"table region_matches is missing", "table gene_info is missing columns gene_length", "ggprep"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("CheckSchema() = %v, want it to mention %q", err, want)
		}
	}

	err = New(testdb.Empty(t)).CheckSchema(ctx)
	if err == nil || !strings.Contains(err.Error(), "is empty") {
		t.Errorf("CheckSchema() on empty tables = %v", err)
	}
}

func TestGenomes(t *testing.T) {
	conn := testdb.New(t)
	conn.Exec(`INSERT INTO genome_info (genome_id, genome_fullname) VALUES ('G0', NULL)`)
	cat, err := New(conn).Genomes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"G1", "G2", "G3", "G0"}; !reflect.DeepEqual(cat.IDs(), want) {
		t.Errorf("IDs() = %v, want table order %v", cat.IDs(), want)
	}
	if n, _ := cat.Name("G2"); n != "Genome Two" {
		t.Errorf("Name(G2) = %q", n)
	}
	if n, _ := cat.Name("G0"); n != "G0" {
		t.Errorf("genome without full name: Name(G0) = %q, want G0", n)
	}
}

func TestClusterIDsForGene(t *testing.T) {
	db := New(testdb.New(t))
	got, err := db.ClusterIDsForGene(ctx, "G2", "g2a")
	if err != nil || !reflect.DeepEqual(got, []string{"c1"}) {
		t.Errorf("ClusterIDsForGene(G2, g2a) = %v, %v", got, err)
	}
	got, err = db.ClusterIDsForGene(ctx, "G1", "g2a")
	if err != nil || len(got) != 0 {
		t.Errorf("ClusterIDsForGene(G1, g2a) = %v, %v; want none", got, err)
	}
}

func TestCluster(t *testing.T) {
	db := New(testdb.New(t))

	c, err := db.Cluster(ctx, "c1")
	if err != nil {
		t.Fatal(err)
	}
	wantProp := model.ClusterProperty{ClusterID: "c1", CogID: "COG0001", RepresentativeGene: "g1a",
		ExpectedLength: "100", FunctionDescription: "ATP synthase"}
	if c.ClusterProperty != wantProp {
		t.Errorf("property = %+v, want %+v", c.ClusterProperty, wantProp)
	}
	if got := geneIDs(c.Genomes["G1"]); !reflect.DeepEqual(got, []string{"g1a", "g1b"}) {
		t.Errorf("G1 genes = %v", got)
	}
	g1b := c.Genomes["G1"].Genes[1]
	wantGene := model.Gene{GeneID: "g1b", Completeness: 50, Description: "synthase alpha fragment",
		Region: &model.Region{GenomeID: "G1", ContigID: "ctgA", Start: 400, End: 550}}
	if !reflect.DeepEqual(*g1b, wantGene) {
		t.Errorf("g1b = %+v %+v, want %+v %+v", *g1b, *g1b.Region, wantGene, *wantGene.Region)
	}
	wantRegion := []*model.Region{{GenomeID: "G3", ContigID: "ctgC", Start: 100, End: 900}}
	if !reflect.DeepEqual(c.Genomes["G3"].Regions, wantRegion) {
		t.Errorf("G3 regions = %v", c.Genomes["G3"].Regions)
	}
	if len(c.Genomes) != 3 {
		t.Errorf("genomes = %d, want 3 (no phantom empty-ID genome)", len(c.Genomes))
	}

	empty, err := db.Cluster(ctx, "c3")
	if err != nil || len(empty.Genomes) != 0 {
		t.Errorf("Cluster(c3) = %+v, %v; want a cluster with no genomes", empty, err)
	}

	if _, err := db.Cluster(ctx, "nope"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("Cluster(nope) error = %v, want ErrNotFound", err)
	}
}

func TestMainPage(t *testing.T) {
	db := New(testdb.New(t))
	req := model.ClusterSearchRequest{Order_By: model.ClusterFieldClusterID, Page: 1, Page_Size: 2}

	page1, err := db.MainPage(ctx, req)
	if err != nil || !reflect.DeepEqual(clusterIDs(page1), []string{"c1", "c2"}) {
		t.Fatalf("page 1 = %v, %v", clusterIDs(page1), err)
	}
	req.Page = 2
	page2, err := db.MainPage(ctx, req)
	if err != nil || !reflect.DeepEqual(clusterIDs(page2), []string{"c3", "c4"}) {
		t.Fatalf("page 2 = %v, %v; clusters without members must still be listed", clusterIDs(page2), err)
	}

	// Genome filter hides members of other genomes but keeps the clusters.
	filtered, err := db.MainPage(ctx, model.ClusterSearchRequest{Order_By: model.ClusterFieldClusterID, Page: 1, Page_Size: 10, Genome_IDs: []string{"G1"}})
	if err != nil || !reflect.DeepEqual(clusterIDs(filtered), []string{"c1", "c2", "c3", "c4"}) {
		t.Fatalf("filtered = %v, %v", clusterIDs(filtered), err)
	}
	c1 := filtered[0]
	if _, ok := c1.Genomes["G2"]; ok {
		t.Error("filter G1: c1 still has G2 genes")
	}
	if _, ok := c1.Genomes["G3"]; ok {
		t.Error("filter G1: c1 still has the G3 region")
	}
	if got := geneIDs(c1.Genomes["G1"]); !reflect.DeepEqual(got, []string{"g1a", "g1b"}) {
		t.Errorf("filter G1: c1 G1 genes = %v", got)
	}
}

func TestSearch(t *testing.T) {
	db := New(testdb.New(t))
	tests := []struct {
		name string
		req  model.ClusterSearchRequest
		want []string
	}{
		{"function substring, sorted by function",
			model.ClusterSearchRequest{Search_For: "ATP", Search_Field: model.ClusterFieldFunction, Order_By: model.ClusterFieldFunction},
			[]string{"c4", "c1"}},
		{"cog id",
			model.ClusterSearchRequest{Search_For: "0002", Search_Field: model.ClusterFieldCOGID, Order_By: model.ClusterFieldClusterID},
			[]string{"c2"}},
		{"cluster id",
			model.ClusterSearchRequest{Search_For: "c", Search_Field: model.ClusterFieldClusterID, Order_By: model.ClusterFieldClusterID},
			[]string{"c1", "c2", "c3", "c4"}},
		{"gene id is exact",
			model.ClusterSearchRequest{Search_For: "g2a", Search_Field: model.ClusterFieldGeneID, Order_By: model.ClusterFieldClusterID},
			[]string{"c1"}},
		{"gene id outside genome filter",
			model.ClusterSearchRequest{Search_For: "g2a", Search_Field: model.ClusterFieldGeneID, Order_By: model.ClusterFieldClusterID, Genome_IDs: []string{"G1"}},
			[]string{}},
		{"no match",
			model.ClusterSearchRequest{Search_For: "zzz", Search_Field: model.ClusterFieldFunction, Order_By: model.ClusterFieldClusterID},
			[]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.Page, tt.req.Page_Size = 1, 10
			got, err := db.Search(ctx, tt.req)
			if err != nil || !reflect.DeepEqual(clusterIDs(got), tt.want) {
				t.Errorf("Search() = %v, %v; want %v", clusterIDs(got), err, tt.want)
			}
			n, err := db.CountSearch(ctx, tt.req)
			if err != nil || n != len(tt.want) {
				t.Errorf("CountSearch() = %d, %v; want %d", n, err, len(tt.want))
			}
		})
	}

	t.Run("paging", func(t *testing.T) {
		req := model.ClusterSearchRequest{Search_For: "c", Search_Field: model.ClusterFieldClusterID,
			Order_By: model.ClusterFieldClusterID, Page: 2, Page_Size: 3}
		got, err := db.Search(ctx, req)
		if err != nil || !reflect.DeepEqual(clusterIDs(got), []string{"c4"}) {
			t.Errorf("page 2 = %v, %v", clusterIDs(got), err)
		}
	})

	t.Run("invalid field", func(t *testing.T) {
		req := model.ClusterSearchRequest{Search_For: "x", Search_Field: model.ClusterFieldTODO, Order_By: model.ClusterFieldClusterID, Page: 1, Page_Size: 10}
		if _, err := db.Search(ctx, req); !errors.Is(err, ErrInvalidSearch) {
			t.Errorf("Search() error = %v, want ErrInvalidSearch", err)
		}
		if _, err := db.CountSearch(ctx, req); !errors.Is(err, ErrInvalidSearch) {
			t.Errorf("CountSearch() error = %v, want ErrInvalidSearch", err)
		}
		req.Search_Field, req.Order_By = model.ClusterFieldFunction, model.ClusterFieldGeneID
		if _, err := db.Search(ctx, req); !errors.Is(err, ErrInvalidSearch) {
			t.Errorf("Search(order by gene_id) error = %v, want ErrInvalidSearch", err)
		}
	})
}

func TestCountClusters(t *testing.T) {
	n, err := New(testdb.New(t)).CountClusters(ctx)
	if err != nil || n != 4 {
		t.Errorf("CountClusters() = %d, %v; want 4", n, err)
	}
}

// Temp tables are per connection; sequential and concurrent reads must not
// see each other's leftovers.
func TestConcurrentPages(t *testing.T) {
	db := New(testdb.New(t))
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		go func() {
			_, err := db.MainPage(ctx, model.ClusterSearchRequest{Page: 1, Page_Size: 2, Genome_IDs: []string{"G1"}})
			errs <- err
		}()
	}
	for i := 0; i < 20; i++ {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}

func TestSortOrder(t *testing.T) {
	conn := testdb.New(t)
	// Same function as c2, to check the cluster_id tie-breaker.
	if _, err := conn.Exec(`INSERT INTO gene_clusters (cluster_id, cog_id, function_description, expected_length)
		VALUES ('c5', 'COG0000', 'Kinase', 100)`); err != nil {
		t.Fatal(err)
	}
	db := New(conn)
	tests := []struct {
		field model.ClusterField
		dir   string
		want  []string
	}{
		{model.ClusterFieldClusterID, "asc", []string{"c1", "c2", "c3", "c4", "c5"}},
		{model.ClusterFieldClusterID, "desc", []string{"c5", "c4", "c3", "c2", "c1"}},
		{model.ClusterFieldFunction, "asc", []string{"c4", "c1", "c3", "c2", "c5"}},
		{model.ClusterFieldFunction, "desc", []string{"c5", "c2", "c3", "c1", "c4"}},
		{model.ClusterFieldCOGID, "desc", []string{"c4", "c3", "c2", "c1", "c5"}},
		{model.ClusterFieldCOGID, "", []string{"c5", "c1", "c2", "c3", "c4"}}, // empty = ascending
	}
	for _, tt := range tests {
		req := model.ClusterSearchRequest{Order_By: tt.field, Order_Dir: tt.dir, Page: 1, Page_Size: 10}
		main, err := db.MainPage(ctx, req)
		if err != nil || !reflect.DeepEqual(clusterIDs(main), tt.want) {
			t.Errorf("MainPage(%s %s) = %v, %v; want %v", tt.field, tt.dir, clusterIDs(main), err, tt.want)
		}
		req.Search_For, req.Search_Field = "c", model.ClusterFieldClusterID
		found, err := db.Search(ctx, req)
		if err != nil || !reflect.DeepEqual(clusterIDs(found), tt.want) {
			t.Errorf("Search(%s %s) = %v, %v; want %v", tt.field, tt.dir, clusterIDs(found), err, tt.want)
		}
	}

	// Pages of a sort with ties neither overlap nor skip.
	var paged []string
	for page := 1; page <= 3; page++ {
		got, err := db.MainPage(ctx, model.ClusterSearchRequest{Order_By: model.ClusterFieldFunction, Order_Dir: "desc", Page: page, Page_Size: 2})
		if err != nil {
			t.Fatal(err)
		}
		paged = append(paged, clusterIDs(got)...)
	}
	if want := []string{"c5", "c2", "c3", "c1", "c4"}; !reflect.DeepEqual(paged, want) {
		t.Errorf("pages joined = %v, want %v", paged, want)
	}
}

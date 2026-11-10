package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yumyai/ggtable/pkg/model"
)

func TestClusterTableEscapesMetadata(t *testing.T) {
	c := model.NewCluster(model.ClusterProperty{ClusterID: "c1", FunctionDescription: `<script>alert(1)</script>`})
	c.Genome("G").Genes = []*model.Gene{{GeneID: "single", Region: &model.Region{GenomeID: "G", ContigID: "c", Start: 1, End: 1}}}
	var out bytes.Buffer
	err := RenderClusterTablePage(&out, model.NewGenomeCatalog([]string{"G"}, nil), c)
	if err != nil {
		t.Fatal(err)
	}
	html := out.String()
	if strings.Contains(html, "<script>alert") || !strings.Contains(html, "&lt;script&gt;") {
		t.Fatal("metadata was not escaped")
	}
	if !strings.Contains(html, "(1 bp)") || !strings.Contains(html, "/ 1 genomes") {
		t.Fatal("incorrect lengths or catalog size")
	}
}

func TestClusterTableWithoutGenes(t *testing.T) {
	for _, regionsOnly := range []bool{false, true} {
		c := model.NewCluster(model.ClusterProperty{ClusterID: "empty"})
		if regionsOnly {
			c.Genome("G").Regions = []*model.Region{{GenomeID: "G", Start: 10, End: 1}}
		}
		var out bytes.Buffer
		if err := RenderClusterTablePage(&out, model.GenomeCatalog{}, c); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "Representative gene:") {
			t.Fatal("missing representative rendered")
		}
	}
}

package model

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func testCatalog() GenomeCatalog {
	return NewGenomeCatalog([]string{"G1", "G2", "G3"}, map[string]string{
		"G1": "Genome One", "G2": "Genome <Two>", "G3": "Genome Three",
	})
}

func TestGenomeCatalog_IsACopy(t *testing.T) {
	ids := []string{"G1"}
	names := map[string]string{"G1": "Genome One"}
	c := NewGenomeCatalog(ids, names)
	ids[0], names["G1"] = "X", "changed"
	c.IDs()[0] = "Y"

	if got := c.IDs(); !reflect.DeepEqual(got, []string{"G1"}) {
		t.Errorf("IDs() = %v, want [G1]", got)
	}
	if n, _ := c.Name("G1"); n != "Genome One" {
		t.Errorf("Name(G1) = %q, want Genome One", n)
	}
}

func TestGenomeCatalog_Reorder(t *testing.T) {
	got, unknown := testCatalog().Reorder([]string{"G3", "nope", "G1", "G3"})

	if want := []string{"G3", "G1"}; !reflect.DeepEqual(got.IDs(), want) {
		t.Errorf("IDs() = %v, want %v", got.IDs(), want)
	}
	if want := []string{"nope"}; !reflect.DeepEqual(unknown, want) {
		t.Errorf("unknown = %v, want %v", unknown, want)
	}
	if n, ok := got.Name("G2"); !ok || n != "Genome <Two>" {
		t.Errorf("hidden genomes keep their names, got %q %v", n, ok)
	}
}

func TestRefs(t *testing.T) {
	g := GeneRef{GenomeID: "G1", ContigID: "ctgA", GeneID: "g1a"}
	if g.SeqID() != "G1//ctgA//g1a" {
		t.Errorf("GeneRef.SeqID() = %q", g.SeqID())
	}
	r := RegionRef{GenomeID: "G3", ContigID: "ctgC", Start: 100, End: 900}
	if r.SeqID() != "G3//ctgC" || r.Range() != "100-900" {
		t.Errorf("RegionRef = %q %q", r.SeqID(), r.Range())
	}
	if err := r.Validate(); err != nil {
		t.Errorf("Validate(%v) = %v", r, err)
	}
	for _, bad := range []RegionRef{{Start: 0, End: 10}, {Start: 10, End: 9}} {
		if err := bad.Validate(); err == nil {
			t.Errorf("Validate(%v) = nil, want error", bad)
		}
	}
}

func TestLabelFasta(t *testing.T) {
	in := ">G1//ctgA//g1a\nACGT\n>G9//ctg//x\nTT\n>G1 no separator\nAA\n"
	want := ">Genome One-G1//ctgA//g1a\nACGT\n>G9//ctg//x\nTT\n>G1 no separator\nAA\n"
	if got := LabelFasta([]byte(in), testCatalog()); got != want {
		t.Errorf("LabelFasta() =\n%s\nwant\n%s", got, want)
	}
	if got := LabelFasta(nil, testCatalog()); got != "" {
		t.Errorf("LabelFasta(nil) = %q, want empty", got)
	}
}

func TestClusterRefs(t *testing.T) {
	c := NewCluster(ClusterProperty{ClusterID: "c1"})
	for _, id := range []string{"G2", "G1"} {
		g := c.Genome(id)
		g.Genes = append(g.Genes, &Gene{GeneID: "g-" + id, Region: &Region{GenomeID: id, ContigID: "ctg"}})
	}
	c.Genome("G3").Regions = []*Region{
		{GenomeID: "G3", ContigID: "ctgC", Start: 100, End: 900},
		{GenomeID: "G3", ContigID: "ctgD"}, // no location
	}

	wantGenes := []GeneRef{{"G1", "ctg", "g-G1"}, {"G2", "ctg", "g-G2"}}
	if got := c.GeneRefs(); !reflect.DeepEqual(got, wantGenes) {
		t.Errorf("GeneRefs() = %v, want %v", got, wantGenes)
	}
	wantRegions := []RegionRef{{"G3", "ctgC", 100, 900}}
	if got := c.RegionRefs(); !reflect.DeepEqual(got, wantRegions) {
		t.Errorf("RegionRefs() = %v, want %v", got, wantRegions)
	}
}

func TestBlastSearchRequest_Validate(t *testing.T) {
	q, err := BlastSearchRequest{BlastType: "blastp", Sequence: "\n  >q\n\n MKV \n"}.Validate()
	if err != nil || q != ">q\nMKV" {
		t.Errorf("Validate() = %q, %v; want \">q\\nMKV\"", q, err)
	}
	if _, err := (BlastSearchRequest{BlastType: "tblastx", Sequence: "A"}).Validate(); err == nil {
		t.Error("Validate() accepted tblastx")
	}
	if _, err := (BlastSearchRequest{BlastType: "blastn", Sequence: " \n "}).Validate(); err == nil {
		t.Error("Validate() accepted an empty sequence")
	}
}

func TestAnnotateBlastReport(t *testing.T) {
	report := strings.Join([]string{
		"<b>Query=</b> q",
		"                                                Score     E",
		"Sequences producing significant alignments:                          (Bits)  Value",
		"",
		"G2//ctgB//g2a                                         150     1e-40",
		"G9//ctgZ//zz                                          50      1e-5",
		"",
		">G2//ctgB//g2a<a name=G2//ctgB//g2a></a> unnamed protein product",
		"Length=80",
		">G9//ctgZ//zz<a name=G9//ctgZ//zz></a> unnamed protein product",
		"  Database: test",
	}, "\n") + "\n"

	got, err := AnnotateBlastReport(strings.NewReader(report), testCatalog())
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(got, "\n")

	if !strings.HasPrefix(lines[4], "Genome &lt;Two&gt;-G2//ctgB//g2a ") || !strings.Contains(lines[4], "150") {
		t.Errorf("TOC line not annotated: %q", lines[4])
	}
	if lines[5] != "G9//ctgZ//zz                                          50      1e-5" {
		t.Errorf("unknown genome TOC line changed: %q", lines[5])
	}
	wantAlign := `>Genome &lt;Two&gt;-G2//ctgB//g2a<a name=G2//ctgB//g2a></a> unnamed protein product <a href="/cluster/heatmap/G2/ctgB/g2a">View in gene table</a>`
	if lines[7] != wantAlign {
		t.Errorf("alignment header =\n%s\nwant\n%s", lines[7], wantAlign)
	}
	if !strings.HasPrefix(lines[9], ">G9//ctgZ//zz<a name") || strings.Contains(lines[9], "href") {
		t.Errorf("unknown genome alignment changed: %q", lines[9])
	}
}

func TestBlastManager_KeepsLatestJobs(t *testing.T) {
	m := NewBlastManager()
	var ids []string
	for i := 0; i < maxJobs+3; i++ {
		ids = append(ids, m.NewJob("blastp").ID)
	}
	for i, id := range ids {
		_, ok := m.GetJob(id)
		if want := i >= 3; ok != want {
			t.Errorf("job %d present = %v, want %v", i, ok, want)
		}
	}
}

func TestBlastManager_Lifecycle(t *testing.T) {
	m := NewBlastManager()
	job := m.NewJob("blastn")
	if job.Status != BlastJobQueued {
		t.Fatalf("new job status = %s", job.Status)
	}
	m.SetRunning(job.ID)
	m.FailJob(job.ID, errors.New("boom"))
	got, _ := m.GetJob(job.ID)
	if got.Status != BlastJobFailed || got.Error != "boom" {
		t.Errorf("job = %+v, want failed with boom", got)
	}
	m.CompleteJob("missing", "x") // unknown IDs are ignored
}

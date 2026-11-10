package model

import (
	"context"
	"errors"
	"testing"
)

type clusterFixture struct {
	ClusterStore
	cluster *Cluster
	err     error
}

func (f clusterFixture) Cluster(context.Context, string) (*Cluster, error) { return f.cluster, f.err }

type sequenceFixture struct {
	SequenceStore
	protein            bool
	genes              []GeneRef
	regions            []RegionRef
	geneErr, regionErr error
}

func (f *sequenceFixture) Genes(_ context.Context, refs []GeneRef, protein bool) ([]byte, error) {
	f.genes = refs
	f.protein = protein
	return []byte(">gene\nAC\n"), f.geneErr
}
func (f *sequenceFixture) Regions(_ context.Context, refs []RegionRef) ([]byte, error) {
	f.regions = refs
	return []byte(">region\nGT\n"), f.regionErr
}

func TestClusterFASTA(t *testing.T) {
	c := NewCluster(ClusterProperty{})
	c.Genome("G").Genes = []*Gene{{GeneID: "g", Region: &Region{GenomeID: "G", ContigID: "ctg", Start: 1, End: 2}}}
	c.Genome("G").Regions = []*Region{{GenomeID: "G", ContigID: "ctg", Start: 3, End: 4}, {GenomeID: "G", ContigID: "unknown"}}
	for _, protein := range []bool{true, false} {
		seqs := &sequenceFixture{}
		got, err := ClusterFASTA(context.Background(), clusterFixture{cluster: c}, seqs, "c", protein)
		want := ">gene\nAC\n"
		if !protein {
			want += ">region\nGT\n"
		}
		if err != nil || string(got) != want {
			t.Fatalf("protein=%v: %q, %v", protein, got, err)
		}
		if seqs.protein != protein || len(seqs.genes) != 1 {
			t.Fatal("gene selection lost")
		}
		if protein && seqs.regions != nil {
			t.Fatal("protein download fetched regions")
		}
		if !protein && len(seqs.regions) != 1 {
			t.Fatal("unlocated region was not skipped")
		}
	}
	boom := errors.New("storage failed")
	for _, tc := range []struct {
		name     string
		clusters clusterFixture
		seqs     sequenceFixture
		want     error
	}{
		{"missing cluster", clusterFixture{err: ErrNotFound}, sequenceFixture{}, ErrNotFound},
		{"genes", clusterFixture{cluster: c}, sequenceFixture{geneErr: boom}, boom},
		{"regions", clusterFixture{cluster: c}, sequenceFixture{regionErr: boom}, boom},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ClusterFASTA(context.Background(), tc.clusters, &tc.seqs, "c", false)
			if got != nil || !errors.Is(err, tc.want) {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

func TestClusterSummary(t *testing.T) {
	c := NewCluster(ClusterProperty{})
	if c.Summary().RepresentativeGene != nil {
		t.Fatal("empty cluster has representative")
	}
	c.Genome("B").Genes = []*Gene{{GeneID: "reverse", Region: &Region{Start: 10, End: 1}}}
	c.Genome("A").Genes = []*Gene{{GeneID: "forward", Region: &Region{Start: 1, End: 10}}, {GeneID: "unlocated"}}
	c.Genome("A").Regions = []*Region{{Start: 3, End: 4}}
	summary := c.Summary()
	if summary.TotalGenes != 3 || summary.TotalRegions != 1 || summary.RepresentativeGene.GeneID != "forward" {
		t.Fatalf("%+v", summary)
	}
	for _, r := range []Region{{Start: 1, End: 10}, {Start: 10, End: 1}} {
		if r.Length() != 10 {
			t.Fatal(r.Length())
		}
	}
	if (Region{Start: 1, End: 1}).Length() != 1 {
		t.Fatal("single base length")
	}
}

func TestNewJobReturnsSnapshot(t *testing.T) {
	m := NewBlastManager()
	job := m.NewJob("blastp")
	job.Status = BlastJobFailed
	stored, _ := m.GetJob(job.ID)
	if stored.Status != BlastJobQueued {
		t.Fatal("caller modified stored job")
	}
	m.SetRunning(job.ID)
	if job.Status != BlastJobFailed {
		t.Fatal("returned snapshot mutated")
	}
}

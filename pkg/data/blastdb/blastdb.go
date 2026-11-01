// Package blastdb reads sequences from, and runs searches against, the BLAST
// databases built by ggprep/steps/build_blastdb.sh (or ggorigin). It shells
// out to NCBI BLAST+ (blastdbcmd, blastn, blastp), which must be on PATH.
package blastdb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/yumyai/ggtable/pkg/model"
)

// DB holds the paths (without extension) of the three BLAST databases.
type DB struct {
	ProtDB   string // gene proteins, IDs GENOME//CONTIG//GENE
	NuclDB   string // gene nucleotides, IDs GENOME//CONTIG//GENE
	GenomeDB string // contigs, IDs GENOME//CONTIG
}

// Open checks that BLAST+ is installed and that each database has an index
// file, then returns the DB.
func Open(protDB, nuclDB, genomeDB string) (*DB, error) {
	if _, err := exec.LookPath("blastdbcmd"); err != nil {
		return nil, errors.New("blastdbcmd not found on PATH: install NCBI BLAST+ (conda install -c bioconda blast)")
	}
	var problems []string
	for _, c := range []struct{ path, kind string }{{protDB, "p"}, {nuclDB, "n"}, {genomeDB, "n"}} {
		if !hasIndex(c.path, c.kind) {
			problems = append(problems, fmt.Sprintf("%s (no .%sin/.%sal file)", c.path, c.kind, c.kind))
		}
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("missing BLAST databases: %s; build them with ggprep (make blastdb) or ggorigin", strings.Join(problems, ", "))
	}
	return &DB{ProtDB: protDB, NuclDB: nuclDB, GenomeDB: genomeDB}, nil
}

// hasIndex reports whether a BLAST DB exists: a single volume (.pin/.nin), a
// multi-volume alias (.pal/.nal), or a first volume (.00.pin/.00.nin).
func hasIndex(base, kind string) bool {
	for _, suffix := range []string{"." + kind + "in", "." + kind + "al", ".00." + kind + "in"} {
		if _, err := os.Stat(base + suffix); err == nil {
			return true
		}
	}
	return false
}

func (db *DB) geneDB(prot bool) string {
	if prot {
		return db.ProtDB
	}
	return db.NuclDB
}

// Gene returns the FASTA of one gene, protein or nucleotide.
func (db *DB) Gene(ctx context.Context, ref model.GeneRef, prot bool) ([]byte, error) {
	out, err := run(ctx, nil, "blastdbcmd", "-db", db.geneDB(prot), "-entry", ref.SeqID())
	if err != nil {
		return nil, fmt.Errorf("fetch gene %s: %w", ref.SeqID(), err)
	}
	return out, nil
}

// Region returns the FASTA of a stretch of a contig.
func (db *DB) Region(ctx context.Context, ref model.RegionRef) ([]byte, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	out, err := run(ctx, nil, "blastdbcmd", "-db", db.GenomeDB, "-entry", ref.SeqID(), "-range", ref.Range())
	if err != nil {
		return nil, fmt.Errorf("fetch region %s:%s: %w", ref.SeqID(), ref.Range(), err)
	}
	return out, nil
}

// Genes returns the FASTA of many genes in one blastdbcmd call.
func (db *DB) Genes(ctx context.Context, refs []model.GeneRef, prot bool) ([]byte, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	var batch strings.Builder
	for _, r := range refs {
		batch.WriteString(r.SeqID() + "\n")
	}
	out, err := run(ctx, strings.NewReader(batch.String()), "blastdbcmd", "-db", db.geneDB(prot), "-entry_batch", "-")
	if err != nil {
		return nil, fmt.Errorf("fetch %d genes: %w", len(refs), err)
	}
	return out, nil
}

// Regions returns the FASTA of many contig stretches in one blastdbcmd call.
func (db *DB) Regions(ctx context.Context, refs []model.RegionRef) ([]byte, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	var batch strings.Builder
	for _, r := range refs {
		if err := r.Validate(); err != nil {
			return nil, err
		}
		// -entry_batch line format: "ID START-END"
		batch.WriteString(r.SeqID() + " " + r.Range() + "\n")
	}
	out, err := run(ctx, strings.NewReader(batch.String()), "blastdbcmd", "-db", db.GenomeDB, "-entry_batch", "-")
	if err != nil {
		return nil, fmt.Errorf("fetch %d regions: %w", len(refs), err)
	}
	return out, nil
}

// Search runs blastn against the gene nucleotide DB or blastp against the
// gene protein DB and returns the raw -html report.
func (db *DB) Search(ctx context.Context, program, queryFasta string) ([]byte, error) {
	var target string
	switch program {
	case "blastn":
		target = db.NuclDB
	case "blastp":
		target = db.ProtDB
	default:
		return nil, fmt.Errorf("unsupported BLAST program %q: use blastn or blastp", program)
	}
	// 500 descriptions and alignments, matching max_target_seqs' default.
	out, err := run(ctx, strings.NewReader(queryFasta), program,
		"-db", target, "-html", "-num_descriptions", "500", "-num_alignments", "500")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", program, err)
	}
	return out, nil
}

// CheckGene fetches ref from both gene databases, to confirm at startup that
// the BLAST IDs match the SQLite tables.
func (db *DB) CheckGene(ctx context.Context, ref model.GeneRef) error {
	for _, prot := range []bool{true, false} {
		if _, err := db.Gene(ctx, ref, prot); err != nil {
			return fmt.Errorf("gene %s from gene_matches is not in BLAST DB %s; the BLAST DBs and gene_table.db "+
				"are from different builds, or the sequence IDs are not GENOME%sCONTIG%sGENE: %w",
				ref.SeqID(), db.geneDB(prot), model.SeqIDSep, model.SeqIDSep, err)
		}
	}
	return nil
}

// run executes a BLAST+ command and returns stdout; on failure the error
// includes stderr, which is where BLAST+ explains what went wrong.
func run(ctx context.Context, stdin *strings.Reader, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%s failed: %w: %s", name, err, msg)
		}
		return nil, fmt.Errorf("%s failed: %w", name, err)
	}
	return stdout.Bytes(), nil
}

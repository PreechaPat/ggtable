package handler

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/yumyai/ggtable/pkg/data/blastdb"
	"github.com/yumyai/ggtable/pkg/model"
)

// helper to create a fake 'blastdbcmd' executable that prints a fixed FASTA
func createFakeBlastdbcmd(t *testing.T, dir string, fasta string) string {
	t.Helper()

	name := "blastdbcmd"
	if runtime.GOOS == "windows" {
		name += ".bat"
	}
	path := filepath.Join(dir, name)

	var content string
	if runtime.GOOS == "windows" {
		// minimal .bat that echoes lines
		lines := []string{"@echo off"}
		for _, ln := range strings.Split(fasta, "\n") {
			if ln == "" {
				lines = append(lines, "echo.")
			} else {
				lines = append(lines, "echo "+ln)
			}
		}
		content = strings.Join(lines, "\r\n") + "\r\n"
	} else {
		content = "#!/usr/bin/env bash\n" +
			"cat <<'EOF'\n" + fasta + "\nEOF\n"
	}

	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake blastdbcmd: %v", err)
	}
	// Ensure executable bit on non-windows
	_ = os.Chmod(path, fs.FileMode(0o755))
	return path
}

// prepend a directory to PATH for this process
func prependPath(t *testing.T, dir string) (restore func()) {
	t.Helper()
	old := os.Getenv("PATH")
	newPath := dir
	if old != "" {
		newPath = dir + string(os.PathListSeparator) + old
	}
	if err := os.Setenv("PATH", newPath); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	return func() { _ = os.Setenv("PATH", old) }
}

func TestGetGeneSequenceHandler_MockBlastdbcmd(t *testing.T) {
	// Arrange fake blastdbcmd
	tmp := t.TempDir()
	fastaOut := ">KCB09//ctg//KCB09_00123\nACGT\n"
	createFakeBlastdbcmd(t, tmp, fastaOut)
	restore := prependPath(t, tmp)
	t.Cleanup(restore)

	appConfig := &AppContext{
		Seqs: &blastdb.DB{
			ProtDB:   filepath.Join(tmp, "prot"),
			NuclDB:   filepath.Join(tmp, "nucl"),
			GenomeDB: filepath.Join(tmp, "genome"),
		},
		Genomes: model.NewGenomeCatalog([]string{"KCB09"}, map[string]string{"KCB09": "TestGenome"}),
	}

	// Build request
	q := url.Values{}
	q.Set("genome_id", "KCB09")
	q.Set("contig_id", "ctg")
	q.Set("gene_id", "KCB09_00123")
	q.Set("is_prot", "false")

	req := httptest.NewRequest(http.MethodGet, "/sequence/by-gene?"+q.Encode(), nil)
	rr := httptest.NewRecorder()

	// Act
	appConfig.GetGeneSequenceHandler(rr, req)

	// Assert
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	got := rr.Body.String()
	// Header should be rewritten to include the genome name prefix
	wantHeader := ">TestGenome-KCB09//ctg//KCB09_00123"
	if !strings.HasPrefix(got, wantHeader) {
		t.Fatalf("unexpected header. got %q, want prefix %q", got, wantHeader)
	}
	if !strings.Contains(got, "ACGT") {
		t.Fatalf("missing sequence body. got %q", got)
	}
}

func TestGetRegionSequenceHandler_InvalidParams(t *testing.T) {
	appConfig := &AppContext{}

	// start is invalid; end is fine
	req := httptest.NewRequest(http.MethodGet, "/sequence/by-region?genome_id=G&contig_id=C&start=oops&end=20", nil)
	rr := httptest.NewRecorder()

	appConfig.GetRegionSequenceHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid params, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Invalid start value") {
		t.Fatalf("expected error about start value, got %q", rr.Body.String())
	}
}

func TestGetRegionSequenceHandler_NoLocation(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/sequence/by-region?genome_id=G&contig_id=C&start=0&end=0", nil)
	rr := httptest.NewRecorder()
	(&AppContext{}).GetRegionSequenceHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a region without location, got %d", rr.Code)
	}
}

func TestGetGeneSequenceHandler_BadIsProt(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/sequence/by-gene?genome_id=G&contig_id=C&gene_id=g&is_prot=maybe", nil)
	rr := httptest.NewRecorder()
	(&AppContext{}).GetGeneSequenceHandler(rr, req) // must not reach the nil store
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

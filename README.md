# Pins Gene Table v3 (ggserve)

Pins Gene Table is a web database for comparing gene content across *Pythium insidiosum* genomes. For each gene cluster it shows which genomes carry it, how many copies, and how complete they are. You can also BLAST against the gene sets and download sequences. The app works for any set of genomes, not only *P. insidiosum*.

This repo is v3, the newest version: a full rewrite of Pins Gene Table v2.0.

- Live instance: https://dt2lrkh17no4h.cloudfront.net/
- Paper (v2.0): [Pins Gene Table v2.0: An Online Genome Database of 37 *Pythium insidiosum* Strains for Gene Content Exploration and Phylogenomic Analysis](https://www.mdpi.com/2309-608X/10/2/112), *J. Fungi* 2024, 10(2), 112. doi:[10.3390/jof10020112](https://doi.org/10.3390/jof10020112)

This README is for people who want to run it on their own data. In short, you need:

1. A data directory with one SQLite database and three BLAST databases ([Your data](#2-your-data)).
2. Docker, or Go ≥ 1.25 plus NCBI BLAST+ on `PATH`.

The data itself is not in this repo. It is produced by a separate preparation pipeline (`ggprep`).

## 1. Run it

With Docker (the image already includes BLAST+):

```bash
docker run -p 8080:8080 \
  --mount type=bind,source="$(pwd)/data",target=/data \
  --env-file pins.env \
  ghcr.io/preechapat/ggtable:latest
```

Open http://localhost:8080. The image has no settings built in; pass them at run time with `--env-file` (here the PINS Table's `pins.env`, or your own file) or `-e GGTITLE=...`. Leave values unquoted in that file: Docker keeps quotes as part of the value. To build the image yourself instead, run `docker build . -t ggtable` in the repo root and use `ggtable` as the image name.

Without Docker, from the repo root:

```bash
go run . -data /path/to/data -v   # -v = debug logging
```

The JS/CSS in `static/` is compiled into the binary, so a built `ggtable` runs from any directory: `./ggtable -data /path/to/data`.

## 2. Your data

### Layout

```
data/
└── db/
    ├── gene_table.db                # SQLite
    └── blastdb/
        ├── genetable_genes_prot.*   # protein seqs of every gene     (makeblastdb -dbtype prot)
        ├── genetable_genes_nucl.*   # nucleotide seqs of every gene  (makeblastdb -dbtype nucl)
        └── genetable_genomes.*      # all contigs of all genomes     (makeblastdb -dbtype nucl)
```

These names are fixed. If `gene_table.db` is missing, SQLite creates an empty file and startup fails with an error about `genome_info`.

### Sequence IDs

Build every BLAST database with `-parse_seqids`, and use `//` to separate the parts of each ID:

| Database            | ID format              | Example                            |
|---------------------|------------------------|------------------------------------|
| `genetable_genes_*` | `GENOME//CONTIG//GENE` | `P16PC//contig000001//P16PC_00001` |
| `genetable_genomes` | `GENOME//CONTIG`       | `P16PC//contig000001`              |

The parts must match `genome_id`, `contig_id`, and `gene_id` in SQLite. Don't use `|` (BLAST treats it as its own ID syntax) or `-`/`_` (they appear inside real IDs). To check a database:

```bash
blastdbcmd -db data/db/blastdb/genetable_genes_prot -entry all -outfmt '%a' | head
```

### SQLite tables

Only the columns the app reads are listed.

| Table            | Columns | Meaning |
|------------------|---------|---------|
| `genome_info`    | `genome_id`, `genome_fullname` | One row per genome. Used for column headers. |
| `gene_clusters`  | `cluster_id`, `cog_id`, `representative_gene`, `function_description`, `expected_length` | One row per cluster. |
| `gene_matches`   | `cluster_id`, `genome_id`, `contig_id`, `gene_id` | Which annotated gene belongs to which cluster. |
| `gene_info`      | `genome_id`, `gene_id`, `contig_id`, `start_location`, `end_location`, `gene_length`, `description` | Gene coordinates. Completeness = `gene_length / expected_length`. |
| `region_matches` | `cluster_id`, `genome_id`, `contig_id`, `start_location`, `end_location` | Unannotated regions that match a cluster (partial hits). |

Add indexes on `*_matches.cluster_id`, `*_matches.genome_id`, `gene_matches.gene_id`, `gene_info.gene_id`, and `gene_info.genome_id`. Without them, pages get slow at around 100k clusters.

## 3. Configure

Each setting can come from a `.env` file, an environment variable, or a flag. Later ones win, in that order. Start from `cp .env.example .env`; `.env` is gitignored. `pins.env` holds the settings for the PINS Table instance: copy it to `.env`, or pass it to Docker with `--env-file pins.env`. Docker images contain no `.env`, so all settings come from `docker run`.

| Env var        | Flag        | Default        | Purpose |
|----------------|-------------|----------------|---------|
| `GGTABLE_DATA` | `-data`     | `./data`       | Data directory |
| `GGTABLE_ADDR` | `-addr`     | `0.0.0.0:8080` | Listen address |
| `GGTITLE`      | `-title`    | `Gene Table v3` | Header title on the heatmap pages (`pins.env` sets `Pins Gene Table v3`) |
| `GGSUBTITLE`   | `-subtitle` | empty (hidden) | Header line under the title |
| `GGTABLE_STATIC` | `-static` | empty (embedded) | Serve `/static/` from this directory instead of the copy built into the binary. For editing JS/CSS without rebuilding, e.g. `-static ./static`. |
| `GGSORTED`     | none        | empty          | Comma-separated `genome_id`s: which genomes appear in the heatmap, and in what order. Set it: without it the column order changes between restarts. |

Old keys (`GGTABLE_DB_SQLITE`, `GGSEQDB`, `GGSEP`) are ignored.

## 4. Before you expose it publicly

- There is no authentication or rate limiting, and each BLAST request starts a BLAST+ process. Put it behind a reverse proxy or CDN.
- The cluster-table and BLAST pages don't HTML-escape their content. Only load data you trust.
- BLAST jobs live in memory, and only the latest 5 are kept. A restart loses them.

## Linking and scripting

Most routes return HTML. The useful ones for linking from elsewhere or scripting:

| Route | Returns |
|-------|---------|
| `GET /search?search=...&search_by=function\|cog_id\|cluster_id\|gene_id` | Filtered heatmap |
| `GET /cluster/table/{cluster_id}` | Detail table for one cluster |
| `GET /sequence/by-gene?genome_id=&contig_id=&gene_id=&is_prot=` | FASTA |
| `GET /sequence/by-region?genome_id=&contig_id=&start=&end=` | FASTA |
| `GET /sequence/by-cluster?cluster_id=&is_prot=` | Multi-FASTA of the whole cluster |
| `POST /blast` | JSON `{"blast_type":"blastn"\|"blastp","sequence":"<fasta>"}`. With `Accept: application/json` it returns `202 {"job_id":...}`; poll `GET /blast/{job_id}` |
| `GET /api/v1/health` | Health check |

There is no JSON data API yet. The full route list is in `NewRouter` in `main.go`.

## Changing the code

Stack: Go `net/http` (no framework), pure-Go SQLite (`modernc.org/sqlite`, so `CGO_ENABLED=0` works), `zap` logging, server-rendered Go templates, a little jQuery.

```
main.go         config, wiring, routes
pkg/handler/    HTTP handlers: parse query → call data/model → render
pkg/model/      domain types and pure rules (no I/O): genome catalog, sequence IDs,
                FASTA labelling, BLAST report rewriting, in-memory BLAST jobs
pkg/data/       opens and checks the data directory (fixed layout, see above)
  sqldb/        SQLite queries: clusters, search, genomes
  blastdb/      BLAST+ wrappers: blastdbcmd sequence fetch, blastn/blastp search
pkg/render/     HTML templates (Go string constants), heatmap colours
internal/testdb/ small SQLite + BLAST fixtures for tests
static/         JS/CSS served at /static/, embedded into the binary
```

Common changes:

- New search field: add a `ClusterField` and extend `ParseClusterField` in `pkg/model/search.go` and `whereExpr` / `orderByExpr` in `pkg/data/sqldb/search.go`, then add the option to the form in `pkg/render/render_ggtable.go`.
- New heatmap colouring: write a `CellColorFunc` in `pkg/render/render_ggtable.go`, register it in `templateFuncMap`, and accept it in the handlers' `color_by` parsing.
- New route: add a method on `*handler.AppContext` and register it in `NewRouter`.

Build, test, release:

```bash
go build ./... && go vet ./... && go test ./...
air -c air.ml                              # live reload during development
docker build . -t ggtable                   # local image, uncommitted changes included
scripts/publish-image.sh --version 3.0.5   # build + push to GHCR (or tag v3.0.5 first and omit --version)
```

The dev container (`.devcontainer/`, `docker-compose.yml`) expects the data directory next to the repo, at `../data`. It mounts the repo's parent folder at `/workspaces` and `../data` at `/data`.

`publish-image.sh` refuses to run with uncommitted changes and needs a `gh` token with `write:packages`. The version is compiled into the binary and printed in the startup log.

Tests build their own small fixtures (`internal/testdb`). Tests that touch the BLAST databases need BLAST+ on `PATH` (e.g. `conda activate bio`) and skip without it.

At startup `pkg/data` checks the data directory (layout, SQLite tables and columns, BLAST indexes, and that one gene from `gene_matches` can be fetched from the BLAST DBs) and exits with the reason if anything is off.

## Contributing, license, contact

Issues and PRs are welcome at https://github.com/PreechaPat/ggtable. Please use [Conventional Commits](https://www.conventionalcommits.org/) and run `go vet ./...` first.

MIT licensed, see [LICENSE](LICENSE). Maintainer: preecha.pat@mahidol.ac.th.

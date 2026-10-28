# Genome Table v.3 (ggserve)

Genome Table v.3 is a web viewer for comparing gene content across genomes. It is a full rewrite of the *P. insidiosum* genome database (v.2). For each gene cluster it shows which genomes carry it, how many copies, and how complete they are. You can also BLAST against the gene sets and download sequences.

Live instance: https://dt2lrkh17no4h.cloudfront.net/

This README is written for people who want to deploy, extend, or load their own data into the app.

- Stack: Go 1.25 (`net/http` only, no web framework), SQLite through `modernc.org/sqlite` (pure Go, so `CGO_ENABLED=0` works), `zap` for logging, server-rendered Go templates, plus a little jQuery in `static/`.
- Runtime dependency: NCBI BLAST+ (`blastn`, `blastp`, `blastdbcmd`) has to be on `PATH`. The app calls these binaries as subprocesses for BLAST searches and for every sequence fetch.
- Data is not in this repo. The app reads a prebuilt SQLite database and three BLAST databases from a data directory (see [Data contract](#data-contract)).

## Quick start

### Docker (prebuilt image)

```bash
# ./data must contain db/gene_table.db and db/blastdb/* (see Data contract)
docker run -p 8080:8080 \
  --mount type=bind,source="$(pwd)/data",target=/data \
  docker.io/preechapat/ggtable:3.0.4
```

Then open http://localhost:8080. The image already includes BLAST+ and sets `GGTABLE_DATA=/data`.

### Docker (from source)

```bash
git clone https://github.com/PreechaPat/ggtable
cd ggtable
docker build . -t preechapat/ggtable:3.0.0-dev
docker run -p 8080:8080 \
  --mount type=bind,source="$(pwd)/data",target=/data \
  preechapat/ggtable:3.0.0-dev
```

### Native (for development)

Requirements: Go ≥ 1.25 and BLAST+ on `PATH` (for example `conda install -c bioconda blast`).

```bash
go run . -data ../data -v        # -v turns on debug logging
```

Run it from the repo root, because static files are served from `./static` relative to the working directory. For live reload, use [air](https://github.com/air-verse/air) with the bundled config: `air -c air.ml`.

## Configuration

Settings are read in this order, and later sources override earlier ones: `.env` file (optional), then environment variables, then command-line flags.

| Env var        | Flag        | Default        | Purpose |
|----------------|-------------|----------------|---------|
| `GGTABLE_DATA` | `-data`     | `./data`       | Data directory root |
| `GGTABLE_ADDR` | `-addr`     | `0.0.0.0:8080` | Listen address |
| `GGTITLE`      | `-title`    | empty          | Page title |
| `GGSUBTITLE`   | `-subtitle` | empty          | Page subtitle |
| `GGSORTED`     | none        | empty          | Comma-separated `genome_id` list that sets the column order (and which genomes appear) in the heatmap. If unset, the order is whatever Go map iteration gives, which is not stable between runs. |
| none           | `-v`        | off            | Debug logging |

Some keys may exist in an old `.env` (`GGTABLE_DB_SQLITE`, `GGSEQDB`, `GGSEP`). The current code ignores them.

## Data contract

The app expects this layout (see `../data` in the development checkout):

```
$GGTABLE_DATA/
└── db/
    ├── gene_table.db                   # SQLite, opened in WAL mode
    └── blastdb/
        ├── genetable_genes_prot.*      # protein seqs of every gene   (makeblastdb -dbtype prot)
        ├── genetable_genes_nucl.*      # nucleotide seqs of every gene (makeblastdb -dbtype nucl)
        └── genetable_genomes.*         # all contigs of all genomes    (makeblastdb -dbtype nucl)
```

The paths are hard-coded in `main.go`. If `gene_table.db` is missing, SQLite silently creates an empty file, and startup then fails on `genome_info`.

### Sequence ID convention

The BLAST databases must be built with `-parse_seqids`. Sequence IDs use `//` as the separator:

| Database               | ID format                  | Example                            |
|------------------------|----------------------------|------------------------------------|
| `genetable_genes_*`    | `GENOME//CONTIG//GENE`     | `P16PC//contig000001//P16PC_00001` |
| `genetable_genomes`    | `GENOME//CONTIG`           | `P16PC//contig000001`              |

`|` cannot be used as the separator because BLAST treats it as part of its own ID syntax. Some genome and gene IDs contain `-` or `_`, so those don't work either. The three parts must match `genome_id`, `contig_id`, and `gene_id` in SQLite. The BLAST result page parses these IDs back out to rename hits and link them to the gene table (`pkg/model/blast.go`).

You can check a database with:

```bash
blastdbcmd -db data/db/blastdb/genetable_genes_prot -entry all -outfmt '%a' | head
```

### SQLite schema

These are the tables the app uses. Columns that the code never reads are left out.

| Table            | Key columns | Role |
|------------------|-------------|------|
| `genome_info`    | `genome_id`, `genome_fullname` | One row per genome. Loaded into memory at startup and used for column headers and BLAST hit names. |
| `gene_clusters`  | `cluster_id`, `cog_id`, `representative_gene`, `function_description`, `expected_length` | One row per cluster. Searchable by function, COG, and cluster ID. |
| `gene_matches`   | `cluster_id`, `genome_id`, `contig_id`, `gene_id` | Links an annotated gene to a cluster. |
| `gene_info`      | `genome_id`, `gene_id`, `contig_id`, `start_location`, `end_location`, `gene_length`, `description` | Gene coordinates. Completeness is computed as `gene_length / expected_length`. |
| `region_matches` | `cluster_id`, `genome_id`, `contig_id`, `start_location`, `end_location` | Unannotated genomic regions that match a cluster (partial hits). |
| `contig_info`    | `genome_id`, `contig_id`, `contig_length` | Contig statistics (not used by the server at the moment). |

All columns are untyped `TEXT`/`INT` with no foreign keys. Add indexes on the join and filter columns (`*_matches.cluster_id`, `*_matches.genome_id`, `gene_matches.gene_id`, `gene_info.gene_id`, `gene_info.genome_id`). Without them, queries over about 100k clusters become slow. Run `sqlite3 data/db/gene_table.db .schema` to see the full schema.

The database and the BLAST databases are produced by a separate preparation pipeline (`ggprep`) that is not part of this repo.

## HTTP routes

All routes are registered in `NewRouter` in `main.go`. Most of them return HTML.

| Method & path | Returns |
|---------------|---------|
| `GET /` | Paged heatmap of all clusters (`page`, `order_by`, `order_dir`, `color_by`) |
| `GET /search` | Filtered heatmap. `search`, `search_by` = `function`\|`cog_id`\|`cluster_id`\|`gene_id`, `page`, `page_size`, `color_by` = `gene_copy_number`\|`max_gene_completeness`, plus `gm_<genome_id>=on` once per genome to keep |
| `GET /cluster/table/{cluster_id}` | Detail table for one cluster |
| `GET /cluster/heatmap/{genome_id}/{contig_id}/{gene_id}` | Heatmap row of the cluster that contains this gene |
| `POST /blast` | JSON `{"blast_type":"blastn"\|"blastp","sequence":"<fasta>"}`. With `Accept: application/json` it returns `202 {"job_id":...}`; otherwise it redirects to the status page |
| `GET /blast/{job_id}` | Job status / HTML report (the page refreshes itself until the job finishes) |
| `GET /sequence/by-gene` | FASTA. `genome_id`, `contig_id`, `gene_id`, `is_prot` |
| `GET /sequence/by-region` | FASTA. `genome_id`, `contig_id`, `start`, `end` |
| `GET /sequence/by-cluster` | Multi-FASTA of every gene in the cluster (plus matched regions if `is_prot=false`). `cluster_id`, `is_prot` |
| `GET /redirect/blastn/`, `/redirect/blastp/` | Redirects to NCBI web BLAST with the sequence filled in |
| `GET /api/v1/health` | Health check (used by the Docker `HEALTHCHECK`) |
| `GET /api/v1/cluster/{cluster_id}` | Currently the same HTML as `/cluster/table/...`. There is no JSON API yet. |

Example:

```bash
curl 'localhost:8080/sequence/by-gene?genome_id=P16PC&contig_id=contig000001&gene_id=P16PC_00001&is_prot=true'
```

## Code layout

```
main.go            config, DB wiring, router
pkg/handler/       HTTP handlers (parse query → call model → call render); AppContext holds shared deps
pkg/model/         SQL queries and domain types (Cluster → Genome → Gene/Region), BLAST execution + output rewriting
pkg/db/            GeneClusterDB (sql.DB + SequenceDB), blastdbcmd wrappers, in-memory BLAST job manager
pkg/render/        HTML templates as Go string constants, plus heatmap colour functions
pkg/middle/        logging / request-ID middleware (defined, not wired into the router yet)
logger/            global zap logger
static/            JS/CSS served at /static/
scripts/           build.sh (local image), publish-image.sh (build + push to GHCR)
```

How a request flows:

- Cluster search. `handler.ClusterSearchPage` builds a `model.ClusterSearchRequest`. `model.SearchGeneCluster` runs inside one read-only transaction:
  1. It fills a `unique_clusters` temp table with the current page of clusters.
  2. It "hydrates" each cluster with genes and regions using `json_group_array`.
  3. `render` turns the result into a genome × cluster heatmap.
- BLAST. `POST /blast` registers a job and runs `blastn`/`blastp -html` in a goroutine. Jobs are kept only in memory, and only the latest 3 are retained. Then `parseAndAddLink` rewrites the hit names to show full genome names and adds "View in gene table" links.

### Common extension points

- New search field: add a `ClusterField` constant and extend `ParseClusterField`, `whereFilterExpr`, and `orderByExpr` in `pkg/model/search.go`. Then add the option to the search form in `pkg/render/render_ggtable.go`.
- New heatmap colouring: write a `CellColorFunc` in `pkg/render/render_ggtable.go`, register it in `templateFuncMap`, and add it to the `color_by` canonicalisation in the handlers.
- New route: add the handler as a method on `*handler.AppContext` and register it in `NewRouter`.
- JSON API: none exists yet. The `api/v1` prefix is reserved for it, and `model.Cluster` already has JSON tags.

## Build, test, release

```bash
go build ./... && go vet ./...
go test ./...
scripts/build.sh                    # local image ggtable:sha-<sha10> + :latest
scripts/publish-image.sh                  # build + push ghcr.io/<gh-user>/ggtable:sha-<sha10> + :latest
scripts/publish-image.sh --version 3.0.5  # ...and also :3.0.5
scripts/publish-image.sh --no-push        # build only
```

To release, either pass `--version`, or tag the commit first (`git tag v3.0.5`) and `publish-image.sh` will pick the version up from the tag. The version becomes an extra image tag and is compiled into the binary (`-ldflags "-X main.version=..."` via the `VERSION` build arg), so it appears in the startup log. Builds without a version report `sha-<sha10>`, and plain `go build` reports `dev`.

`publish-image.sh` won't run if tracked files have uncommitted changes, so that the image tag always matches its commit. It needs a `gh` token with the `write:packages` scope. The GitHub Actions workflow (`.github/workflows/docker-build.yml`) builds and pushes the `sha-` and `latest` tags, but only when started manually.

## Known issues

These are worth knowing before you build on the code:

- Tests in `pkg/db` and `pkg/model` currently fail. They expect BLAST+ on `PATH` and fixtures under `test_data/` (gitignored), and the local fixtures still use the old `|` ID separator.
- The app has no authentication or rate limiting. Each BLAST request starts a BLAST+ process. Put it behind a reverse proxy or CDN before exposing it publicly.
- The cluster-table and BLAST pages use `text/template`, which does no HTML escaping. Only load data you trust.

## Contributing

Issues and PRs are welcome at https://github.com/PreechaPat/ggtable. Use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages (`feat:`, `fix:`, …) and run `go vet ./...` before opening a PR.

## License

MIT. See [LICENSE](LICENSE).

## Contact

Open an issue, or email the maintainer at preecha.pat@mahidol.ac.th.

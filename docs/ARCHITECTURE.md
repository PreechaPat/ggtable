# MVC and storage

The package names describe their roles: `model` is the model, `handler` is the
controller, and `render` is the view. `main.go` is the composition root: it opens
data, configures dependencies and routes, and starts the HTTP server.

Dependency direction:

```text
main -> handler -> model
                -> render -> model
main -> data -> sqldb   -> model
             -> blastdb -> model
```

`pkg/model` owns domain types, sequence IDs, validation, cluster statistics,
cluster FASTA assembly, and in-memory BLAST job state. `ClusterStore` and
`SequenceStore` describe the storage operations it consumes. Storage errors
are classified using `model.ErrNotFound` and `model.ErrInvalidSearch`.
It imports neither HTTP nor concrete database implementations.

`pkg/data` owns the fixed data directory layout, startup validation, and resource
lifecycle. `sqldb` owns SQL, hydration, filtering and transactions. `blastdb`
owns the flat-file sequence databases and external BLAST commands. Both return
model types. Keep file names and the `//` sequence ID separator compatible with
ggprep and ggorigin. Close the store after its consumers stop using it.

`pkg/handler` translates HTTP requests, selects domain operations, maps errors
to the existing response formats, and calls views. Its storage dependencies are
interfaces, allowing failure cases to be tested without installed BLAST tools.
Do not add SQL or command execution here.

`pkg/render` owns HTML templates, display labels and colors. Cluster summary
rules live in the model; views consume those results. Use `html/template` for
metadata interpolation. The legacy BLAST report renderer still consumes HTML
produced and annotated by the BLAST pipeline; migrating that HTML contract is a
separate remaining task.

Tests follow the seams: pure model tests for rules and assembly, SQLite fixture
tests for query behavior, BLAST fixture tests for executable integration,
controller tests for HTTP behavior, and render tests for escaping and empty
states. BLAST integration tests skip when the tools are absent.

Run `go build ./...`, `go vet ./...`, `go test ./...`, and
`go test -race ./...` from the repository root.

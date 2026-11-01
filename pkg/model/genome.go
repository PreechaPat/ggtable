package model

// GenomeCatalog is the list of genomes shown as heatmap columns, in display
// order, plus each genome's full name. It is loaded once at startup and then
// only read, so it is safe to share between requests.
type GenomeCatalog struct {
	ids   []string
	names map[string]string
}

// NewGenomeCatalog builds a catalog. ids sets the display order; names maps
// genome ID to full name.
func NewGenomeCatalog(ids []string, names map[string]string) GenomeCatalog {
	c := GenomeCatalog{ids: append([]string(nil), ids...), names: make(map[string]string, len(names))}
	for k, v := range names {
		c.names[k] = v
	}
	return c
}

// IDs returns the genome IDs in display order.
func (c GenomeCatalog) IDs() []string { return append([]string(nil), c.ids...) }

// Names returns genome ID -> full name. Callers must not modify it.
func (c GenomeCatalog) Names() map[string]string { return c.names }

// Name returns the full name of a genome.
func (c GenomeCatalog) Name(id string) (string, bool) {
	n, ok := c.names[id]
	return n, ok
}

// Reorder returns a catalog that shows only the given genomes, in the given
// order (GGSORTED). IDs not in the catalog are dropped and returned as unknown.
func (c GenomeCatalog) Reorder(ids []string) (GenomeCatalog, []string) {
	var kept, unknown []string
	seen := map[string]bool{}
	for _, id := range ids {
		switch {
		case seen[id]:
			continue
		case !c.has(id):
			unknown = append(unknown, id)
		default:
			kept = append(kept, id)
		}
		seen[id] = true
	}
	return GenomeCatalog{ids: kept, names: c.names}, unknown
}

func (c GenomeCatalog) has(id string) bool {
	_, ok := c.names[id]
	return ok
}

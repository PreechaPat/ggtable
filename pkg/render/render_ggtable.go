package render

import (
	"fmt"
	"html/template"
	"io"
	"math"

	"github.com/yumyai/ggtable/pkg/handler/request"
	"github.com/yumyai/ggtable/pkg/model"
)

// calculateColorByCompleteness maps a value from 70 to 100 to a color between #ff0000 (red) and #00ff00 (green).
func calculateColorByCompleteness(value float64) string {

	// For over 100% return green
	if value >= 100 {
		return fmt.Sprintf("#%02X%02X00", 0, 255)
	}

	// Return grey color if it is lower than 70%
	if value < 70 {
		return "#8B8989"
	}

	// Normalized value into 0-100
	normalized := (value - 70) / (100 - 70)

	var r, g int

	if normalized <= 50 {
		r = 255
		g = int(math.Round(normalized * 255))
	} else {
		r = int(math.Round((1 - normalized) * 255))
		g = 255
	}

	return fmt.Sprintf("#%02X%02X00", r, g)
}

// calculateByCopyNumber maps copy number to warm colors.
// 0 -> grey (region-only), 1..5 -> distinct YlOrRd-like buckets,
// >5 -> gradient from deep orange to dark red up to a cap.
func calculateByCopyNumber(val int) string {
	value := float64(val)
	// Region-only (no genes): keep grey to distinguish from warm palette
	if value <= 0 {
		return "#CCCCCC"
	}

	// Distinct buckets for 1..5 using YlOrRd scheme
	v := int(math.Floor(value + 1e-9))
	switch v {
	case 1:
		return "#FFFFB2" // light yellow
	case 2:
		return "#FECC5C" // yellow-orange
	case 3:
		return "#FD8D3C" // orange
	case 4:
		return "#F03B20" // red-orange
	case 5:
		return "#BD0026" // red
	}

	// Gradient for >5 copies
	const capVal = 30.0
	if value > capVal {
		value = capVal
	}
	// Interpolate from start (#BD0026) to end (#800000)
	sr, sg, sb := 189.0, 0.0, 38.0 // #BD0026
	er, eg, eb := 128.0, 0.0, 0.0  // #800000
	// Normalize t from 6..capVal
	t := (value - 5.0) / (capVal - 5.0)
	r := int(math.Round(lerp(sr, er, t)))
	g := int(math.Round(lerp(sg, eg, t)))
	b := int(math.Round(lerp(sb, eb, t)))
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

// CellColorFunc defines how a cell's color is computed from its contents.
type CellColorFunc func(genes []*model.Gene, regions []*model.Region) string

// Cell represents a single table cell's data for a genome column.
type Cell struct {
	Genes   []*model.Gene
	Regions []*model.Region
	Color   string
	Blank   bool
}

// arrangeGenomeWithColor arranges genomes according to genome IDs and uses
// colorFn to compute the color for each resulting cell.
func arrangeGenomeWithColor(genomes map[string]*model.Genome, genomeIDs []string, colorFn CellColorFunc) []Cell {
	out := make([]Cell, 0, len(genomeIDs))
	for _, genomeID := range genomeIDs {
		genome, exists := genomes[genomeID]
		if exists {
			var (
				allGenes   []*model.Gene
				allRegions []*model.Region
			)
			for _, gene := range genome.Genes {
				allGenes = append(allGenes, gene)
			}
			for _, region := range genome.Regions {
				allRegions = append(allRegions, region)
			}
			color := colorFn(allGenes, allRegions)
			out = append(out, Cell{Genes: allGenes, Regions: allRegions, Color: color, Blank: false})
		} else {
			out = append(out, Cell{Genes: []*model.Gene{}, Regions: []*model.Region{}, Color: "#000000", Blank: true})
		}
	}
	return out
}

// colorByMaxCompleteness colors a cell using the maximum gene completeness.
// Returns grey when there are no genes.
func colorByMaxCompleteness(genes []*model.Gene, _ []*model.Region) string {
	if len(genes) == 0 {
		return "#CCCCCC"
	}
	maxCompleteness := -1.0
	for _, g := range genes {
		if g.Completeness > maxCompleteness {
			maxCompleteness = g.Completeness
		}
	}
	return calculateColorByCompleteness(maxCompleteness)
}

// colorByCopyNumber colors by the number of gene members present in the cell.
func colorByCopyNumber(genes []*model.Gene, _ []*model.Region) string {
	if len(genes) == 0 {
		return "#CCCCCC"
	}
	return calculateByCopyNumber(len(genes))
}

// // arrangeGenome is the default variant used by templates; it uses colorByMaxCompleteness.
// func arrangeGenome(genomes map[string]*model.Genome, genomeIDs []string) []Cell {
// 	return arrangeGenomeWithColor(genomes, genomeIDs, colorByMaxCompleteness)
// }

// arrangeGenomeColorByCompleteness arranges genomes and colors by max completeness.
func arrangeGenomeColorByCompleteness(genomes map[string]*model.Genome, genomeIDs []string) []Cell {
	return arrangeGenomeWithColor(genomes, genomeIDs, colorByMaxCompleteness)
}

// arrangeGenomeColorByCopyNumber arranges genomes and colors by copy number.
func arrangeGenomeColorByCopyNumber(genomes map[string]*model.Genome, genomeIDs []string) []Cell {
	return arrangeGenomeWithColor(genomes, genomeIDs, colorByCopyNumber)
}

var searchPageTemplate *template.Template

// init initializes the templates used for rendering the HTML page.
func init() {
	mainTmpl := `
	<!DOCTYPE html>
	<html>
	<head>
	    <link href="/static/style.css" rel="stylesheet"></link>
	    <link href="/static/cform.css" rel="stylesheet"></link>
		<script src="/static/script.js" defer></script>
		<script src="/static/cform.js" defer></script>
		<title>Genome Cluster Analysis</title>
	</head>
	<body>
		<header class="app-header">
			<h1 class="app-name">Pins Gene Table v3.0</h1>
			<p class="app-description">
				the online informatics tool for analyzing and comparing gene content of P. insidiosum with related species.
			</p>
		</header>
		<div class="gtable-header">
			{{template "combinedForms" .}}
		</div>
		{{template "table" .}}
		{{template "pagination" .}}
	</body>
	</html>`

	// Combine multiple forms
	combinedForms := `
	{{define "combinedForms"}}
		<div class="combined-forms">
			<div class="form-column">
				<h3>Search Options</h3>
				{{template "searchForm1" .}}
			</div>
			<div class="form-column">
				<h3>BLAST search</h3>
				{{template "searchBLAST" .}}
			</div>
			<div class="form-column legend-column">
				<h3>Legend</h3>
				{{template "legend" .}}
			</div>
		</div>
	{{end}}
	`
	// <option value="gene_id"  {{if eq .SearchField "gene_id"}}selected{{end}}>Gene ID</option>

	searchForm1 := `
	{{ define "searchForm1"}}
  <form id="searchForm" action="/search" method="GET">
    <label for="search"></label>
    <div class="form-row">
      <label>Search by:<select name="search_by" id="search_by">
        <option value="function"   {{if eq .SearchField "function"}}selected{{end}}>Function</option>
        <option value="cog_id"     {{if eq .SearchField "cog_id"}}selected{{end}}>COG</option>
        <option value="cluster_id" {{if eq .SearchField "cluster_id"}}selected{{end}}>Cluster ID</option>
		<option value="gene_id"  {{if eq .SearchField "gene_id"}}selected{{end}}>Gene ID (exact match)</option>
	  </select></label>
	  <input type="text" name="search" placeholder="Search goes here"value="{{.SearchText}}"></input>
	    <input type="submit" value="Search"></input>
    </div>
	<div>
	<label>Page Size:
	<select name="page_size" id="page_size">
      <option value=50  {{if eq .PageSize 50}}selected{{end}}>50</option>
      <option value=100 {{if eq .PageSize 100}}selected{{end}}>100</option>
      <option value=250 {{if eq .PageSize 250}}selected{{end}}>250</option>
      <option value=500 {{if eq .PageSize 500}}selected{{end}}>500</option>
    </select>
	</label>
	</div>


	<div>
	<label>Color By:
	<select name="color_by" id="color_by" onchange="this.form.submit()" title="Choose how to color each cell">
	  <option value="gene_copy_number" {{if eq .ColorBy "gene_copy_number"}}selected{{end}}>Gene copy number</option>
	  <option value="max_gene_completeness" {{if eq .ColorBy "max_gene_completeness"}}selected{{end}}>Max gene completeness</option>
	</select>
	</label>
	</div>
    <!-- Remember page number, order by and order direction -->
    <input type="hidden" name="page" id="page" value="{{.CurrentPage}}"></input>
    <input type="hidden" name="order_by" id="order_by" value={{.OrderBy}}></input>
    <input type="hidden" name="order_dir" id="order_dir" value="{{.OrderDir}}"></input>

    {{template "filterByGenome" .}}
    {{template "filterByGene" .}}

  </form>
{{end}}
	`

	searchBLAST := `
	{{define "searchBLAST"}}
		<form id="searchBLAST">
			<div class="form-row">
				<label>BLAST Type:
					<select name="blast_type" id="blast_type">
						<option value=blastn>BLASTN</option>
						<option value=blastp>BLASTP</option>
					</select>
				</label>
			</div>
			<div class="form-row">
				<label>Sequence:</label>
				<textarea name="sequence" rows="4" cols="50" placeholder="Enter sequence here"></textarea>
			</div>
			<div class="form-row">
				<input type="submit" formaction="/blast" formmethod="POST" value="BLAST Search">
			</div>
		</form>
	{{end}}
`

	legendTmpl := `
	{{define "legend"}}
  {{if eq .ColorBy "gene_copy_number"}}
    <div class="legend">
      <div class="legend-row">
        <span class="legend-item"><span class="legend-swatch" style="background:#CCCCCC"></span><span>0 (region-only)</span></span>
        <span class="legend-item"><span class="legend-swatch" style="background:#FFFFB2"></span><span>1 copy</span></span>
        <span class="legend-item"><span class="legend-swatch" style="background:#FECC5C"></span><span>2 copies</span></span>
        <span class="legend-item"><span class="legend-swatch" style="background:#FD8D3C"></span><span>3 copies</span></span>
        <span class="legend-item"><span class="legend-swatch" style="background:#F03B20"></span><span>4 copies</span></span>
        <span class="legend-item"><span class="legend-swatch" style="background:#BD0026"></span><span>5 copies</span></span>
      </div>
      <div class="legend-row">
        <span class="legend-item"><span class="legend-swatch legend-swatch--wide" style="background: -webkit-linear-gradient(left,#BD0026,#800000); background: linear-gradient(90deg,#BD0026,#800000);"></span><span>6+ copies (darker = more)</span></span>
      </div>
    </div>
  {{else}}
    <div class="legend">
      <div class="legend-row">
        <span class="legend-item"><span class="legend-swatch" style="background:#8B8989"></span><span>&lt; 70%</span></span>
        <!-- Add WebKit-prefixed fallback for older Chrome -->
        <span class="legend-item"><span class="legend-swatch legend-swatch--wide" style="background: -webkit-linear-gradient(left,#FF0000,#FFFF00,#00FF00); background: linear-gradient(90deg,#FF0000,#FFFF00,#00FF00);"></span><span>70%–100%</span></span>
      </div>
    </div>
  {{end}}
	{{end}}`

	filterByGenome := `
	{{define "filterByGenome"}}
		<div class="collapsible">
			<div class="collapse-header">
				Genome(s) to display
			</div>
			<div class="collapse-content">
				<div>
					<button type="button" id="toggle-all-genomes" style="margin-bottom: 8px;">Select/Deselect All</button>
				</div>
				<div class="stacked-checkboxes">
					{{range .AllGenomeIDs}}
						{{ $key := . }} {{ $value := index $.GenomeNames $key }}
						<label style="display: block; margin-bottom: 4px; font-size 0.8rem">
							<input type="checkbox" 
							  class="genome-checkbox"
							  name="gm_{{$key}}"
							  value="y"
							  {{if hasKey $.SelectedGenome $key}}checked{{end}} />
							{{$value}}
						</label>
					{{end}}
				</div>
			</div>
		</div>
	{{end}}
	`
	filterByGene := `
	{{define "filterByGene"}}
	{{end}}
	`

	tableTmpl := `
    {{define "table"}}
        <table class="genetable" border="1">
            <tr>
            <th><a href="javascript:void(0)" onclick="updateForm({order_by: 'cluster_id'})">Cluster ID</a></th>
            <th><a href="javascript:void(0)" onclick="updateForm({order_by: 'cog_id'})">CogID</a></th>
            <th>Expected Length</th>
            <th class="col-func"><a href="javascript:void(0)" onclick="updateForm({order_by: 'function'})">Function Description</a></th>
                {{range .SelectedGenomeIDs}}<th class="rotate-text" title="{{index $.GenomeNames .}}"><span class="rotate-label">{{index $.GenomeNames .}}</span></th>{{end}}
            </tr>
            {{range .Rows}}
                <tr>
                    <td>
                        {{.ClusterProperty.ClusterID}}
                        <div class="menu">
                            <a href="#" class="close-menu">[close]</a>
                            [<a href="/cluster/table/{{.ClusterProperty.ClusterID}}" target="_blank">Overview</a>]
                            [<a href="/sequence/by-cluster?cluster_id={{ .ClusterProperty.ClusterID }}&is_prot=false" target="_blank">FNA</a>]
                            [<a href="/sequence/by-cluster?cluster_id={{ .ClusterProperty.ClusterID }}&is_prot=true" target="_blank">FAA</a>]
                        </div>
                    </td>
                    <td>{{.ClusterProperty.CogID}}</td>
                    <td>{{.ClusterProperty.ExpectedLength}}</td>
                    <td class="col-func">
                    <span class="truncate" title="{{.ClusterProperty.FunctionDescription}}">
                        {{.ClusterProperty.FunctionDescription}}
                    </span>
                    </td>
                    {{ range $index, $loc_map := (call $.ArrangeGenome .Genomes $.SelectedGenomeIDs) }}
                        {{template "cellContent" $loc_map}}
                    {{end}}
                </tr>
            {{end}}
        </table>

	{{end}}`

	cellTmpl := `
    {{define "cellContent"}}
        <td bgcolor={{.Color}}>
            <div class="menu">
                <a href="#" class="close-menu">[close]</a>
                {{if .Blank}}
                    <div>No information</div>
                {{else}}
                    {{range $index, $gene := .Genes}}
                        <div>{{$gene.GeneID}} - 
                            [<a href="/sequence/by-gene?genome_id={{$gene.Region.GenomeID}}&contig_id={{$gene.Region.ContigID}}&gene_id={{$gene.GeneID}}&is_prot=false" target="_blank">N</a>]
                            [<a href="/sequence/by-gene?genome_id={{$gene.Region.GenomeID}}&contig_id={{$gene.Region.ContigID}}&gene_id={{$gene.GeneID}}&is_prot=true" target="_blank">P</a>]
                            [<a href="/redirect/blastp?genome_id={{$gene.Region.GenomeID}}&contig_id={{$gene.Region.ContigID}}&gene_id={{$gene.GeneID}}" target="_blank">BLASTP</a>]
                        </div>
                    {{end}}
                    {{range $index, $region := .Regions}}
                        <div>
                            Region - {{ $region }}
                                [<a href="/sequence/by-region?genome_id={{$region.GenomeID}}&contig_id={{$region.ContigID}}&start={{$region.Start}}&end={{$region.End}}" target="_blank">N</a>]
                                [<a href="/redirect/blastn?genome_id={{ .GenomeID }}&contig_id={{ .ContigID }}&start={{ .Start }}&end={{ .End }}" target="_blank">BLASTN</a>]
                        </div>
                    {{end}}
                {{end}}
            </div>
        </td>
    {{end}}`

	paginationTmpl := `{{define "pagination"}}
	<div class="pagination">
		<div>Total page: {{.TotalPage}}</div>
		{{if gt .CurrentPage 1}}
			<a href="javascript:void(0);" onclick="updateForm({page: {{sub .CurrentPage 1}}})">&lt;&lt; prev</a>
		{{else}}
			<span>&lt;&lt; prev</span>
		{{end}}
		<span>{{.CurrentPage}} / {{.TotalPage}}</span>
		{{if lt .CurrentPage .TotalPage}}
			<a href="javascript:void(0);" onclick="updateForm({page: {{add .CurrentPage 1}}})">next &gt;&gt;</a>
		{{else}}
			<span>next &gt;&gt;</span>
		{{end}}
	</div>{{end}}`

	funcMap := template.FuncMap{
		// "arrangeGenome":                 arrangeGenome,
		// "arrangeGenomeColorByCopyNumber": arrangeGenomeColorByCopyNumber,
		// "arrangeGenomeColorByCompleteness": arrangeGenomeColorByCompleteness,
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"eqs": func(a, b string) bool { return a == b },
		"eqi": func(a, b int) bool { return a == b },
		"hasKey": func(m map[string]struct{}, k string) bool {
			_, ok := m[k]
			return ok
		},
	}

	searchPageTemplate = template.New("ggtable").Funcs(funcMap)
	searchPageTemplate = template.Must(searchPageTemplate.Parse(mainTmpl))
	searchPageTemplate = template.Must(searchPageTemplate.Parse(combinedForms))
	searchPageTemplate = template.Must(searchPageTemplate.Parse(searchForm1))
	searchPageTemplate = template.Must(searchPageTemplate.Parse(searchBLAST))
	searchPageTemplate = template.Must(searchPageTemplate.Parse(legendTmpl))
	searchPageTemplate = template.Must(searchPageTemplate.Parse(filterByGenome))
	searchPageTemplate = template.Must(searchPageTemplate.Parse(filterByGene))
	searchPageTemplate = template.Must(searchPageTemplate.Parse(tableTmpl))
	searchPageTemplate = template.Must(searchPageTemplate.Parse(cellTmpl))
	searchPageTemplate = template.Must(searchPageTemplate.Parse(paginationTmpl))
}

// RenderClusterHeatmapPage renders the heatmap table view for one or more clusters.
// Header for arrange the genome id
func RenderClusterHeatmapPage(w io.Writer, rows []*model.Cluster, search_request request.ClusterSearchRequest, totalPage int) error {

	genomeIDAll := model.ALL_GENOME_ID
	genomeMapAll := model.MAP_HEADER
	header := search_request.Genome_IDs
	currentPage := search_request.Page
	pageSize := search_request.Page_Size
	orderBy := search_request.Order_By.String()
	orderDir := search_request.Order_Dir
	if orderDir != "desc" {
		orderDir = "asc"
	}
	headerSet := make(map[string]struct{})
	for _, id := range header {
		headerSet[id] = struct{}{}
	}

	// Reorder `headerSet` reorder according to `genomeIDAll`
	reorderedGenomeIDs := []string{}
	for _, id := range genomeIDAll {
		if _, exists := headerSet[id]; exists {
			reorderedGenomeIDs = append(reorderedGenomeIDs, id)
		}
	}

	// For each Cluster row, build an array
	data := struct {
		Rows              []*model.Cluster
		SelectedGenomeIDs []string
		AllGenomeIDs      []string
		GenomeNames       map[string]string
		OrderBy           string
		OrderDir          string
		SelectedGenome    map[string]struct{}
		SearchText        string
		SearchField       string
		CurrentPage       int
		TotalPage         int
		PageSize          int
		ArrangeGenome     func(map[string]*model.Genome, []string) []Cell
		ColorBy           string
	}{
		Rows:              rows,
		SelectedGenomeIDs: reorderedGenomeIDs,
		AllGenomeIDs:      genomeIDAll,
		GenomeNames:       genomeMapAll,
		OrderBy:           orderBy,
		OrderDir:          orderDir,
		SelectedGenome:    headerSet,
		SearchText:        search_request.Search_For,
		SearchField:       search_request.Search_Field.String(),
		CurrentPage:       currentPage,
		TotalPage:         totalPage,
		PageSize:          pageSize,
		ArrangeGenome:     arrangeGenomeColorByCopyNumber,
		ColorBy:           search_request.Color_By,
	}

	// Pick arranger based on color selection
	switch search_request.Color_By {
	case "max_gene_completeness":
		data.ArrangeGenome = arrangeGenomeColorByCompleteness
	default:
		data.ColorBy = "gene_copy_number"
		data.ArrangeGenome = arrangeGenomeColorByCopyNumber
	}

	return searchPageTemplate.Execute(w, data)
}

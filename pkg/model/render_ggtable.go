package model

import (
	"fmt"
	"html/template"
	"io"
	"math"

	"github.com/yumyai/ggtable/pkg/handler/types"
)

// getColor maps a value from 70 to 100 to a color between #ff0000 (red) and #00ff00 (green).
func getColor(value float64) string {

	if value >= 100 {
		return fmt.Sprintf("#%02X%02X00", 0, 255)
	}

	// Return grey color if it is lower than 70%
	// The region will also return grey since it will always have -1.
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

// Arrange genomes in each row in order according to genome_names
// Also fill the blank if the genome does not exists.
func arrangeGenome(genomes map[string]*Genome, genome_ids []string) <-chan map[string]interface{} {

	outchan := make(chan map[string]interface{})

	go func() {
		defer close(outchan)
		// Sort output by predefined genome names, then return all under it.
		for _, genome_id := range genome_ids {
			genome, exists := genomes[genome_id]

			if exists { // Gene or region found in this genome

				// Find the best completeness and collect all gene_id
				var (
					maxCompletenessGene = -1.0
					allGenes            []*Gene
					allRegions          []*Region
				)

				for _, gene := range genome.Genes {
					allGenes = append(allGenes, gene)
					if gene.Completeness > maxCompletenessGene {
						maxCompletenessGene = gene.Completeness
					}
				}

				for _, region := range genome.Regions {
					allRegions = append(allRegions, region)
				}

				color := getColor(maxCompletenessGene)
				outchan <- map[string]interface{}{
					"genes":   allGenes,
					"regions": allRegions,
					"color":   color,
					"blank":   false,
				}
			} else {
				// No gene or region found for this genome
				outchan <- map[string]interface{}{
					"genes":   []*Gene{},
					"regions": []*Region{},
					"color":   "#000000",
					"blank":   true,
				}
			}
		}
	}()

	return outchan

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
		</div>
	{{end}}
	`

	searchForm1 := `
	{{ define "searchForm1"}}
		<form id="searchForm">
			<label for="search"></label>
			<div class="form-row">
				<label>Search by:<select name="search_by" id="search_by">
					<option value="function"   {{if eq .SearchField "function"}}selected{{end}}>Function</option>
					<option value="cog"        {{if eq .SearchField "cog"}}selected{{end}}>COG</option>
					<option value="cluster_id" {{if eq .SearchField "cluster_id"}}selected{{end}}>Cluster ID</option>
				</select></label>
				<input type="text" name="search" placeholder="In" value="{{.SearchText}}"></input>
			</div>
			<label>Page Size:<select name="page_size" id="page_size">
				<option value=25 {{if eq .PageSize 25}}selected{{end}}>25</option>
				<option value=50 {{if eq .PageSize 50}}selected{{end}}>50</option>
				<option value=100{{if eq .PageSize 100}}selected{{end}}>100</option>
			</select></label>
			<input type="hidden" name="page" value=1></input>
			{{template "filterByGenome" .}}
			{{template "filterByGene" .}}
			<input type="submit" formaction="/search" formmethod="GET" value="Search"></input>
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
				<th>Cluster ID</th>
				<th>CogID</th>
				<th>Expected Length</th>
				<th class="col-func">Function Description</th>
				{{range .SelectedGenomeIDs}}<th class="rotate-text">{{index $.GenomeNames .}}</th>{{end}}
			</tr>
			{{range .Rows}}
				<tr>
					<td>
						{{.ClusterProperty.ClusterID}}
						<div class="menu">
							<a href="#" class="close-menu">[close]</a>
							[<a href="/cluster/{{.ClusterProperty.ClusterID}}">Overview</a>]
							[<a href="/sequence/by-cluster?cluster_id={{ .ClusterProperty.ClusterID }}&is_prot=false">FNA</a>]
							[<a href="/sequence/by-cluster?cluster_id={{ .ClusterProperty.ClusterID }}&is_prot=true">FAA</a>]
						</div>
					</td>
					<td>{{.ClusterProperty.CogID}}</td>
					<td>{{.ClusterProperty.ExpectedLength}}</td>
<td class="col-func">
  <span class="truncate" title="{{.ClusterProperty.FunctionDescription}}">
    {{.ClusterProperty.FunctionDescription}}
  </span>
</td>					{{ range $index, $loc_map := arrangeGenome .Genomes $.SelectedGenomeIDs}}
						{{template "cellContent" $loc_map}}
					{{end}}
				</tr>
			{{end}}
		</table>

	{{end}}`

	cellTmpl := `
	{{define "cellContent"}}
		<td bgcolor={{.color}}>
			<div class="menu">
				<a href="#" class="close-menu">[close]</a>
				{{if .blank}}
					<div>No information</div>
				{{else}}
					{{range $index, $gene := .genes}}
						<div>{{$gene.GeneID}} - 
							[<a href="/sequence/by-gene?genome_id={{$gene.Region.GenomeID}}&contig_id={{$gene.Region.ContigID}}&gene_id={{$gene.GeneID}}&is_prot=false">N</a>]
							[<a href="/sequence/by-gene?genome_id={{$gene.Region.GenomeID}}&contig_id={{$gene.Region.ContigID}}&gene_id={{$gene.GeneID}}&is_prot=true">P</a>]
							[<a target="_blank" href="/redirect/blastp?genome_id={{$gene.Region.GenomeID}}&contig_id={{$gene.Region.ContigID}}&gene_id={{$gene.GeneID}}">BLASTP</a>]
						</div>
					{{end}}
					{{range $index, $region := .regions}}
						<div>
							Region - {{ $region }}
								[<a href="/sequence/by-region?genome_id={{$region.GenomeID}}&contig_id={{$region.ContigID}}&start={{$region.Start}}&end={{$region.End}}">N</a>]
								[<a target="_blank" href="/redirect/blastn?genome_id={{ .GenomeID }}&contig_id={{ .ContigID }}&start={{ .Start }}&end={{ .End }}">BLASTN</a>]
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
			<a href="javascript:void(0);" onclick="updatePage({{sub .CurrentPage 1}}, {{.PageSize}})">&lt;&lt; prev</a>
		{{else}}
			<span>&lt;&lt; prev</span>
		{{end}}
		<span>{{.CurrentPage}} / {{.TotalPage}}</span>
		{{if lt .CurrentPage .TotalPage}}
			<a href="javascript:void(0);" onclick="updatePage({{add .CurrentPage 1}}, {{.PageSize}})">next &gt;&gt;</a>
		{{else}}
			<span>next &gt;&gt;</span>
		{{end}}
	</div>{{end}}`

	funcMap := template.FuncMap{
		"arrangeGenome": arrangeGenome,
		"add":           func(a, b int) int { return a + b },
		"sub":           func(a, b int) int { return a - b },
		"eqs":           func(a, b string) bool { return a == b },
		"eqi":           func(a, b int) bool { return a == b },
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
	searchPageTemplate = template.Must(searchPageTemplate.Parse(filterByGenome))
	searchPageTemplate = template.Must(searchPageTemplate.Parse(filterByGene))
	searchPageTemplate = template.Must(searchPageTemplate.Parse(tableTmpl))
	searchPageTemplate = template.Must(searchPageTemplate.Parse(cellTmpl))
	searchPageTemplate = template.Must(searchPageTemplate.Parse(paginationTmpl))
}

// Function to render an HTML page with a table
// Header for arrange the genome id
func RenderClustersAsTable(w io.Writer, rows []*Cluster, search_request types.ClusterSearchRequest, totalPage int) error {

	genomeIDAll := ALL_GENOME_ID
	genomeMapAll := MAP_HEADER
	header := search_request.GenomeIDs
	currentPage := search_request.Page
	pageSize := search_request.Page_size

	// Create a set (map) for quick lookup of `header` (genomeIDs)
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
		Rows              []*Cluster
		SelectedGenomeIDs []string
		AllGenomeIDs      []string
		GenomeNames       map[string]string
		// For keep track when changing page
		SelectedGenome map[string]struct{}
		SearchText     string
		SearchField    string
		GenomeChecked  map[string]bool
		CurrentPage    int
		TotalPage      int
		PageSize       int
	}{
		Rows:              rows,
		SelectedGenomeIDs: reorderedGenomeIDs,
		AllGenomeIDs:      genomeIDAll,
		GenomeNames:       genomeMapAll,
		// For keep track when changing page
		SelectedGenome: headerSet,
		SearchText:     search_request.Search_for,
		SearchField:    search_request.Search_field,
		CurrentPage:    currentPage,
		TotalPage:      totalPage,
		PageSize:       pageSize,
	}

	return searchPageTemplate.Execute(w, data)
}

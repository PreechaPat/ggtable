package render

import (
	"html/template"
	"io"

	"github.com/yumyai/ggtable/pkg/handler/request"
	"github.com/yumyai/ggtable/pkg/model"
)

var clusterPageTemplate *template.Template

func init() {
	clusterMainTmpl := `
	<!DOCTYPE html>
	<html>
	<head>
	    <link href="/static/style.css" rel="stylesheet"></link>
	    <link href="/static/cform.css" rel="stylesheet"></link>
		<script src="/static/cluster_heatmap.js" defer></script>
		<script src="/static/cform.js" defer></script>
		<title>Genome Cluster Analysis</title>
	</head>
	<body>
		<header class="app-header">
			<h1 class="app-name">Pins Gene Table v3</h1>
			<p class="app-description">
				the online informatics tool for analyzing and comparing gene content of P. insidiosum with related species.
			</p>
		</header>
		<div class="gtable-header">
			<div class="combined-forms">
				<div class="form-column">
					<h3>Single Cluster Heatmap</h3>
					{{with index .Rows 0}}
						<p>Cluster ID: {{.ClusterProperty.ClusterID}}</p>
					{{end}}
					<h3>Heatmap Controls</h3>
					{{template "clusterControls" .}}
				</div>
				<div class="form-column legend-column">
					<h3>Legend</h3>
					{{template "legend" .}}
				</div>
			</div>
		</div>
		{{template "table" .}}
	</body>
	</html>`

	clusterControls := `
	{{define "clusterControls"}}
  <form id="clusterHeatmapForm" method="GET">
	<div>
	<label>Color By:
	<select name="color_by" id="color_by" onchange="this.form.submit()" title="Choose how to color each cell">
	  <option value="gene_copy_number" {{if eq .ColorBy "gene_copy_number"}}selected{{end}}>Gene copy number</option>
	  <option value="max_gene_completeness" {{if eq .ColorBy "max_gene_completeness"}}selected{{end}}>Max gene completeness</option>
	</select>
	</label>
	</div>

	<input type="hidden" name="page" id="page" value="{{.CurrentPage}}"></input>
	<input type="hidden" name="order_by" id="order_by" value={{.OrderBy}}></input>
	<input type="hidden" name="order_dir" id="order_dir" value="{{.OrderDir}}"></input>
	<input type="hidden" name="page_size" id="page_size" value="{{.PageSize}}"></input>

	{{template "filterByGenome" .}}
	<div style="margin-top: 8px;">
		<button type="submit">Apply filters</button>
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

	clusterPageTemplate = template.New("cluster-heatmap").Funcs(templateFuncMap)
	clusterPageTemplate = template.Must(clusterPageTemplate.Parse(clusterMainTmpl))
	clusterPageTemplate = template.Must(clusterPageTemplate.Parse(clusterControls))
	clusterPageTemplate = template.Must(clusterPageTemplate.Parse(legendTmpl))
	clusterPageTemplate = template.Must(clusterPageTemplate.Parse(filterByGenome))
	clusterPageTemplate = template.Must(clusterPageTemplate.Parse(tableTmpl))
	clusterPageTemplate = template.Must(clusterPageTemplate.Parse(cellTmpl))
}

// RenderClusterStandaloneHeatmapPage renders the cluster heatmap without search/BLAST controls.
func RenderClusterStandaloneHeatmapPage(w io.Writer, rows []*model.Cluster, searchRequest request.ClusterSearchRequest, totalPage int) error {
	data := buildClusterHeatmapPageData(rows, searchRequest, totalPage)
	return clusterPageTemplate.Execute(w, data)
}

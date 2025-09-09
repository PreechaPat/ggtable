// Render HTML for viewing a cluster

package render

import (
	"io"
	"math"
	"text/template"

	"github.com/yumyai/ggtable/logger"
	"github.com/yumyai/ggtable/pkg/model"
	"go.uber.org/zap"
)

var cluster_page_template *template.Template

// init initializes the templates used for rendering the cluster page.
func init() {
	mainTmpl := `
	<!DOCTYPE html>
	<html>
	<head>
	    <link href="static/style.css" rel="stylesheet"></link>
		<script src="static/script.js" defer></script>
		<title>Cluster Analysis: {{ .Cluster.ClusterProperty.ClusterID }}</title>
	</head>
	<body>
		<h1>Cluster Analysis: {{ .Cluster.ClusterProperty.ClusterID }} </h1>
		{{template "cluster_summary" . }}
		{{template "cluster_info" .Cluster}}
		<h2>Resources</h2>
		    <ul>
				<li>[<a href="/sequence/by-cluster?cluster_id={{ .Cluster.ClusterProperty.ClusterID }}&is_prot=false" target="_blank">FNA</a>]All nucleotide sequences in FASTA format</li>
				<li>[<a href="/sequence/by-cluster?cluster_id={{ .Cluster.ClusterProperty.ClusterID }}&is_prot=true" target-"_blank">FAA</a>]All protein sequences in FASTA format</li>
			</ul>
		<script>
		</script>
	</body>
	</html>`

	clusterSummaryTempl := `
	  {{define "cluster_summary"}}
		<div>
			Summary of {{ .Cluster.ClusterProperty.ClusterID }}
		</div>
		<div>
		    <p>This cluster consists of {{ len $.Cluster.Genomes }} genomes.</p>
			<p>Function: {{ $.Cluster.ClusterProperty.FunctionDescription }}</p>
			<p>Consists of {{ $.TotalGenes }} gene members and {{ $.TotalRegions }} homologous genomic regions from {{ len $.Cluster.Genomes }} / 101 genomes.</p>
			<p>Representative gene: {{ $.RepresentativeGene.GeneID }} ({{ calculateGeneLength $.RepresentativeGene.Region.End $.RepresentativeGene.Region.Start }} bp) [ {{$.RepresentativeGene.Region.GenomeID }}]</p>
		</div>
	  {{end}}
	`

	clusterInfoTmpl := `
	{{define "cluster_info"}}
		<table border="1">
		<tr>
			<th>Genome ID</th>
			<th>Gene ID</th>
			<th>Start</th>
			<th>Stop</th>
			<th>Length (bp)</th>
			<th>Contig ID</th>
			<th>Function Description</th>
			<th>Links</th>
		</tr>
		{{ range $genome_name, $genome := .Genomes }}
			{{ range $gene := $genome.Genes }}
				<tr style="background-color: #d9f2e6; color: #333333">
					<td>{{ $genome_name }}</td>
					<td>{{ $gene.GeneID }}</td>
					{{ with $gene.Region }}
						<td>{{ .Start }}</td>
						<td>{{ .End }}</td>
						<td>{{ calculateGeneLength .End .Start }}</td>
						<td>{{ .ContigID }}</td>
					{{ else }}
						<td>N/A</td>
						<td>N/A</td>
						<td>N/A</td>
						<td>N/A</td>
					{{ end }}
					<td> {{ .Description }} </td>
					<td>
						[<a href="/sequence/by-gene?genome_id={{$gene.Region.GenomeID}}&contig_id={{$gene.Region.ContigID}}&gene_id={{$gene.GeneID}}&is_prot=false" target="_blank">FNA</a>]
						[<a href="/sequence/by-gene?genome_id={{$gene.Region.GenomeID}}&contig_id={{$gene.Region.ContigID}}&gene_id={{$gene.GeneID}}&is_prot=true" target="_blank">FAA</a>]
						[<a href="/redirect/blastp?genome_id={{$gene.Region.GenomeID}}&contig_id={{$gene.Region.ContigID}}&gene_id={{$gene.GeneID}}" target="_blank">BLASTP</a>]
					</td>
				</tr>
			{{ end }}
			{{ range $region := $genome.Regions }}
				<tr style="background-color: #f2d9d9; color: #333333">
					<td>{{ $genome_name }}</td>
					<td> Region - {{ .GenomeID }}|{{ .ContigID }}:{{ .Start }}-{{ .End }} </td>
					<td>{{ .Start }}</td>
					<td>{{ .End }}</td>
					<td>{{ calculateGeneLength .End .Start }}</td>
					<td>{{ .ContigID }}</td>
					<td> N/A </td>
					<td>
						[<a href="/sequence/by-region?genome_id={{ .GenomeID }}&contig_id={{ .ContigID }}&start={{ .Start }}&end={{ .End }}" target="_blank">FNA</a>]
						[<a href="/redirect/blastn?genome_id={{ .GenomeID }}&contig_id={{ .ContigID }}&start={{ .Start }}&end={{ .End }}" target="_blank">BLASTN</a>]
					</td>
				</tr>
			{{ end }}
		{{ end }}
		</table>
	{{end}}`

	cluster_page_template = template.New("cluster_page")

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"eqs": func(a, b string) bool { return a == b },
		"eqi": func(a, b int) bool { return a == b },
		"hasKey": func(m map[string]struct{}, k string) bool {
			_, ok := m[k]
			return ok
		},
		"calculateGeneLength": func(a, b int) int { return int(math.Abs(float64(a - b + 1))) },
	}

	cluster_page_template = cluster_page_template.Funcs(funcMap)
	cluster_page_template = template.Must(cluster_page_template.Parse(mainTmpl))
	cluster_page_template = template.Must(cluster_page_template.Parse(clusterSummaryTempl))
	cluster_page_template = template.Must(cluster_page_template.Parse(clusterInfoTmpl))

}

// Longest gene would be used as representative gene
func getLongestGene(cluster *model.Cluster) *model.Gene {

	var longestGene *model.Gene
	maxLength := 0

	for _, genome := range cluster.Genomes {
		for _, gene := range genome.Genes {
			if gene.Region != nil {
				length := int(math.Abs(float64(gene.Region.End - gene.Region.Start)))
				if length > maxLength {
					// Found a longer gene, update the longestGene and maxLength
					longestGene = gene
					maxLength = length
				}
			}
		}
	}

	return longestGene
}

// Function to render an HTML page with a table
func RenderClusterPage(w io.Writer, cluster *model.Cluster) error {

	logger.Info("Rendering cluster page on", zap.String("cluster-id", cluster.ClusterProperty.ClusterID))

	// Count stats
	totalGenes := 0
	totalRegions := 0
	genomeCount := 0

	// Count genes and regions in each genome
	for _, genome := range cluster.Genomes {
		if len(genome.Genes) > 0 || len(genome.Regions) > 0 {
			genomeCount++ // Count genomes with genes or regions
		}
		totalGenes += len(genome.Genes)     // Count total number of gene members
		totalRegions += len(genome.Regions) // Count total number of regions
	}

	// Find representative gene
	rep_gene := getLongestGene(cluster)

	data := struct {
		Cluster            *model.Cluster
		RepresentativeGene *model.Gene
		GenomeNames        map[string]string
		TotalGenes         int
		TotalRegions       int
	}{
		Cluster:            cluster,
		RepresentativeGene: rep_gene,
		GenomeNames:        model.MAP_HEADER,
		TotalGenes:         totalGenes,
		TotalRegions:       totalRegions,
	}

	return cluster_page_template.Execute(w, data)
}

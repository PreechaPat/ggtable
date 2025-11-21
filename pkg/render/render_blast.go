package render

import (
	"io"
	"text/template"

	"github.com/yumyai/ggtable/logger"
	"go.uber.org/zap"
)

var blast_page_template *template.Template

// BlastPageData describes the state of a BLAST job for rendering.
type BlastPageData struct {
	JobID                  string
	BlastType              string
	BlastReport            string
	Status                 string
	ErrorMessage           string
	ShouldRefresh          bool
	RefreshIntervalSeconds int
}

// init initializes the templates used for rendering the HTML page.
func init() {
	mainTmpl := `
	<!DOCTYPE html>
	<html>
	<head>
	    <title>Gene Table BLAST</title>
	    <style>
        pre {
            white-space: pre-wrap;
            word-wrap: break-word;
        }
   		</style>
		{{ if .ShouldRefresh }}
        <script>
	        setTimeout(function () { window.location.reload(); }, {{ mul .RefreshIntervalSeconds 1000 }});
        </script>
		{{ end }}
	</head>
	<body>
		<h1>Gene Table V3</h1>
		<p><strong>Job ID:</strong> {{ .JobID }}</p>
		<p><strong>BLAST type:</strong> {{ .BlastType}}</p>
		<p><strong>Status:</strong> {{ .Status }}</p>
		{{ if .ErrorMessage }}
			<p style="color: red;">{{ .ErrorMessage }}</p>
		{{ else if .BlastReport }}
    		<pre>{{ .BlastReport }}</pre>
		{{ else }}
			<p>Your BLAST search is still running. This page refreshes every {{ .RefreshIntervalSeconds }} seconds.</p>
		{{ end }}
	</body>
	</html>`

	blast_page_template = template.New("blast_page").Funcs(template.FuncMap{
		"mul": func(a, b int) int { return a * b },
	})
	blast_page_template = template.Must(blast_page_template.Parse(mainTmpl))
}

// Function to render an HTML page with a table
func RenderBLASTPage(w io.Writer, data BlastPageData) error {
	logger.Info("Rendering blast page", zap.String("job_id", data.JobID), zap.String("status", data.Status))
	return blast_page_template.Execute(w, data)
}

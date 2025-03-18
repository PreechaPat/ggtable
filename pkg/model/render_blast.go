package model

import (
	"io"
	"text/template"

	"github.com/yumyai/ggtable/logger"
	"go.uber.org/zap"
)

var blast_page_template *template.Template

// init initializes the templates used for rendering the HTML page.
func init() {
	mainTmpl := `
	<!DOCTYPE html>
	<html>
	<head>
	    <style>
        pre {
            white-space: pre-wrap;
            word-wrap: break-word;
        }
   		</style>
	</head>
	<body>
		<h1>Gene Table V.3.0</h1>
		<p>BLAST type: {{ .Blast_type}} </p>
    	<pre>{{ .Blast_report }}</pre>
	</body>
	</html>`

	blast_page_template = template.New("blast_page")
	blast_page_template = template.Must(blast_page_template.Parse(mainTmpl))
}

// Function to render an HTML page with a table
func RenderBLASTPage(w io.Writer, raw_blast_result string, blast_type string) error {

	logger.Info("Rendering blast page on", zap.String("BLAST", blast_type))

	data := struct {
		Blast_type   string
		Blast_report string
	}{
		Blast_type:   blast_type,
		Blast_report: raw_blast_result,
	}

	return blast_page_template.Execute(w, data)
}

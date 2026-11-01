package middle

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAcceptsGzip(t *testing.T) {
	for header, want := range map[string]bool{
		"":                       false,
		"gzip":                   true,
		"deflate, gzip;q=1.0, *": true,
		"br, GZIP":               true,
		"gzip;q=0":               false,
		"gzip; q=0.000":          false,
		"identity":               false,
		"x-gzip":                 false,
	} {
		if got := acceptsGzip(header); got != want {
			t.Errorf("acceptsGzip(%q) = %v, want %v", header, got, want)
		}
	}
}

func serve(h http.Handler, acceptEncoding string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/", nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestGzip(t *testing.T) {
	page := strings.Repeat("<td>cell</td>", 1000)
	html := Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "13000") // must be dropped when compressing
		io.WriteString(w, page[:5000])            // several writes, content type sniffed
		io.WriteString(w, page[5000:])
	}))

	rr := serve(html, "gzip")
	if rr.Header().Get("Content-Encoding") != "gzip" || rr.Header().Get("Content-Length") != "" {
		t.Fatalf("headers = %v, want gzip and no Content-Length", rr.Header())
	}
	if !strings.Contains(rr.Header().Get("Vary"), "Accept-Encoding") {
		t.Error("missing Vary: Accept-Encoding")
	}
	zr, err := gzip.NewReader(rr.Body)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(zr)
	if string(got) != page {
		t.Errorf("decompressed body differs (%d bytes, want %d)", len(got), len(page))
	}
	if rr.Body.Len() > len(page)/10 {
		t.Errorf("compressed size %d is not much smaller than %d", rr.Body.Len(), len(page))
	}

	if rr := serve(html, ""); rr.Header().Get("Content-Encoding") != "" || rr.Body.String() != page {
		t.Error("client without gzip got a compressed or altered body")
	}
}

func TestGzip_PassThrough(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"binary": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("png"))
		},
		"not modified": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/css")
			w.WriteHeader(http.StatusNotModified)
		},
		"already encoded": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Content-Encoding", "br")
			w.Write([]byte("x"))
		},
	}
	for name, h := range cases {
		rr := serve(Gzip(h), "gzip")
		if rr.Header().Get("Content-Encoding") == "gzip" {
			t.Errorf("%s: compressed, want pass-through", name)
		}
	}
}

package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"sync"
)

// staticHandler serves files with an ETag made from their content, so
// browsers revalidate with If-None-Match and get a 304 instead of the file.
// Embedded files have no modification time, so without it they would be
// sent again on every page. Hashes are computed once per file.
type staticHandler struct {
	files  fs.FS
	server http.Handler
	mu     sync.Mutex
	etags  map[string]string
}

func newStaticHandler(files fs.FS) *staticHandler {
	return &staticHandler{files: files, server: http.FileServerFS(files), etags: map[string]string{}}
}

func (s *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if etag := s.etag(r.URL.Path); etag != "" {
		w.Header().Set("ETag", etag)
		// Revalidate each time, so edited files (with -static) show up at once.
		w.Header().Set("Cache-Control", "no-cache")
	}
	s.server.ServeHTTP(w, r)
}

// etag returns the quoted content hash of the file at urlPath, or "" if it
// is not a readable file (the file server then answers 404 itself).
func (s *staticHandler) etag(urlPath string) string {
	name := path.Clean("/" + urlPath)[1:]
	if name == "" {
		return ""
	}
	info, err := fs.Stat(s.files, name)
	if err != nil || info.IsDir() {
		return ""
	}
	// Key on size and mtime too, so a file edited on disk gets a new hash.
	key := name + "|" + info.ModTime().String() + "|" + strconv.FormatInt(info.Size(), 10)

	s.mu.Lock()
	defer s.mu.Unlock()
	if etag, ok := s.etags[key]; ok {
		return etag
	}
	data, err := fs.ReadFile(s.files, name)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	s.etags[key] = etag
	return etag
}

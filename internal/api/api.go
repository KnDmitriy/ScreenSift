package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"screensift/internal/indexer"
	"screensift/internal/store"
)

type Server struct {
	http    *http.Server
	store   *store.Store
	indexer *indexer.Indexer
}

func NewServer(addr string, st *store.Store, ix *indexer.Indexer) *Server {
	s := &Server{store: st, indexer: ix}
	mux := http.NewServeMux()
	mux.HandleFunc("/search", s.search)
	mux.HandleFunc("/documents", s.documents)
	mux.HandleFunc("/documents/", s.document)
	mux.HandleFunc("/stats", s.stats)
	mux.HandleFunc("/index/rebuild", s.rebuild)
	mux.HandleFunc("/openapi.yaml", openAPI)
	mux.HandleFunc("/swagger", swagger)
	s.http = &http.Server{Addr: addr, Handler: logging(mux), ReadHeaderTimeout: 5 * time.Second}
	return s
}
func (s *Server) Start() error                       { return s.http.ListenAndServe() }
func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, e := strconv.Atoi(v); e == nil {
			limit = n
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	res, err := s.store.Search(q, limit)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
func (s *Server) documents(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeErr(w, 405, errors.New("method not allowed"))
		return
	}
	limit, offset := 100, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, e := strconv.Atoi(v); e == nil {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, e := strconv.Atoi(v); e == nil {
			offset = n
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	d, err := s.store.List(limit, offset)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) document(w http.ResponseWriter, r *http.Request) {
	idstr := strings.TrimPrefix(r.URL.Path, "/documents/")
	if strings.HasSuffix(idstr, "/reindex") {
		idstr = strings.TrimSuffix(idstr, "/reindex")
		id, err := strconv.ParseInt(idstr, 10, 64)
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		if err = s.indexer.Reindex(id); err != nil {
			writeErr(w, 404, err)
			return
		}
		writeJSON(w, 202, map[string]any{"status": "queued", "id": id})
		return
	}
	id, err := strconv.ParseInt(idstr, 10, 64)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	d, err := s.store.GetByID(id)
	if err != nil {
		writeErr(w, 404, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.Stats()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	m := map[string]any{"images": st.Images, "indexed": st.Indexed, "failed": st.Failed, "processing": st.Processing, "pending": st.Pending, "queueSize": s.indexer.QueueSize(), "workers": s.indexer.Workers(), "lastIndexedAt": st.LastIndexedAt}
	writeJSON(w, 200, m)
}
func (s *Server) rebuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeErr(w, 405, errors.New("method not allowed"))
		return
	}
	if err := s.indexer.Rebuild(); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 202, map[string]string{"status": "rebuild queued"})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { next.ServeHTTP(w, r) })
}

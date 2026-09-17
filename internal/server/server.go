package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Rayfts/WhyThis/internal/harness"
	"github.com/Rayfts/WhyThis/internal/risk"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

type Service interface {
	Line(context.Context, string) (evidence.Report, error)
	FileHistory(context.Context, string) (evidence.Report, error)
	Symbol(context.Context, string) (evidence.Report, error)
	Commit(context.Context, string) (evidence.Report, error)
	Similar(context.Context, string, int) (evidence.Report, error)
	Ask(context.Context, string) (evidence.Report, error)
	PR(context.Context, int) (evidence.Report, error)
	Risk(context.Context, string) (risk.Report, error)
	Harnesses(context.Context) []harness.Capabilities
	Capability(context.Context, string) (harness.Capabilities, error)
}

type Server struct {
	Addr    string
	Service Service
	Logger  *slog.Logger
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, map[string]any{"ok": true}) })
	mux.HandleFunc("GET /v1/archaeology", s.archaeology)
	mux.HandleFunc("GET /v1/history", s.history)
	mux.HandleFunc("GET /v1/symbol", s.symbol)
	mux.HandleFunc("GET /v1/commit", s.commit)
	mux.HandleFunc("GET /v1/similar", s.similar)
	mux.HandleFunc("GET /v1/ask", s.ask)
	mux.HandleFunc("GET /v1/pr", s.pr)
	mux.HandleFunc("GET /v1/risk", s.risk)
	mux.HandleFunc("GET /v1/harnesses", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.Service.Harnesses(r.Context()))
	})
	mux.HandleFunc("GET /v1/capabilities", s.capabilities)
	return limitBody(mux)
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	if s.Addr == "" {
		s.Addr = "127.0.0.1:7788"
	}
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
	srv := &http.Server{Addr: s.Addr, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	s.Logger.Info("whythis server listening", "addr", s.Addr)
	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) archaeology(w http.ResponseWriter, r *http.Request) {
	target, ok := requiredQuery(w, r, "target")
	if !ok {
		return
	}
	rep, err := s.Service.Line(r.Context(), target)
	writeResult(w, rep, err)
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	path, ok := requiredQuery(w, r, "path")
	if !ok {
		return
	}
	rep, err := s.Service.FileHistory(r.Context(), path)
	writeResult(w, rep, err)
}

func (s *Server) symbol(w http.ResponseWriter, r *http.Request) {
	name, ok := requiredQuery(w, r, "name")
	if !ok {
		return
	}
	rep, err := s.Service.Symbol(r.Context(), name)
	writeResult(w, rep, err)
}

func (s *Server) commit(w http.ResponseWriter, r *http.Request) {
	sha, ok := requiredQuery(w, r, "sha")
	if !ok {
		return
	}
	rep, err := s.Service.Commit(r.Context(), sha)
	writeResult(w, rep, err)
}

func (s *Server) similar(w http.ResponseWriter, r *http.Request) {
	sha, ok := requiredQuery(w, r, "sha")
	if !ok {
		return
	}
	limit := 10
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			http.Error(w, "limit must be a positive integer", http.StatusBadRequest)
			return
		}
		limit = v
	}
	rep, err := s.Service.Similar(r.Context(), sha, limit)
	writeResult(w, rep, err)
}

func (s *Server) ask(w http.ResponseWriter, r *http.Request) {
	q, ok := requiredQuery(w, r, "q")
	if !ok {
		return
	}
	rep, err := s.Service.Ask(r.Context(), q)
	writeResult(w, rep, err)
}

func (s *Server) pr(w http.ResponseWriter, r *http.Request) {
	raw, ok := requiredQuery(w, r, "number")
	if !ok {
		return
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		http.Error(w, "number must be a positive integer", http.StatusBadRequest)
		return
	}
	rep, err := s.Service.PR(r.Context(), n)
	writeResult(w, rep, err)
}

func (s *Server) capabilities(w http.ResponseWriter, r *http.Request) {
	id, ok := requiredQuery(w, r, "id")
	if !ok {
		return
	}
	cap, err := s.Service.Capability(r.Context(), id)
	writeResult(w, cap, err)
}

func (s *Server) risk(w http.ResponseWriter, r *http.Request) {
	path, ok := requiredQuery(w, r, "path")
	if !ok {
		return
	}
	rep, err := s.Service.Risk(r.Context(), path)
	writeResult(w, rep, err)
}

func requiredQuery(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		http.Error(w, key+" is required", http.StatusBadRequest)
		return "", false
	}
	return v, true
}

func writeResult(w http.ResponseWriter, v any, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next.ServeHTTP(w, r)
	})
}

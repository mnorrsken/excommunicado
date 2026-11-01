// Package server serves the Outlook add-in and the scoring API on localhost.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"strings"

	"github.com/mnorrsken/excommunicado/internal/scorer"
	"github.com/mnorrsken/excommunicado/internal/version"
)

//go:embed web
var webFS embed.FS

const maxBody = 1 << 20

// Scorer rates mails. *scorer.Scorer implements it.
type Scorer interface {
	Score(ctx context.Context, subject, body string) (scorer.Result, error)
	Ready(ctx context.Context) bool
}

// Handler returns the whole HTTP API. certPEM is served so the setup script can
// install trust in the certificate.
func Handler(s Scorer, certPEM []byte) http.Handler {
	web, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err) // the embedded directory always exists
	}
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(web))
	mux.HandleFunc("GET /cert.pem", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Write(certPEM)
	})
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"llm_ready": s.Ready(r.Context()), "version": version.Version})
	})
	mux.HandleFunc("POST /api/score", scoreHandler(s))
	return guard(mux)
}

func scoreHandler(s Scorer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// JSON only: browsers then refuse to let other web sites post mail text here.
		if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, "content type must be application/json")
			return
		}
		var in struct {
			Subject string `json:"subject"`
			Body    string `json:"body"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		res, err := s.Score(r.Context(), in.Subject, in.Body)
		switch {
		case errors.Is(err, scorer.ErrEmpty):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, context.Canceled):
			// The pane moved on to another mail. Nobody is listening.
		case err != nil:
			// Only the error is logged, never the mail.
			log.Printf("scoring: %v", err)
			writeError(w, http.StatusServiceUnavailable, "the language model is not available yet")
		default:
			writeJSON(w, http.StatusOK, res)
		}
	}
}

// guard rejects requests that name another host, which stops web sites from reaching
// this server through DNS rebinding, and sets headers for all responses.
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = strings.Trim(r.Host, "[]")
		}
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		// Office web views keep old add-in files for a long time otherwise.
		h.Set("Cache-Control", "no-cache")
		// office.js loads more scripts and frames from Microsoft that can't be listed
		// here without testing every Outlook version, so this stays narrow.
		h.Set("Content-Security-Policy", "object-src 'none'; base-uri 'none'")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writing response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

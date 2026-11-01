// Command excommunicado serves the Outlook add-in and scores mails with a local LLM.
package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/mnorrsken/excommunicado/internal/scorer"
	"github.com/mnorrsken/excommunicado/internal/server"
	"github.com/mnorrsken/excommunicado/internal/version"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:47823", "address to listen on")
	llmURL := flag.String("llm", "http://127.0.0.1:8080", "URL of the llama.cpp server")
	dataDir := flag.String("data", "data", "directory for the HTTPS certificate")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0o700); err != nil {
		log.Fatalf("creating data directory: %v", err)
	}
	cert, certPEM, err := server.LoadOrCreateCert(*dataDir)
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{
		Addr:              *addr,
		Handler:           server.Handler(scorer.New(*llmURL), certPEM),
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("excommunicado %s (%s) on https://%s", version.Version, version.Commit, *addr)
	log.Fatal(srv.ListenAndServeTLS("", ""))
}

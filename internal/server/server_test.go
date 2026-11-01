package server

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mnorrsken/excommunicado/internal/scorer"
)

type stubScorer struct {
	res scorer.Result
	err error
}

func (s stubScorer) Score(context.Context, string, string) (scorer.Result, error) {
	return s.res, s.err
}
func (s stubScorer) Ready(context.Context) bool { return true }

func TestScoreAPI(t *testing.T) {
	tests := []struct {
		name        string
		scorer      stubScorer
		contentType string
		body        string
		want        int
	}{
		{"ok", stubScorer{res: scorer.Result{Score: 7}}, "application/json", `{"body":"x"}`, 200},
		{"charset is fine", stubScorer{}, "application/json; charset=utf-8", `{"body":"x"}`, 200},
		{"form post refused", stubScorer{}, "text/plain", `{"body":"x"}`, 415},
		{"bad json", stubScorer{}, "application/json", `{`, 400},
		{"too big", stubScorer{}, "application/json", `{"body":"` + strings.Repeat("x", maxBody) + `"}`, 400},
		{"empty mail", stubScorer{err: scorer.ErrEmpty}, "application/json", `{}`, 422},
		{"llm down", stubScorer{err: errors.New("down")}, "application/json", `{"body":"x"}`, 503},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "https://localhost:47823/api/score", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			rec := httptest.NewRecorder()
			Handler(tt.scorer, nil).ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("status %d, want %d: %s", rec.Code, tt.want, rec.Body)
			}
		})
	}
}

func TestHostGuard(t *testing.T) {
	tests := []struct {
		host string
		want int
	}{
		{"localhost:47823", 200},
		{"127.0.0.1:47823", 200},
		{"[::1]:47823", 200},
		{"localhost", 200},
		{"evil.example:47823", 403},
		{"evil.example", 403},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/health", nil)
			req.Host = tt.host
			rec := httptest.NewRecorder()
			Handler(stubScorer{}, nil).ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestStaticFiles(t *testing.T) {
	for _, path := range []string{"/taskpane.html", "/taskpane.js", "/taskpane.css", "/manifest.xml", "/icon-16.png", "/icon-32.png", "/icon-64.png", "/icon-80.png", "/icon-128.png", "/cert.pem"} {
		req := httptest.NewRequest("GET", "https://localhost"+path, nil)
		rec := httptest.NewRecorder()
		Handler(stubScorer{}, []byte("cert")).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", path, rec.Code)
		}
	}
}

func TestManifestIsWellFormed(t *testing.T) {
	data, err := webFS.ReadFile("web/manifest.xml")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	if v.XMLName.Local != "OfficeApp" {
		t.Errorf("root element %q", v.XMLName.Local)
	}
}

func TestCertCoversLocalhostOnly(t *testing.T) {
	dir := t.TempDir()
	_, certPEM, err := LoadOrCreateCert(dir)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if cert.IsCA {
		t.Error("certificate must not be a CA")
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	for _, host := range []string{"localhost", "127.0.0.1"} {
		if _, err := cert.Verify(x509.VerifyOptions{DNSName: host, Roots: roots, CurrentTime: time.Now()}); err != nil {
			t.Errorf("%s: %v", host, err)
		}
	}
	if _, err := cert.Verify(x509.VerifyOptions{DNSName: "evil.example", Roots: roots}); err == nil {
		t.Error("valid for another host")
	}

	_, again, err := LoadOrCreateCert(dir)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(certPEM) {
		t.Error("certificate changed on restart, the user would have to trust it again")
	}
}

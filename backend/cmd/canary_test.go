package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WisewareOrg/WiseKiosk/backend/internal/canarytest"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/router"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/staticserve"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/upstream"
)

const (
	// canary is the value planted through the delivery path and swept for.
	canary = canarytest.Value
	// canarySecret is the logical name the keyed entry declares.
	canarySecret = "WISEKIOSK_TEST_SERVER_KEY"
	// canaryHeader is where the keyed entry places the value.
	canaryHeader = "X-Test-Source-Key"
	// canaryParam is the second placement the keyed entry uses.
	canaryParam = "apikey"
	// configBody is the file served beside the index, standing for whatever an
	// operator has mounted into the tree.
	configBody = `{"fixture":"config"}`
)

// canaryLog collects the standard logger's output behind a lock, so a reader of
// the collected text takes the same one every Write takes.
type canaryLog struct {
	mu        sync.Mutex
	collected strings.Builder
}

func (l *canaryLog) Write(line []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.collected.Write(line)
}

func (l *canaryLog) text() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.collected.String()
}

// captureLog points the standard logger at a sink for the test's duration and
// restores the writer it replaced, which is not necessarily os.Stderr: an outer
// harness redirecting the logger keeps its redirect.
func captureLog(t *testing.T) *canaryLog {
	t.Helper()

	sink := &canaryLog{}
	previous := log.Writer()
	log.SetOutput(sink)
	t.Cleanup(func() { log.SetOutput(previous) })
	return sink
}

// sweep fails where value appears in a response's body or in any of its header
// values, naming the surface that carried it.
func sweep(t *testing.T, surface string, recorder *httptest.ResponseRecorder, value string) {
	t.Helper()

	if strings.Contains(recorder.Body.String(), value) {
		t.Errorf("%s: the response body carries the secret value: %q", surface, recorder.Body)
	}
	for name, values := range recorder.Header() {
		for _, carried := range values {
			if strings.Contains(carried, value) {
				t.Errorf("%s: response header %s carries the secret value: %q", surface, name, carried)
			}
		}
	}
}

// keyedSource is the upstream the keyed entry calls. It keeps the two
// placements the last call carried and answers what a test sets.
type keyedSource struct {
	server *httptest.Server

	mu     sync.Mutex
	header string
	query  string
	status int
	body   string
}

func newKeyedSource(t *testing.T) *keyedSource {
	t.Helper()

	source := &keyedSource{status: http.StatusOK, body: `{"reading":42}`}
	source.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		source.mu.Lock()
		source.header = r.Header.Get(canaryHeader)
		source.query = r.URL.Query().Get(canaryParam)
		status, answer := source.status, source.body
		source.mu.Unlock()

		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, answer)
	}))
	t.Cleanup(source.server.Close)
	return source
}

func (s *keyedSource) placements() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.header, s.query
}

func (s *keyedSource) answers(status int, answer string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status, s.body = status, answer
}

// keyedEntry registers a source needing canarySecret and placing it in both an
// outbound header and an outbound query parameter.
func keyedEntry(source *keyedSource) router.Entry {
	return router.Entry{
		Config: upstream.Config{
			SuccessTTL:        10 * time.Minute,
			NegativeTTL:       time.Minute,
			RequestsPerMinute: 60,
			Burst:             10,
			Timeout:           5 * time.Second,
			MaxBytes:          1 << 16,
		},
		Source: "keyed",
		Secret: canarySecret,
		InjectSecret: func(request *http.Request, secretValue string) {
			request.Header.Set(canaryHeader, secretValue)
			query := request.URL.Query()
			query.Set(canaryParam, secretValue)
			request.URL.RawQuery = query.Encode()
		},
		Shape: func(payload []byte) (any, error) {
			var decoded map[string]any
			if err := json.Unmarshal(payload, &decoded); err != nil {
				return nil, err
			}
			return decoded, nil
		},
	}
}

// keyedSeam is the /api/ seam these cases assemble the server over: one source
// at a path of its own, judged and addressed the way a module judges and
// addresses its own, and the framework's own answer for every other API path.
//
// It stands in for a module rather than using one: what these cases measure is
// the assembled process, and a real module would tie them to that module's
// source and its upstream.
func keyedSeam(source *keyedSource) http.Handler {
	route := router.NewRoute(keyedEntry(source))

	seam := http.NewServeMux()
	seam.HandleFunc("POST /api/keyed", func(w http.ResponseWriter, r *http.Request) {
		station := r.URL.Query().Get("station")
		if station == "" {
			router.Reject(w, router.InvalidParameters, "a station is required")
			return
		}
		route.Serve(w, r, station, source.server.URL+"/?station="+url.QueryEscape(station))
	})
	seam.Handle("/api/", router.NewFallback(seam))
	return seam
}

// servedTree writes an index and a configuration file into a temporary
// directory and returns it.
func servedTree(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	files := map[string]string{"index.html": indexBody, "config.json": configBody}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	return dir
}

// plant writes the canary value to a file and points the delivery variable at
// that file.
func plant(t *testing.T) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "server-key")
	if err := os.WriteFile(path, []byte(canary+"\n"), 0o600); err != nil {
		t.Fatalf("planting the secret: %v", err)
	}
	t.Setenv(canarySecret+"_FILE", path)
}

// TestNoSecretValueReachesAnyServedSurface plants a known value through the
// <NAME>_FILE delivery path and sweeps every surface the assembled server
// answers on — liveness, the served tree, the API route and its upstream
// failure — for it, in every response body, every response header value and
// the captured log output (ADR 0023 rev 2).
// TST022
func TestNoSecretValueReachesAnyServedSurface(t *testing.T) {
	plant(t)
	logged := captureLog(t)

	source := newKeyedSource(t)
	server := newServer(staticserve.New(http.Dir(servedTree(t))), keyedSeam(source))

	served := send(server, http.MethodPost, "/api/keyed?station=one")
	if served.Code != http.StatusOK {
		t.Fatalf("POST /api/keyed: status = %d, want %d (%s)", served.Code, http.StatusOK, served.Body)
	}

	// Without this the sweep below passes on a server that placed no secret at
	// all.
	header, query := source.placements()
	if header != canary {
		t.Errorf("upstream saw header %s = %q, want the planted value", canaryHeader, header)
	}
	if query != canary {
		t.Errorf("upstream saw parameter %s = %q, want the planted value", canaryParam, query)
	}

	// A second station, so the failure is fetched rather than served from the
	// first answer's cache entry.
	source.answers(http.StatusUnauthorized, `{"error":"key `+canary+` is not valid"}`)
	failed := send(server, http.MethodPost, "/api/keyed?station=two")

	// The status each surface is swept under, so a surface answering something
	// other than what it serves cannot pass the sweep on an empty body.
	surfaces := []struct {
		surface  string
		recorder *httptest.ResponseRecorder
		status   int
	}{
		{"GET " + healthPath, get(server, healthPath), http.StatusOK},
		{"GET /", get(server, "/"), http.StatusOK},
		{"GET /config.json", get(server, "/config.json"), http.StatusOK},
		{"GET /no-such-asset.js", get(server, "/no-such-asset.js"), http.StatusNotFound},
		{"POST /api/keyed", served, http.StatusOK},
		{"POST /api/keyed, upstream failed", failed, http.StatusBadGateway},
	}
	for _, swept := range surfaces {
		if swept.recorder.Code != swept.status {
			t.Errorf("%s: status = %d, want %d", swept.surface, swept.recorder.Code, swept.status)
		}
		sweep(t, swept.surface, swept.recorder, canary)
	}

	if collected := logged.text(); strings.Contains(collected, canary) {
		t.Errorf("the captured log output carries the secret value: %q", collected)
	}
}

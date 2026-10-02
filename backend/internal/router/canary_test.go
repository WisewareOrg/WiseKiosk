package router

import (
	"context"
	"encoding/json"
	"errors"
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

	"github.com/WisewareOrg/WiseKiosk/backend/internal/upstream"
)

// canaryQuery is the second outbound placement the canary entry puts the
// secret in, beside secretHeader.
const canaryQuery = "apikey"

// logSink collects the standard logger's output behind a lock, so a reader of
// the collected text takes the same one every Write takes.
type logSink struct {
	mu        sync.Mutex
	collected strings.Builder
}

func (s *logSink) Write(line []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.collected.Write(line)
}

func (s *logSink) text() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.collected.String()
}

// captureLog points the standard logger at a sink for the test's duration and
// restores the writer it replaced, which is not necessarily os.Stderr: an outer
// harness redirecting the logger keeps its redirect.
func captureLog(t *testing.T) *logSink {
	t.Helper()

	sink := &logSink{}
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

// canarySource is the upstream the canary entry calls. It keeps the two
// placements the last call carried, which is what tells an injected secret
// from one that was never placed, and answers what a test sets.
type canarySource struct {
	server *httptest.Server

	mu     sync.Mutex
	header string
	query  string
	status int
	body   string
}

func newCanarySource(t *testing.T) *canarySource {
	t.Helper()

	source := &canarySource{status: http.StatusOK, body: `{"reading":42}`}
	source.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		source.mu.Lock()
		source.header = r.Header.Get(secretHeader)
		source.query = r.URL.Query().Get(canaryQuery)
		status, answer := source.status, source.body
		source.mu.Unlock()

		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, answer)
	}))
	t.Cleanup(source.server.Close)
	return source
}

// placements returns the header and query values the last call carried.
func (s *canarySource) placements() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.header, s.query
}

func (s *canarySource) answers(status int, answer string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status, s.body = status, answer
}

// canaryStaged is the fixture for a source needing secretName and placing it in
// both an outbound header and an outbound query parameter.
func canaryStaged(source *canarySource, name string) fixture {
	return fixture{
		entry: canaryEntry(source, name),
		serve: func(route *Route, w http.ResponseWriter, r *http.Request) {
			named := station(r)
			if named == "" {
				Reject(w, InvalidParameters, "a station is required")
				return
			}
			route.Serve(w, r, named, source.server.URL+"/?station="+url.QueryEscape(named))
		},
	}
}

// canaryEntry registers a source needing secretName.
func canaryEntry(source *canarySource, name string) Entry {
	return Entry{
		Config: testConfig(),
		Source: name,
		Secret: secretName,
		InjectSecret: func(request *http.Request, secretValue string) {
			request.Header.Set(secretHeader, secretValue)
			query := request.URL.Query()
			query.Set(canaryQuery, secretValue)
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

// plant writes the canary value to a file and points the delivery variable at
// that file.
func plant(t *testing.T) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "source-key")
	if err := os.WriteFile(path, []byte(canary+"\n"), 0o600); err != nil {
		t.Fatalf("planting the secret: %v", err)
	}
	t.Setenv(secretName+"_FILE", path)
}

// TestNoSecretValueReachesAnyResponseOrLog plants a known value through the
// <NAME>_FILE delivery path and sweeps every route's response body, every
// response header value and the captured log output for it (ADR 0023 rev 2).
// TST022
func TestNoSecretValueReachesAnyResponseOrLog(t *testing.T) {
	logged := captureLog(t)

	t.Run("a resolved secret reaches the upstream and no client surface", func(t *testing.T) {
		plant(t)
		source := newCanarySource(t)
		handler := newRouter([]fixture{canaryStaged(source, "keyed")}, newFakeClock().now)

		recorder := ask(handler, moduleRouteMethod, "/api/keyed?station=one")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusOK, recorder.Body)
		}

		// Without this the sweep below passes on a backend that placed no
		// secret at all.
		header, query := source.placements()
		if header != canary {
			t.Errorf("upstream saw header %s = %q, want the planted value", secretHeader, header)
		}
		if query != canary {
			t.Errorf("upstream saw parameter %s = %q, want the planted value", canaryQuery, query)
		}

		sweep(t, "the served response", recorder, canary)
	})

	t.Run("an upstream echoing the secret in its error is not echoed on", func(t *testing.T) {
		for _, status := range []int{http.StatusUnauthorized, http.StatusInternalServerError} {
			t.Run(fmt.Sprint(status), func(t *testing.T) {
				plant(t)
				source := newCanarySource(t)
				source.answers(status, `{"error":"key `+canary+` is not valid"}`)
				handler := newRouter([]fixture{canaryStaged(source, "keyed")}, newFakeClock().now)

				recorder := ask(handler, moduleRouteMethod, "/api/keyed?station=one")

				wantFailure(t, recorder, http.StatusBadGateway, "keyed", causeUpstreamStatus)
				if header, _ := source.placements(); header != canary {
					t.Errorf("upstream saw header %s = %q, want the planted value", secretHeader, header)
				}
				sweep(t, "the echoed upstream error", recorder, canary)
			})
		}
	})

	t.Run("a transport failure carries no URL-placed secret", func(t *testing.T) {
		plant(t)
		source := newCanarySource(t)
		proxy := upstream.New(testConfig(), newFakeClock().now)
		rt := &Route{entry: canaryEntry(source, "keyed"), proxy: proxy}
		target := source.server.URL + "/?station=one"

		// The live call first, so the sweep below is known to be sweeping an
		// error raised over a URL that carried the secret.
		if _, err := rt.fetch(target)(context.Background()); err != nil {
			t.Fatalf("the live call: %v", err)
		}
		if _, query := source.placements(); query != canary {
			t.Fatalf("upstream saw parameter %s = %q, want the planted value", canaryQuery, query)
		}

		source.server.Close()
		result, err := proxy.Do(context.Background(), "keyed", "one", rt.fetch(target))
		if err != nil {
			t.Fatalf("the pipeline returned %v, want a result", err)
		}
		if result.Kind != upstream.Unreachable {
			t.Fatalf("kind = %v, want %v against a closed listener", result.Kind, upstream.Unreachable)
		}
		if result.Err == nil {
			t.Fatal("the unreachable result carries no error to sweep")
		}
		if rendered := fmt.Sprintf("%v", result.Err); strings.Contains(rendered, canary) {
			t.Errorf("the cached result's error carries the secret value: %q", rendered)
		}

		recorder := httptest.NewRecorder()
		rt.respond(recorder, result)
		sweep(t, "the transport failure", recorder, canary)
	})

	t.Run("an unresolvable secret is named and never shown", func(t *testing.T) {
		// Empty rather than absent: the variable is then unset for Resolve
		// however the ambient environment is set, and restored when the
		// subtest ends.
		t.Setenv(secretName+"_FILE", "")

		source := newCanarySource(t)
		handler := newRouter([]fixture{canaryStaged(source, "keyed")}, newFakeClock().now)

		recorder := ask(handler, moduleRouteMethod, "/api/keyed?station=one")

		decoded := wantFailure(t, recorder, http.StatusBadGateway, "keyed", causeSecretUnresolvable)
		if message, _ := decoded["message"].(string); !strings.Contains(message, secretName) {
			t.Errorf("message = %q, want the secret named", message)
		}
		if header, query := source.placements(); header != "" || query != "" {
			t.Errorf("upstream saw header %q and parameter %q, want no call at all", header, query)
		}
		sweep(t, "the unresolvable-secret failure", recorder, canary)
	})

	t.Run("the API paths no route serves", func(t *testing.T) {
		plant(t)
		source := newCanarySource(t)
		handler := newRouter([]fixture{canaryStaged(source, "keyed")}, newFakeClock().now)

		requests := []struct{ method, target string }{
			{moduleRouteMethod, "/api/unknown"},
			{moduleRouteMethod, "/api/"},
			{http.MethodGet, "/api/keyed?station=one"},
			{moduleRouteMethod, "/api/keyed"},
		}
		for _, request := range requests {
			recorder := ask(handler, request.method, request.target)
			sweep(t, request.method+" "+request.target, recorder, canary)
		}
	})

	t.Run("an outcome no case names", func(t *testing.T) {
		plant(t)
		route := &Route{entry: canaryEntry(newCanarySource(t), "keyed")}

		recorder := httptest.NewRecorder()
		// Beyond the pipeline's own set, which is the branch reaching the one
		// log line the framework writes.
		route.respond(recorder, upstream.Result{Kind: upstream.Kind(99), Err: errors.New("no outcome")})

		sweep(t, "the undistinguished failure", recorder, canary)
	})

	if collected := logged.text(); strings.Contains(collected, canary) {
		t.Errorf("the captured log output carries the secret value: %q", collected)
	}
}

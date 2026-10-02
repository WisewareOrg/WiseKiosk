package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WisewareOrg/WiseKiosk/backend/internal/staticserve"
)

const indexBody = "<!doctype html><div id=\"app\"></div>"

// assembled returns a server over a temporary tree holding an index, with api
// at the /api/ seam.
func assembled(t *testing.T, api http.Handler) http.Handler {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(indexBody), 0o644); err != nil {
		t.Fatalf("writing the index: %v", err)
	}
	return newServer(staticserve.New(http.Dir(dir)), api)
}

// send runs one request against a handler.
func send(handler http.Handler, method, target string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	return recorder
}

// get runs one GET against a handler, which is every served surface but a
// module data route.
func get(handler http.Handler, target string) *httptest.ResponseRecorder {
	return send(handler, http.MethodGet, target)
}

// healthPath is the path the boundary schema declares for liveness. It is
// asserted here rather than defined: newServer registers what the generated
// router registers, so a schema that moved the route fails this test.
const healthPath = "/healthz"

func TestRoutesHealth(t *testing.T) {
	recorder := get(assembled(t, nil), healthPath)

	if recorder.Code != http.StatusOK {
		t.Errorf("GET %s: status = %d, want %d", healthPath, recorder.Code, http.StatusOK)
	}
}

func TestRoutesTheServedTree(t *testing.T) {
	handler := assembled(t, nil)

	recorder := get(handler, "/")
	if recorder.Code != http.StatusOK || recorder.Body.String() != indexBody {
		t.Errorf("GET /: status = %d body = %q, want %d and the index", recorder.Code, recorder.Body.String(), http.StatusOK)
	}
	if recorder := get(handler, "/no-such-asset.js"); recorder.Code != http.StatusNotFound {
		t.Errorf("GET /no-such-asset.js: status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestApiIsNotFoundUntilARouterIsSupplied(t *testing.T) {
	handler := assembled(t, nil)

	for _, target := range []string{"/api/", "/api/anything", "/api/a/b/c"} {
		recorder := get(handler, target)
		if recorder.Code != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want %d", target, recorder.Code, http.StatusNotFound)
		}
		if strings.Contains(recorder.Body.String(), indexBody) {
			t.Errorf("GET %s: body = %q, want no fallback to the index", target, recorder.Body.String())
		}
	}
}

func TestApiSeamReceivesEveryApiPath(t *testing.T) {
	var reached string
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = r.URL.Path
		w.WriteHeader(http.StatusTeapot)
	})

	recorder := get(assembled(t, api), "/api/source")
	if recorder.Code != http.StatusTeapot {
		t.Errorf("status = %d, want the supplied handler's %d", recorder.Code, http.StatusTeapot)
	}
	if reached != "/api/source" {
		t.Errorf("the seam saw %q, want %q", reached, "/api/source")
	}
}

func TestRunReturnsTwoOnABadFlag(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"-no-such-flag"}, &stderr); code != 2 {
		t.Errorf("run: code = %d, want 2", code)
	}
	if stderr.Len() == 0 {
		t.Error("run: no usage written to stderr for a bad flag")
	}
}

func TestHealthCheckOriginDefaultsToTheFixedAddr(t *testing.T) {
	// The literal, not "http://localhost"+addr — the same expression the
	// production body uses would pass at a wrong addr too.
	const want = "http://localhost:8080"
	if got := healthCheckOrigin(); got != want {
		t.Errorf("healthCheckOrigin() = %q, want %q", got, want)
	}
}

func TestRunSelfCheckSucceeds(t *testing.T) {
	server := httptest.NewServer(assembled(t, nil))
	t.Cleanup(server.Close)

	original := healthCheckOrigin
	healthCheckOrigin = func() string { return server.URL }
	t.Cleanup(func() { healthCheckOrigin = original })

	var stderr bytes.Buffer
	if code := run([]string{"-health-check"}, &stderr); code != 0 {
		t.Errorf("run: code = %d, want 0; stderr = %q", code, stderr.String())
	}
}

// TestRunSelfCheckFails probes a closed listener — the origin resolved once,
// then torn down before run reaches it — the same unreachable-instance shape
// health_test.go uses for Check itself.
func TestRunSelfCheckFails(t *testing.T) {
	server := httptest.NewServer(assembled(t, nil))
	origin := server.URL
	server.Close()

	original := healthCheckOrigin
	healthCheckOrigin = func() string { return origin }
	t.Cleanup(func() { healthCheckOrigin = original })

	var stderr bytes.Buffer
	if code := run([]string{"-health-check"}, &stderr); code != 1 {
		t.Errorf("run: code = %d, want 1", code)
	}
	if stderr.Len() == 0 {
		t.Error("run: no error written to stderr for a failing self-check")
	}
}

func TestRunServesUntilTheListenerFails(t *testing.T) {
	original := serve
	wantErr := errors.New("boom")
	var gotAddr string
	serve = func(a string, h http.Handler) error {
		gotAddr = a
		return wantErr
	}
	t.Cleanup(func() { serve = original })

	var stderr bytes.Buffer
	if code := run(nil, &stderr); code != 1 {
		t.Errorf("run: code = %d, want 1", code)
	}
	if gotAddr != addr {
		t.Errorf("run: serve called with addr = %q, want %q", gotAddr, addr)
	}
	if !strings.Contains(stderr.String(), wantErr.Error()) {
		t.Errorf("run: stderr = %q, want it to contain %q", stderr.String(), wantErr.Error())
	}
}

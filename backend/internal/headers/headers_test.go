package headers

import (
	_ "embed"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WisewareOrg/WiseKiosk/backend/internal/boundary"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/health"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/registry"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/staticserve"
)

// wantCSPFile and wantPermissionsPolicyFile hold csp.txt and
// permissions-policy.txt, read independently of headers.go's own embed.
//
//go:embed csp.txt
var wantCSPFile string

//go:embed permissions-policy.txt
var wantPermissionsPolicyFile string

// schemaRoutes assembles the same schema surface backend/cmd wires into
// newServer.
type schemaRoutes struct {
	health.Route
	registry.Modules
}

var _ boundary.ServerInterface = schemaRoutes{}

// assembled serves a temporary tree holding an index and a configuration,
// wrapped the way newServer wraps it.
func assembled(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><div id=\"app\"></div>"), 0o644); err != nil {
		t.Fatalf("writing the index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("writing config.json: %v", err)
	}

	mux := http.NewServeMux()
	boundary.HandlerFromMux(schemaRoutes{}, mux)
	mux.Handle("/", staticserve.New(http.Dir(dir)))
	return Wrap(mux)
}

// rejectedWeatherBody names a point outside the module's declared constraint,
// which the module refuses before any upstream call — this test's route
// coverage is about the headers a served response carries, not a forecast.
const rejectedWeatherBody = `{"lat":999,"lon":0}`

func TestServesTheHeaderSetOnEveryRoute(t *testing.T) {
	wantCSP := strings.TrimSpace(wantCSPFile)
	wantPermissionsPolicy := strings.TrimSpace(wantPermissionsPolicyFile)
	handler := assembled(t)

	cases := []struct {
		name        string
		method      string
		target      string
		body        string
		contentType string
	}{
		{name: "the served tree", method: http.MethodGet, target: "/", contentType: "text/html; charset=utf-8"},
		{name: "the configuration", method: http.MethodGet, target: "/config.json", contentType: "application/json"},
		{name: "liveness", method: http.MethodGet, target: "/healthz", contentType: "text/plain; charset=utf-8"},
		{
			name:        "a rejected weather request",
			method:      http.MethodPost,
			target:      "/api/weather",
			body:        rejectedWeatherBody,
			contentType: "application/json",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var body io.Reader
			if c.body != "" {
				body = strings.NewReader(c.body)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(c.method, c.target, body))

			if got := recorder.Header().Get("Content-Security-Policy"); got != wantCSP {
				t.Errorf("Content-Security-Policy = %q, want %q", got, wantCSP)
			}
			if strings.Contains(recorder.Header().Get("Content-Security-Policy"), "\n") {
				t.Errorf("Content-Security-Policy carries a newline")
			}
			if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
			}
			if got := recorder.Header().Get("Permissions-Policy"); got != wantPermissionsPolicy {
				t.Errorf("Permissions-Policy = %q, want %q", got, wantPermissionsPolicy)
			}
			if got := recorder.Header().Get("Content-Type"); got != c.contentType {
				t.Errorf("Content-Type = %q, want %q", got, c.contentType)
			}
		})
	}
}

func TestPermissionsPolicyFileMatchesTheGeneration(t *testing.T) {
	generated := generatePermissionsPolicy(universe, allowlist)
	want := strings.TrimSpace(wantPermissionsPolicyFile)
	if generated != want {
		t.Errorf("permissions-policy.txt has drifted from the universe/allowlist generation:\ngenerated: %s\ntracked:   %s", generated, want)
	}
}

// cspDirectives splits a Content-Security-Policy header value into each
// directive's name and token list.
func cspDirectives(value string) map[string][]string {
	directives := make(map[string][]string)
	for _, part := range strings.Split(value, ";") {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		directives[fields[0]] = fields[1:]
	}
	return directives
}

func TestCSPConnectDefaultAndBaseAreExactlySelf(t *testing.T) {
	handler := assembled(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	directives := cspDirectives(recorder.Header().Get("Content-Security-Policy"))
	for _, name := range []string{"default-src", "connect-src", "base-uri"} {
		if got := directives[name]; len(got) != 1 || got[0] != "'self'" {
			t.Errorf("%s = %v, want exactly [\"'self'\"] — no host, wildcard, scheme-source or 'unsafe-' form admitted", name, got)
		}
	}
}

// permissionsPolicyGrants splits a Permissions-Policy header value into the
// features it grants and the features it denies.
func permissionsPolicyGrants(value string) (granted, denied map[string]bool) {
	granted = make(map[string]bool)
	denied = make(map[string]bool)
	for _, entry := range strings.Split(value, ",") {
		name, list, ok := strings.Cut(strings.TrimSpace(entry), "=")
		if !ok {
			continue
		}
		if strings.Trim(list, "()") == "" {
			denied[name] = true
		} else {
			granted[name] = true
		}
	}
	return granted, denied
}

func TestPermissionsPolicyGrantsExactlyTheAllowlist(t *testing.T) {
	handler := assembled(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	granted, denied := permissionsPolicyGrants(recorder.Header().Get("Permissions-Policy"))

	wantGranted := make(map[string]bool, len(allowlist))
	for _, feature := range allowlist {
		wantGranted[feature] = true
	}

	for feature := range wantGranted {
		if !granted[feature] {
			t.Errorf("allowlisted feature %q is not granted in the served Permissions-Policy", feature)
		}
	}
	for feature := range granted {
		if !wantGranted[feature] {
			t.Errorf("served Permissions-Policy grants %q, which is not on the allowlist", feature)
		}
	}
	for _, feature := range universe {
		if !wantGranted[feature] && !denied[feature] {
			t.Errorf("universe feature %q is served neither granted nor denied", feature)
		}
	}
}

// TestGeneratePermissionsPolicyGrantsTheAllowlist drives generatePermissionsPolicy
// directly with a synthetic universe/allowlist — the production allowlist is
// empty, so this is the only case that reaches the grant branch.
func TestGeneratePermissionsPolicyGrantsTheAllowlist(t *testing.T) {
	got := generatePermissionsPolicy([]string{"b", "a"}, []string{"b"})
	want := "a=(), b=(self)"
	if got != want {
		t.Errorf("generatePermissionsPolicy = %q, want %q", got, want)
	}
}

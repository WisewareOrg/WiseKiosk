package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"github.com/WisewareOrg/WiseKiosk/backend/internal/secret"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/upstream"
)

// fakeClock is the clock the routes read, advanced by a test rather than waited
// on: no test here sleeps for a TTL.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// upstreamFake is the source the test entries point at. It counts the calls
// that reached it, which is how a test tells an answer that went upstream from
// one that did not, and keeps the last request's headers.
type upstreamFake struct {
	server *httptest.Server

	mu      sync.Mutex
	calls   int
	headers http.Header
	status  int
	body    string
	release chan struct{}
}

func newUpstreamFake(t *testing.T) *upstreamFake {
	t.Helper()

	fake := &upstreamFake{status: http.StatusOK, body: `{"reading":42}`}
	fake.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fake.mu.Lock()
		fake.calls++
		fake.headers = r.Header.Clone()
		status, body, release := fake.status, fake.body, fake.release
		fake.mu.Unlock()

		// Counted before the wait, so a call that reached the source is counted
		// whether or not it has been let go.
		if release != nil {
			<-release
		}

		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(fake.server.Close)
	return fake
}

// hold stages a slow source: every call reaches it and then waits, until the
// returned function is called. Calling that function twice is harmless, so a
// test releases the source itself and registers the same call as its cleanup.
func (u *upstreamFake) hold() func() {
	release := make(chan struct{})

	u.mu.Lock()
	u.release = release
	u.mu.Unlock()

	return sync.OnceFunc(func() { close(release) })
}

func (u *upstreamFake) count() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.calls
}

func (u *upstreamFake) header(name string) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.headers.Get(name)
}

func (u *upstreamFake) answers(status int, body string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.status, u.body = status, body
}

// testConfig is the policy the test entries run under. Every value is set, as
// an entry's must be.
func testConfig() upstream.Config {
	return upstream.Config{
		SuccessTTL:        10 * time.Minute,
		NegativeTTL:       time.Minute,
		RequestsPerMinute: 60,
		Burst:             10,
		Timeout:           5 * time.Second,
		MaxBytes:          1 << 16,
	}
}

// testEntry is a fake module's registration: the policy the route runs under,
// and the payload as the upstream body read as an object. What a request names
// is not the entry's — it is the fixture's, below.
func testEntry(fake *upstreamFake, source string) Entry {
	return Entry{
		Config: testConfig(),
		Source: source,
		Shape: func(body []byte) (any, error) {
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				return nil, err
			}
			return payload, nil
		},
	}
}

// fixture is one fake module: the entry the framework serves it under, and the
// half the framework does not hold — what the module makes of a request,
// which is either a rejection of its own or the two strings a route is served
// with. A real module reads its generated request body; a fixture reads a query,
// and the framework cannot tell the difference because it reads neither.
type fixture struct {
	entry Entry
	serve func(route *Route, w http.ResponseWriter, r *http.Request)
}

// station is what a fixture request names, and what the answer to it is held
// under.
func station(r *http.Request) string {
	return r.URL.Query().Get("station")
}

// staged is the fixture for source, answering from fake: a request naming no
// station is the module's own rejection, and one that names a station is served
// under that station from the fake upstream.
func staged(fake *upstreamFake, source string) fixture {
	return fixture{
		entry: testEntry(fake, source),
		serve: func(route *Route, w http.ResponseWriter, r *http.Request) {
			named := station(r)
			if named == "" {
				Reject(w, InvalidParameters, "a station is required")
				return
			}
			route.Serve(w, r, named, fake.server.URL+"/?station="+url.QueryEscape(named))
		},
	}
}

// newRouter assembles a mux the way the process assembles one: a module data
// route per fixture, registered under the method the boundary schema declares
// them with, and the framework's seam over the rest of the API path space.
//
// The framework ships no such constructor — a module registers its own route
// through the generated server interface, and the composite is the assembly
// point's — so what a case needs is built here rather than kept in the package
// under test.
func newRouter(fixtures []fixture, now func() time.Time) http.Handler {
	mux := http.NewServeMux()
	for _, f := range fixtures {
		route := newRoute(f.entry, now)
		serve := f.serve
		mux.Handle(moduleRouteMethod+" "+apiPrefix+f.entry.Source,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { serve(route, w, r) }))
	}

	mux.Handle(apiPrefix, NewFallback(mux))
	return mux
}

// ask runs one request against a handler.
func ask(handler http.Handler, method, target string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	return recorder
}

// body reads a response as a JSON object, so a test asserts on the fields that
// are there rather than on the type it expected.
func body(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	if got := recorder.Header().Get("Content-Type"); got != contentTypeJSON {
		t.Errorf("Content-Type = %q, want %q", got, contentTypeJSON)
	}

	var decoded map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("reading the body %q: %v", recorder.Body.String(), err)
	}
	return decoded
}

// wantRejection asserts a client-rejection body: the cause, and the absence of
// the module field that would make it an upstream failure instead.
func wantRejection(t *testing.T, recorder *httptest.ResponseRecorder, status int, cause string) {
	t.Helper()

	if recorder.Code != status {
		t.Errorf("status = %d, want %d", recorder.Code, status)
	}

	decoded := body(t, recorder)
	if decoded["cause"] != cause {
		t.Errorf("cause = %v, want %q", decoded["cause"], cause)
	}
	if message, _ := decoded["message"].(string); message == "" {
		t.Error("the rejection carries no message to render")
	}
	if _, carried := decoded["module"]; carried {
		t.Errorf("the rejection carries a module field: %v", decoded)
	}
}

// wantFailure asserts an upstream-failure body: the module it is contained to,
// and the cause.
func wantFailure(t *testing.T, recorder *httptest.ResponseRecorder, status int, module, cause string) map[string]any {
	t.Helper()

	if recorder.Code != status {
		t.Errorf("status = %d, want %d", recorder.Code, status)
	}

	decoded := body(t, recorder)
	if decoded["module"] != module {
		t.Errorf("module = %v, want %q", decoded["module"], module)
	}
	if decoded["cause"] != cause {
		t.Errorf("cause = %v, want %q", decoded["cause"], cause)
	}
	if message, _ := decoded["message"].(string); message == "" {
		t.Error("the failure carries no message to render")
	}
	return decoded
}

// TST025
func TestAnIdenticalRequestIsServedFromCacheUntilTheTTLExpires(t *testing.T) {
	clock := newFakeClock()
	fake := newUpstreamFake(t)
	staged := staged(fake, "readings")
	handler := newRouter([]fixture{staged}, clock.now)

	first := ask(handler, http.MethodPost, "/api/readings?station=one")
	if first.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want %d (%s)", first.Code, http.StatusOK, first.Body)
	}

	second := ask(handler, http.MethodPost, "/api/readings?station=one")
	if second.Body.String() != first.Body.String() {
		t.Errorf("second body = %q, want the first's %q", second.Body, first.Body)
	}
	if calls := fake.count(); calls != 1 {
		t.Errorf("upstream calls within the TTL = %d, want 1", calls)
	}

	clock.advance(staged.entry.SuccessTTL)
	third := ask(handler, http.MethodPost, "/api/readings?station=one")
	if third.Code != http.StatusOK {
		t.Errorf("after the TTL: status = %d, want %d", third.Code, http.StatusOK)
	}
	if calls := fake.count(); calls != 2 {
		t.Errorf("upstream calls after the TTL = %d, want 2", calls)
	}
}

// TestNonConformingParametersAreRejectedWithNoUpstreamCall reads the rejection
// contract rather than any judgement: what the module makes of a request is the
// module's, and what this asserts is that the answer it is given to write is the
// client rejection the boundary declares and that nothing goes upstream behind
// it. Which values a source admits is that module's own item to fail.
// TST029
func TestNonConformingParametersAreRejectedWithNoUpstreamCall(t *testing.T) {
	fake := newUpstreamFake(t)
	handler := newRouter([]fixture{staged(fake, "readings")}, newFakeClock().now)

	recorder := ask(handler, moduleRouteMethod, "/api/readings")

	wantRejection(t, recorder, http.StatusBadRequest, InvalidParameters)
	if calls := fake.count(); calls != 0 {
		t.Errorf("upstream calls = %d, want none", calls)
	}
}

const (
	// secretName is the logical name the keyed test entry declares.
	secretName = "WISEKIOSK_TEST_SOURCE_KEY"
	// secretHeader is where the keyed test entry places the value.
	secretHeader = "X-Test-Source-Key"
	// canary is the value planted through the delivery path and swept for.
	canary = canarytest.Value
)

// keyedEntry is a test entry needing secretName, placed as a header.
func keyedEntry(fake *upstreamFake, source string) Entry {
	entry := testEntry(fake, source)
	entry.Secret = secretName
	entry.InjectSecret = func(request *http.Request, secretValue string) {
		request.Header.Set(secretHeader, secretValue)
	}
	return entry
}

// stagedKeyed is the staged fixture for a source needing secretName.
func stagedKeyed(fake *upstreamFake, source string) fixture {
	keyed := staged(fake, source)
	keyed.entry = keyedEntry(fake, source)
	return keyed
}

// writeSecretFile writes contents to a named file under a fresh directory at
// the given mode and returns the path. The mode is the case's subject where it
// denies the read, so it is set on the write rather than after it.
func writeSecretFile(t *testing.T, name, contents string, mode os.FileMode) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

// TST017
func TestAnUnresolvableSecretFailsOnlyItsOwnSource(t *testing.T) {
	// One row per cause secret.Resolve distinguishes, so a cause given its own
	// handling later cannot lose its route behaviour. setUp returns the path the
	// delivery variable is pointed at, empty where the case leaves it unset.
	cases := []struct {
		name  string
		setUp func(t *testing.T) string
	}{
		{
			// Empty rather than absent: the variable is then unset for Resolve
			// however the ambient environment is set, and restored when the
			// subtest ends.
			name:  "the path is unset",
			setUp: func(t *testing.T) string { return "" },
		},
		{
			name:  "the file is missing",
			setUp: func(t *testing.T) string { return filepath.Join(t.TempDir(), "no-such-secret") },
		},
		{
			name: "the file is unreadable",
			setUp: func(t *testing.T) string {
				if os.Geteuid() == 0 {
					t.Skip("running as root: mode bits do not deny a read")
				}
				return writeSecretFile(t, "unreadable", canary, 0o000)
			},
		},
		{
			name: "the file is empty after trailing-whitespace stripping",
			setUp: func(t *testing.T) string {
				return writeSecretFile(t, "blank", " \t\r\n\n", 0o600)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := c.setUp(t)
			t.Setenv(secretName+"_FILE", path)

			fake := newUpstreamFake(t)
			handler := newRouter([]fixture{stagedKeyed(fake, "keyed"), staged(fake, "open")}, newFakeClock().now)

			failed := ask(handler, http.MethodPost, "/api/keyed?station=one")
			decoded := wantFailure(t, failed, http.StatusBadGateway, "keyed", causeSecretUnresolvable)
			if message, _ := decoded["message"].(string); !strings.Contains(message, secretName) {
				t.Errorf("message = %q, want the secret named", message)
			}
			if path != "" && strings.Contains(failed.Body.String(), path) {
				t.Errorf("the failure carries the path the secret was looked for at: %q", failed.Body)
			}
			// The unreadable case's file holds the canary, so this is the
			// contents of an unresolvable secret reaching the wire.
			if strings.Contains(failed.Body.String(), canary) {
				t.Errorf("the failure carries the contents of the file: %q", failed.Body)
			}
			if calls := fake.count(); calls != 0 {
				t.Errorf("upstream calls for the keyed source = %d, want none", calls)
			}

			served := ask(handler, http.MethodPost, "/api/open?station=one")
			if served.Code != http.StatusOK {
				t.Errorf("the source needing no secret: status = %d, want %d (%s)", served.Code, http.StatusOK, served.Body)
			}
			if reading := body(t, served)["reading"]; reading != float64(42) {
				t.Errorf("the source needing no secret: reading = %v, want the upstream's", reading)
			}
		})
	}
}

func TestAResolvedSecretReachesUpstreamAndNoResponse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source-key")
	if err := os.WriteFile(path, []byte(canary+"\n"), 0o600); err != nil {
		t.Fatalf("planting the secret: %v", err)
	}
	t.Setenv(secretName+"_FILE", path)

	fake := newUpstreamFake(t)
	handler := newRouter([]fixture{stagedKeyed(fake, "keyed")}, newFakeClock().now)

	recorder := ask(handler, http.MethodPost, "/api/keyed?station=one")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	// Without this the sweep below passes on a backend that never placed the
	// secret at all.
	if placed := fake.header(secretHeader); placed != canary {
		t.Errorf("upstream saw %s = %q, want the planted value", secretHeader, placed)
	}

	if strings.Contains(recorder.Body.String(), canary) {
		t.Errorf("the response body carries the secret value: %q", recorder.Body)
	}
	for name, values := range recorder.Header() {
		for _, value := range values {
			if strings.Contains(value, canary) {
				t.Errorf("response header %s carries the secret value: %q", name, value)
			}
		}
	}
}

func TestASecretDoesNotFollowARedirect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source-key")
	if err := os.WriteFile(path, []byte(canary+"\n"), 0o600); err != nil {
		t.Fatalf("planting the secret: %v", err)
	}
	t.Setenv(secretName+"_FILE", path)

	elsewhere := newUpstreamFake(t)
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.server.URL, http.StatusFound)
	}))
	t.Cleanup(redirecting.Close)

	// The module hands the framework the redirecting server, so what the secret
	// is placed on is the call this route makes rather than the one it is sent on
	// to.
	keyed := stagedKeyed(elsewhere, "keyed")
	keyed.serve = func(route *Route, w http.ResponseWriter, r *http.Request) {
		route.Serve(w, r, station(r), redirecting.URL)
	}
	handler := newRouter([]fixture{keyed}, newFakeClock().now)

	wantFailure(t, ask(handler, http.MethodPost, "/api/keyed?station=one"),
		http.StatusBadGateway, "keyed", causeUpstreamStatus)

	if calls := elsewhere.count(); calls != 0 {
		t.Errorf("the redirect target was called %d times, want none", calls)
	}
	if placed := elsewhere.header(secretHeader); placed != "" {
		t.Errorf("the redirect target saw %s = %q, want the secret to have stayed behind", secretHeader, placed)
	}
}

// TST029
func TestARouteAnswersTheModuleRouteMethodOnly(t *testing.T) {
	fake := newUpstreamFake(t)
	handler := newRouter([]fixture{staged(fake, "readings")}, newFakeClock().now)

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete} {
		recorder := ask(handler, method, "/api/readings?station=one")

		wantRejection(t, recorder, http.StatusMethodNotAllowed, causeMethodNotAllowed)
		if allow := recorder.Header().Get("Allow"); allow != allowedMethods {
			t.Errorf("%s: Allow = %q, want %q", method, allow, allowedMethods)
		}
	}

	if calls := fake.count(); calls != 0 {
		t.Errorf("upstream calls = %d, want none", calls)
	}

	// The method the 405 above advertises, so a route refusing it would be
	// refusing the one method it names.
	if served := ask(handler, allowedMethods, "/api/readings?station=one"); served.Code != http.StatusOK {
		t.Errorf("%s: status = %d, want %d (%s)", allowedMethods, served.Code, http.StatusOK, served.Body)
	}
}

// TST029
func TestAPathNoEntryRegisteredIsUnknown(t *testing.T) {
	fake := newUpstreamFake(t)
	handler := newRouter([]fixture{staged(fake, "readings")}, newFakeClock().now)

	for _, target := range []string{"/api/unknown", "/api/", "/api/readings/extra"} {
		wantRejection(t, ask(handler, http.MethodPost, target), http.StatusNotFound, causeUnknownSource)
	}

	if calls := fake.count(); calls != 0 {
		t.Errorf("upstream calls = %d, want none", calls)
	}
}

// TST027, TST029
func TestARequestOverTheRateLimitIsRejected(t *testing.T) {
	fake := newUpstreamFake(t)
	limited := staged(fake, "readings")
	limited.entry.RequestsPerMinute, limited.entry.Burst = 1, 1
	handler := newRouter([]fixture{limited}, newFakeClock().now)

	if served := ask(handler, http.MethodPost, "/api/readings?station=one"); served.Code != http.StatusOK {
		t.Fatalf("the first request: status = %d, want %d (%s)", served.Code, http.StatusOK, served.Body)
	}

	// A second station, so the answer is decided by the bucket rather than by
	// the cache.
	refused := ask(handler, http.MethodPost, "/api/readings?station=two")

	wantRejection(t, refused, http.StatusTooManyRequests, causeRateLimited)
	if calls := fake.count(); calls != 1 {
		t.Errorf("upstream calls = %d, want the one the bucket had a token for", calls)
	}
}

func TestAFailedExchangeIsThisModulesFailure(t *testing.T) {
	fake := newUpstreamFake(t)
	handler := newRouter([]fixture{staged(fake, "readings")}, newFakeClock().now)

	fake.answers(http.StatusServiceUnavailable, "upstream is down")
	decoded := wantFailure(t, ask(handler, http.MethodPost, "/api/readings?station=one"),
		http.StatusBadGateway, "readings", causeUpstreamStatus)
	if message, _ := decoded["message"].(string); !strings.Contains(message, "503") {
		t.Errorf("message = %q, want the upstream status named", message)
	}

	fake.answers(http.StatusOK, "this is not JSON")
	wantFailure(t, ask(handler, http.MethodPost, "/api/readings?station=two"),
		http.StatusBadGateway, "readings", causeMalformedPayload)
}

func TestASourceThatCannotBeReachedIsThisModulesFailure(t *testing.T) {
	fake := newUpstreamFake(t)
	handler := newRouter([]fixture{staged(fake, "readings")}, newFakeClock().now)
	fake.server.Close()

	wantFailure(t, ask(handler, http.MethodPost, "/api/readings?station=one"),
		http.StatusBadGateway, "readings", causeUnreachable)
}

// TestACallerWhoseContextEndedIsStillAnswered ends the context directly rather
// than through a server, so it covers the outcome both paths share: the 503
// ADR 0026 rev 2 defines, where returning unwritten would emit an empty 200 and
// reusing an upstream cause would blame an upstream that was never called.
func TestACallerWhoseContextEndedIsStillAnswered(t *testing.T) {
	fake := newUpstreamFake(t)

	// The staged source holds every call, so the pipeline has no result to hand
	// back and the ended context is what decides the answer.
	release := fake.hold()
	t.Cleanup(release)

	rt := newRoute(testEntry(fake, "readings"), newFakeClock().now)
	ended, cancel := context.WithCancel(context.Background())
	cancel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(moduleRouteMethod, "/api/readings?station=one", nil).WithContext(ended)
	rt.Serve(recorder, request, "one", fake.server.URL)

	wantFailure(t, recorder, http.StatusServiceUnavailable, "readings", CauseShuttingDown)
}

// TST029
func TestEveryOutcomeCarriesItsOwnStatusAndCause(t *testing.T) {
	// The path the secret was looked for at, which the wire message must not
	// carry.
	const lookedFor = "/run/secrets/source-key"

	rt := &Route{entry: keyedEntry(newUpstreamFake(t), "keyed")}
	cases := []struct {
		name   string
		result upstream.Result
		status int
		cause  string
	}{
		{"unreachable", upstream.Result{Kind: upstream.Unreachable, Err: errors.New("dial: connection refused")},
			http.StatusBadGateway, causeUnreachable},
		{"timeout", upstream.Result{Kind: upstream.Timeout, Err: context.DeadlineExceeded},
			http.StatusGatewayTimeout, causeTimeout},
		{"upstream status", upstream.Result{Kind: upstream.UpstreamStatus, Status: http.StatusServiceUnavailable},
			http.StatusBadGateway, causeUpstreamStatus},
		{"oversize", upstream.Result{Kind: upstream.Oversize, Err: errors.New("over the size bound")},
			http.StatusBadGateway, causeOversize},
		{"unresolvable secret", upstream.Result{Kind: upstream.Unreachable, Err: &secret.UnresolvableError{
			Name: secretName, Path: lookedFor, Cause: secret.ErrFileMissing}},
			http.StatusBadGateway, causeSecretUnresolvable},
	}

	seen := make(map[string]string, len(cases))
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, cause, message := rt.failure(c.result)

			if status != c.status {
				t.Errorf("status = %d, want %d", status, c.status)
			}
			if cause != c.cause {
				t.Errorf("cause = %q, want %q", cause, c.cause)
			}
			if message == "" {
				t.Error("the failure carries no message to render")
			}
			if strings.Contains(message, lookedFor) {
				t.Errorf("message = %q, want no path in it", message)
			}
			if c.cause == causeSecretUnresolvable && !strings.Contains(message, secretName) {
				t.Errorf("message = %q, want the secret named", message)
			}
			if first, taken := seen[cause]; taken {
				t.Errorf("cause %q is already %s's, so the two are not told apart", cause, first)
			}
			seen[cause] = c.name
		})
	}
}

// TestARequestBodyIsReadNoFurtherThanTheBound covers the cap on what a module
// route reads of a request: how much this process can be made to buffer is the
// framework's to decide, not a caller's. It is read against a handler that reads
// the body whole rather than through a module, because a module's own decoder
// refuses a long body for reasons of its own and would pass this either way.
func TestARequestBodyIsReadNoFurtherThanTheBound(t *testing.T) {
	// read returns what a handler applying the bound got of a body of size bytes.
	read := func(size int) (int, error) {
		var got int
		var err error
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			BoundBody(w, r)
			body, readErr := io.ReadAll(r.Body)
			got, err = len(body), readErr
		})
		handler.ServeHTTP(httptest.NewRecorder(),
			httptest.NewRequest(moduleRouteMethod, "/api/readings", strings.NewReader(strings.Repeat("x", size))))
		return got, err
	}

	// Inside the bound first: without this the case below passes on a bound that
	// refuses every body there is, which reads as working and serves nothing.
	if got, err := read(maxRequestBytes); err != nil || got != maxRequestBytes {
		t.Errorf("a body of the %d bytes the bound allows read %d bytes and %v, want it read whole",
			maxRequestBytes, got, err)
	}

	got, err := read(maxRequestBytes + 1)
	if err == nil {
		t.Errorf("a body one byte past the bound read %d bytes and no error, want the read to stop", got)
	}
	if got > maxRequestBytes {
		t.Errorf("%d bytes were read, want no more than the %d the bound allows", got, maxRequestBytes)
	}
}

// TST029
func TestNoTwoCausesShareASpelling(t *testing.T) {
	// Every cause constant, compared against every other. The sweep above reads
	// only what failure() returns, and CauseShuttingDown and the rejection
	// causes leave by other paths; each one's own test compares a response
	// against the same constant it asserts, so consolidating two onto one
	// string stays green everywhere but here (ADR 0026 rev 2).
	all := []struct {
		name  string
		cause string
	}{
		{"InvalidParameters", InvalidParameters},
		{"causeUnknownSource", causeUnknownSource},
		{"causeMethodNotAllowed", causeMethodNotAllowed},
		{"causeRateLimited", causeRateLimited},
		{"causeUnreachable", causeUnreachable},
		{"causeTimeout", causeTimeout},
		{"causeUpstreamStatus", causeUpstreamStatus},
		{"causeOversize", causeOversize},
		{"causeSecretUnresolvable", causeSecretUnresolvable},
		{"causeMalformedPayload", causeMalformedPayload},
		{"causeUpstreamFailure", causeUpstreamFailure},
		{"CauseShuttingDown", CauseShuttingDown},
	}

	seen := make(map[string]string, len(all))
	for _, c := range all {
		if c.cause == "" {
			t.Errorf("%s is empty, so it names no outcome on the wire", c.name)
			continue
		}
		if first, taken := seen[c.cause]; taken {
			t.Errorf("%s and %s are both %q, so the two outcomes are not told apart", first, c.name, c.cause)
		}
		seen[c.cause] = c.name
	}
}

// TST029
func TestAnOutcomeNoCaseNamesIsStillRenderable(t *testing.T) {
	var logged bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(previous) })

	rt := &Route{entry: testEntry(newUpstreamFake(t), "readings")}

	// Beyond the pipeline's own set, which is where a kind added to it later
	// arrives here.
	status, cause, message := rt.failure(upstream.Result{Kind: upstream.Kind(99)})

	if status != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", status, http.StatusBadGateway)
	}
	if cause != causeUpstreamFailure {
		t.Errorf("cause = %q, want %q", cause, causeUpstreamFailure)
	}
	if message == "" {
		t.Error("the failure carries no message to render")
	}
	if !strings.Contains(logged.String(), "readings") {
		t.Errorf("logged %q, want the source named", logged.String())
	}
}

func TestAnEntryTheFrameworkCannotServePanicsAtConstruction(t *testing.T) {
	fake := newUpstreamFake(t)

	cases := map[string]func(Entry) Entry{
		"no source":                  func(entry Entry) Entry { entry.Source = ""; return entry },
		"no shaping function":        func(entry Entry) Entry { entry.Shape = nil; return entry },
		"a secret it does not place": func(entry Entry) Entry { entry.Secret = secretName; return entry },
		"a placement for a secret it does not name": func(entry Entry) Entry {
			entry.InjectSecret = func(*http.Request, string) {}
			return entry
		},
	}

	for name, break_ := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered == nil {
					t.Error("construction returned, want a panic")
				}
			}()
			newRoute(break_(testEntry(fake, "readings")), newFakeClock().now)
		})
	}
}

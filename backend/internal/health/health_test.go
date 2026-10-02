package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/WisewareOrg/WiseKiosk/backend/internal/boundary"
)

// schemaRoutes carries this package's route into the generated router, which
// registers the schema's whole set at once. The embedded interface supplies
// every other route, is nil, and is reached by nothing here — a route this
// package does not answer is one no case below asks for, and naming any of them
// would put another package's path in this one's tests.
type schemaRoutes struct {
	boundary.ServerInterface
}

func (schemaRoutes) GetHealthz(w http.ResponseWriter, r *http.Request) {
	Route{}.GetHealthz(w, r)
}

// served is an instance answering the schema's routes the way the process does
// — through the generated router — so Check meets the path it will meet in a
// container rather than one the test wrote.
func served(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(boundary.HandlerFromMux(schemaRoutes{}, http.NewServeMux()))
	t.Cleanup(server.Close)
	return server
}

func TestRouteReportsServing(t *testing.T) {
	recorder := httptest.NewRecorder()
	Route{}.GetHealthz(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Body.Len() != 0 {
		t.Errorf("body = %q, want none — the schema declares no body", recorder.Body.String())
	}
}

func TestCheckAcceptsA200(t *testing.T) {
	if err := Check(served(t).URL); err != nil {
		t.Errorf("Check: unexpected error: %v", err)
	}
}

func TestCheckRefusesAnotherStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	if err := Check(server.URL); err == nil {
		t.Error("Check: no error for a 503")
	}
}

// TestCheckRefusesAWedgedInstance is the case a connection-refused test cannot
// reach: the listener accepts and the handler never answers, which is a process
// alive but wedged.
func TestCheckRefusesAWedgedInstance(t *testing.T) {
	wedged := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-wedged
	}))
	t.Cleanup(func() {
		close(wedged)
		server.Close()
	})

	// Off the test's own goroutine, so a Check with no bound fails here rather
	// than hanging until the whole package times out. The wait is twice the
	// bound, which asserts Check has one at all rather than measuring a loaded
	// machine's scheduling.
	returned := make(chan error, 1)
	go func() { returned <- Check(server.URL) }()

	select {
	case err := <-returned:
		if err == nil {
			t.Error("Check: no error against a server that never answers")
		}
	case <-time.After(2 * checkTimeout):
		t.Errorf("Check did not return within %s, want it bounded by the %s timeout", 2*checkTimeout, checkTimeout)
	}
}

func TestCheckRefusesAnUnreachableInstance(t *testing.T) {
	server := httptest.NewServer(boundary.HandlerFromMux(schemaRoutes{}, http.NewServeMux()))
	origin := server.URL
	server.Close()

	if err := Check(origin); err == nil {
		t.Error("Check: no error against a closed listener")
	}
}

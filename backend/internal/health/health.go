// Package health answers process liveness over HTTP, and asks the same of a
// running instance from inside the image (ADR 0020 rev 4). Both halves go
// through the generated boundary package, so the route they meet on is the
// schema's rather than a path written here (ADR 0008 rev 6).
package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/WisewareOrg/WiseKiosk/backend/internal/boundary"
)

// checkTimeout bounds one probe end to end. The probe is a loopback request to
// a handler that consults nothing, so it answers in microseconds or not at all.
const checkTimeout = 2 * time.Second

// checker is the client Check probes with, bounded where http.DefaultClient is
// not.
var checker = &http.Client{Timeout: checkTimeout}

// Route serves the boundary schema's liveness path. It satisfies the generated
// boundary.ServerInterface, which is what registers the path — nothing here
// names it.
type Route struct{}

// GetHealthz reports that the process is serving and nothing else: it reads no
// configuration and consults no dependency (ADR 0007 rev 2). The 200 carries no
// body, because the schema declares none and every consumer reads the status.
// A bodiless response still declares a type
// (SRS028<!-- Served responses declare their type, and forbid the browser inferring one -->),
// set before WriteHeader since a header written after is dropped.
func (Route) GetHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}

// Check asks the instance served at origin whether it is serving, returning nil
// for a 200 and an error for anything else — no response, a response carrying
// another status, or no answer within checkTimeout. The last is a process alive
// but wedged, which Check reports itself rather than leaving to a deployment
// artifact to bound.
func Check(origin string) error {
	client, err := boundary.NewClientWithResponses(origin, boundary.WithHTTPClient(checker))
	if err != nil {
		return fmt.Errorf("health check %s: %w", origin, err)
	}

	response, err := client.GetHealthzWithResponse(context.Background())
	if err != nil {
		return fmt.Errorf("health check %s: %w", origin, err)
	}

	if response.StatusCode() != http.StatusOK {
		return fmt.Errorf("health check %s: answered %s", origin, response.Status())
	}
	return nil
}

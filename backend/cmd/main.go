// The WiseKiosk backend: one origin serving the frontend bundle, whatever an
// operator has mounted beside it, and the API.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/WisewareOrg/WiseKiosk/backend/internal/boundary"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/headers"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/health"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/registry"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/router"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/staticserve"
)

const (
	// addr is the service port, fixed by ADR 0020 rev 4 along with the two
	// flags main reads.
	addr = ":8080"
	// defaultStaticRoot is the bundle location for a run from the repository
	// root; a container points -static-root at its own.
	defaultStaticRoot = "frontend/dist"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run parses flags out of args and either performs the self-check or serves,
// returning the process exit code. It is kept apart from main so it can run
// under test with its own flag set and injected stderr — main() itself
// cannot: it either blocks serving or exits the test binary.
//
// serve and healthCheckOrigin below are the seams a test overrides to reach
// both branches without binding the fixed port.
func run(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("wisekiosk", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("static-root", defaultStaticRoot, "directory served as the frontend bundle")
	selfCheck := fs.Bool("health-check", false, "ask the local instance for liveness and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *selfCheck {
		if err := health.Check(healthCheckOrigin()); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}

	err := serve(addr, newServer(staticserve.New(http.Dir(*root)), nil))
	_, _ = fmt.Fprintln(stderr, err)
	return 1
}

// serve is the seam a test overrides to avoid binding the fixed port.
var serve = func(addr string, h http.Handler) error {
	return http.ListenAndServe(addr, h)
}

// healthCheckOrigin is the seam a test overrides to point the -health-check
// probe at an httptest origin instead of the fixed addr.
var healthCheckOrigin = func() string {
	return "http://localhost" + addr
}

// newServer assembles the routes: the boundary schema's own, the seam over the
// rest of the API path space, and the served tree for everything else. A nil
// seam is the framework's own, which answers for every API path the schema
// declares no route at. The schema's paths are registered by the generated
// router, so no path it declares is written here (ADR 0008 rev 6).
//
// A module data route's generated pattern is more specific than the /api/ seam,
// so it takes that path — the schema decides where a route is and the module
// embedded below decides what it does. The generated wrapper reads nothing of
// the request on the way: a module route's inputs are its JSON body, which the
// module reads itself, so the wrapper binds nothing and has no rejection of its
// own to write.
//
// headers.Wrap sits over the whole mux, so every response — the served tree,
// the seam and every schema route alike — carries the same response headers.
func newServer(static, seam http.Handler) http.Handler {
	mux := http.NewServeMux()
	boundary.HandlerFromMux(schemaRoutes{}, mux)
	if seam == nil {
		seam = router.NewFallback(mux)
	}
	mux.Handle("/api/", seam)
	mux.Handle("/", static)
	return headers.Wrap(mux)
}

// schemaRoutes is the whole of the boundary schema's server interface. The
// generated interface is one, and its two kinds of path are owned by different
// packages — an infrastructure route by the package that answers it, every
// module data route by the module itself through the registry — so the two are
// composed at the assembly point rather than in either of them.
type schemaRoutes struct {
	health.Route
	registry.Modules
}

// The tie the composite exists for, asserted where it is read rather than at the
// call above: a schema route no embedded type serves is a missing method here.
var _ boundary.ServerInterface = schemaRoutes{}

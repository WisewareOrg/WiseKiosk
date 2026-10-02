package registry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WisewareOrg/WiseKiosk/backend/internal/boundary"
)

// schemaRoutes carries this package's module data routes into the generated
// router, which registers the schema's whole set at once. GetHealthz stands in
// for the schema's infrastructure routes, which are another package's and are
// asked for by nothing here.
type schemaRoutes struct {
	Modules
}

func (schemaRoutes) GetHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// registered returns the generated router over this package's modules,
// assembled the way the process assembles it.
func registered() http.Handler {
	mux := http.NewServeMux()
	boundary.HandlerFromMux(schemaRoutes{}, mux)
	return mux
}

// TestAModuleDataRouteReachesTheModulesOwnRouteUntouched reads the seam
// ADR 0008 rev 6 decides: the schema registers the path, the generated interface
// obliges the method, and the method the embedded field promotes is the module's
// own. The request carries a body naming a point no source will carry upstream,
// so what comes back is a rejection only the module could have written — which
// says both that the path reached that module and that the body it judged is the
// one that arrived. No outbound call is made either way.
func TestAModuleDataRouteReachesTheModulesOwnRouteUntouched(t *testing.T) {
	const beyondEveryRange = `{"lat":999,"lon":999}`

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/weather", strings.NewReader(beyondEveryRange))
	registered().ServeHTTP(recorder, request)

	if recorder.Code == http.StatusNotFound {
		t.Fatalf("POST /api/weather is a path the schema declares and no module serves (%s)", recorder.Body)
	}
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want the module's own rejection %d (%s)", recorder.Code, http.StatusBadRequest, recorder.Body)
	}
}

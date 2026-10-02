package weather

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"

	"github.com/WisewareOrg/WiseKiosk/backend/internal/boundary"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/router"
)

// entry is this module's route registration: the module contract's part 5,
// assembled from the shaping library beside it rather than restated anywhere
// shared, so the figures a requirement settled have one home.
func entry() router.Entry {
	return router.Entry{
		Config: Config(),
		Source: Source,
		// The module shapes into the generated payload type, which the framework
		// takes as the JSON-encodable value any module may return.
		Shape: func(body []byte) (any, error) { return Shape(body) },
		// Open-Meteo is keyless, so there is nothing to name and nothing to place.
	}
}

// served is the framework route built from that entry, one per process. An entry
// the framework cannot serve panics here, before the process serves.
var served = router.NewRoute(entry())

// WeatherRoute is this module's whole footprint in the shared tree: the registry
// embeds it, and the generated server interface is what obliges the method below
// to exist. It is usable at its zero value, so registering the module needs no
// call and names it once.
type WeatherRoute struct{}

// PostApiWeather serves the schema's POST /api/weather. What a request carries
// is the generated request body, so the two names in it are the schema's and are
// spelled nowhere in this package; what this module judges is the point they
// stand for
// (SRS043<!-- The weather module declares the known-good constraint the location it is asked about must satisfy -->).
func (WeatherRoute) PostApiWeather(w http.ResponseWriter, r *http.Request) {
	router.BoundBody(w, r)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		router.Reject(w, router.InvalidParameters, errRequestNotDecodable.Error())
		return
	}
	request, err := decodeRequest(body)
	if err != nil {
		router.Reject(w, router.InvalidParameters, err.Error())
		return
	}
	if err := validate(request); err != nil {
		router.Reject(w, router.InvalidParameters, err.Error())
		return
	}
	served.Serve(w, r, key(request), buildURL(request))
}

// errRequestNotDecodable and errRequestMissingCoordinate are decodeRequest's
// two sentinel failures, carrying the two 400 message texts the handler
// rejects with.
var (
	errRequestNotDecodable      = errors.New("the request body could not be read as this source's parameters")
	errRequestMissingCoordinate = errors.New("the request must name both a latitude and a longitude")
)

// decodeRequest reads the point a request names. It is pure — bytes in, the
// request or a sentinel error out.
func decodeRequest(body []byte) (boundary.WeatherRequest, error) {
	// Pre-set to NaN: a coordinate still NaN after a decode is one the body did
	// not carry, JSON having no NaN literal to write, which is distinct from a
	// coordinate of nought.
	request := boundary.WeatherRequest{Lat: math.NaN(), Lon: math.NaN()}
	decoder := json.NewDecoder(bytes.NewReader(body))
	// A body carrying anything beyond the two the schema names is refused rather
	// than read past: the answer is held under the point alone, so a request
	// carrying more would buy a second rate budget for the same place
	// (SRS012<!-- Request parameters validated against known-good per-source constraints -->).
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return boundary.WeatherRequest{}, errRequestNotDecodable
	}
	// The pre-fill above is the load-bearing half: the constraint is written as
	// the range a coordinate must be inside, which NaN is not, so validate
	// refuses an omitted one on its own. This buys the message naming what the
	// body left out rather than a range it never fell outside.
	if math.IsNaN(request.Lat) || math.IsNaN(request.Lon) {
		return boundary.WeatherRequest{}, errRequestMissingCoordinate
	}
	return request, nil
}

// key names what an answer is about. It is the decoded point written this
// module's one way, so two spellings of one place are one cache entry and one
// rate budget rather than two
// (SRS047<!-- The weather module asks an answering source at most four times an hour for a location -->).
func key(request boundary.WeatherRequest) string {
	return degrees(request.Lat) + "," + degrees(request.Lon)
}

// Package router serves a module's data route: the policy the route runs under,
// and the request flow from the module's own judgement of the request, through
// the upstream pipeline, to the boundary body it answers with (ADR 0026 rev 2).
package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/WisewareOrg/WiseKiosk/backend/internal/boundary"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/secret"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/upstream"
)

// Entry is one upstream-backed module's route registration: the secret the
// source needs, the shaping of its answer, and every policy governing the
// route. It is the seam between the framework here and a module's shaping
// library. What a request carries is not among them: the module reads its own
// generated request body and hands this side the two strings below.
type Entry struct {
	// Config is the six policies this route runs under, all required.
	upstream.Config

	// Source names the cache namespace, the rate bucket, and the module a
	// failure body is contained to.
	Source string
	// Secret is the logical name of the secret this source needs, empty where
	// it needs none.
	Secret string
	// InjectSecret places the resolved secret value in the outbound request.
	// Set exactly where Secret is, and reached only then.
	InjectSecret func(req *http.Request, secretValue string)
	// Shape turns the upstream body into the JSON-encodable frontend payload,
	// without I/O. The body is the cached response, held by every caller that
	// response is served to, so Shape modifies neither it nor anything else
	// two concurrent calls share. Required.
	Shape func(body []byte) (any, error)
}

const (
	// apiPrefix is the path space every route sits in.
	apiPrefix = "/api/"
	// moduleRouteMethod is the method the boundary schema declares every module
	// data route under, and so the method the generated router registers each
	// of them by. It is read here to tell a path the schema declares from one
	// it does not, which is a question about the schema rather than about any
	// module.
	moduleRouteMethod = http.MethodPost
	// allowedMethods is the 405 response's Allow header.
	allowedMethods = moduleRouteMethod
	// contentTypeJSON is the type every body here is written as.
	contentTypeJSON = "application/json"
)

// InvalidParameters is the cause a request naming values its source will not
// carry upstream is rejected under. It is exported because a module judges its
// own request body and answers with this itself, this side reading nothing of a
// request (ADR 0026 rev 2).
const InvalidParameters = "invalid-parameters"

// The causes a rejected request carries, distinguishing what the framework
// refused before any upstream call.
const (
	causeUnknownSource    = "unknown-source"
	causeMethodNotAllowed = "method-not-allowed"
	causeRateLimited      = "rate-limited"
)

// The causes a failed data request carries, one per outcome the pipeline
// distinguishes plus the three the framework classifies itself.
const (
	causeUnreachable        = "unreachable"
	causeTimeout            = "timeout"
	causeUpstreamStatus     = "upstream-status"
	causeOversize           = "oversize"
	causeSecretUnresolvable = "secret-unresolvable"
	causeMalformedPayload   = "malformed-payload"
	// causeUpstreamFailure is what an outcome carrying no cause of its own is
	// rendered as.
	causeUpstreamFailure = "upstream-failure"
)

// CauseShuttingDown is the one failure no upstream call produced: this
// backend stopped serving before the answer was ready (ADR 0026 rev 2).
// Exported: a module route that answers its own 503 outside Route.Serve
// (park_wait_times) writes this same Cause.
const CauseShuttingDown = "shutting-down"

// MessageShuttingDown is CauseShuttingDown's own Message text. Exported
// alongside it for the same reason.
const MessageShuttingDown = "this backend stopped serving before this source could answer"

// MalformedMessage is what a module's own reshaping failure renders as.
// Exported: park_wait_times answers its own malformed-payload outcome
// outside Route.Serve and writes this same Message.
const MalformedMessage = "the source's response could not be read as this module's payload"

// outbound is the client every route's fetch makes its call with. It follows no
// redirect, returning the 3xx as the response, and sets no timeout of its own:
// the deadline is the one the pipeline puts on the call's context.
//
// Its Transport is nil, so each call resolves http.DefaultTransport as it is
// made. A module suite stages its source by swapping that global, so naming a
// Transport here would leave those suites counting calls that no longer reach
// them, with nothing failing to say so.
var outbound = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// NewRoute returns the route serving entry, against the wall clock. An entry
// the framework cannot serve panics here rather than at the request it would
// fail.
func NewRoute(e Entry) *Route {
	return newRoute(e, time.Now)
}

// newRoute builds the route against the clock now, which its cache TTLs and its
// rate limit are measured on.
func newRoute(e Entry, now func() time.Time) *Route {
	e.check()
	return &Route{entry: e, proxy: upstream.New(e.Config, now)}
}

// check panics on an entry missing a part the request flow calls.
func (e Entry) check() {
	switch {
	case e.Source == "":
		panic("router: an entry names no source")
	case e.Shape == nil:
		panic("router: entry " + e.Source + " carries no shaping function")
	case e.Secret != "" && e.InjectSecret == nil:
		panic("router: entry " + e.Source + " names secret " + e.Secret + " and does not place it")
	case e.Secret == "" && e.InjectSecret != nil:
		panic("router: entry " + e.Source + " places a secret and names none")
	}
}

// Route serves one entry's requests.
type Route struct {
	entry Entry
	proxy *upstream.Proxy
}

// Fetch runs the pipeline for one (key, target) pair without writing a
// response: the same cache, rate-limit and outbound-timeout bounds Serve
// answers a whole request with, for a caller that combines more than one
// fetch into a single response — a module whose request names several
// things this source is asked about individually, one Fetch per thing,
// rather than the one-target-per-request shape Serve assumes. The
// returned error means this caller's context ended, exactly as it does
// for Serve, and is the caller's to answer.
func (rt *Route) Fetch(ctx context.Context, key, target string) (upstream.Result, error) {
	return rt.proxy.Do(ctx, rt.entry.Source, key, rt.fetch(target))
}

// Serve answers the request from the pipeline. The module has already judged
// what the request named and reduced it to the two strings here: key names what
// the answer is about, and is what the response cache and the rate budget are
// held against, so two spellings of one thing are one entry and one budget;
// target is the upstream URL the module built for it. The pipeline's error
// means this caller's context ended, and is answered as this module's failure.
func (rt *Route) Serve(w http.ResponseWriter, r *http.Request, key, target string) {
	result, err := rt.Fetch(r.Context(), key, target)
	if err != nil {
		// The pipeline errors only where this caller's context ended: a client
		// that has gone, which is what ends one here, or a server shutting down
		// under one still connected, once a shutdown call exists to do it. Both
		// are written the 503 outcome (ADR 0026 rev 2), where returning
		// unwritten emits an empty 200. Only the second reads it.
		rt.fail(w, http.StatusServiceUnavailable, CauseShuttingDown, MessageShuttingDown)
		return
	}
	rt.respond(w, result)
}

// fetch returns the call the pipeline makes on a cache miss: the entry places
// its secret, and the shared client makes the call to the module's target.
func (rt *Route) fetch(target string) upstream.Fetcher {
	return func(ctx context.Context) (*upstream.Response, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}

		if rt.entry.Secret != "" {
			resolved, err := secret.Resolve(rt.entry.Secret)
			if err != nil {
				return nil, err
			}
			// Unwrapped here and handed straight to the entry's injector
			// (ADR 0023 rev 2).
			rt.entry.InjectSecret(request, resolved.Reveal())
		}

		response, err := outbound.Do(request)
		if err != nil {
			return nil, rt.outboundFailed(err)
		}
		return &upstream.Response{Status: response.StatusCode, Body: response.Body}, nil
	}
}

// outboundError is a failed outbound call, naming the source and carrying the
// cause beneath it.
type outboundError struct {
	source string
	cause  error
}

func (e *outboundError) Error() string {
	return "source " + e.source + ": outbound call failed: " + e.cause.Error()
}

// Unwrap keeps the cause reachable to the errors.Is the pipeline classifies a
// deadline with.
func (e *outboundError) Unwrap() error {
	return e.cause
}

// outboundFailed replaces the *url.Error net/http returns, whose Error text
// renders the request URL, with one carrying the cause and no URL. An entry's
// injector may place its secret in the query string, and this error is held in
// Result.Err for the negative TTL (ADR 0023 rev 2).
func (rt *Route) outboundFailed(err error) error {
	var wrapped *url.Error
	if errors.As(err, &wrapped) {
		err = wrapped.Err
	}
	return &outboundError{source: rt.entry.Source, cause: err}
}

// respond writes the boundary body the result calls for: the shaped payload on
// a success, a rejection where no outbound call was made, and this module's
// upstream failure otherwise.
func (rt *Route) respond(w http.ResponseWriter, result upstream.Result) {
	switch result.Kind {
	case upstream.Success:
		rt.succeed(w, result.Body)
	case upstream.RateLimited:
		reject(w, http.StatusTooManyRequests, causeRateLimited, "this source is being asked for more often than its limit allows")
	default:
		status, cause, message := rt.failure(result)
		rt.fail(w, status, cause, message)
	}
}

// succeed writes the module's payload. It is encoded before the status is, and
// a payload that cannot be encoded is answered as this module's failure.
func (rt *Route) succeed(w http.ResponseWriter, body []byte) {
	payload, err := rt.entry.Shape(body)
	if err != nil {
		rt.fail(w, http.StatusBadGateway, causeMalformedPayload, MalformedMessage)
		return
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		rt.fail(w, http.StatusBadGateway, causeMalformedPayload, MalformedMessage)
		return
	}
	writeEncoded(w, http.StatusOK, encoded)
}

// failure names the status, cause and message for a result that carries no
// body. An unresolvable secret arrives as a failed exchange, so it is read from
// the error before the outcome is; the message carries the secret's name and
// neither its value nor the path it was looked for at. An outcome no case names
// is logged and rendered under the undistinguished cause.
func (rt *Route) failure(result upstream.Result) (int, string, string) {
	var unresolvable *secret.UnresolvableError
	if errors.As(result.Err, &unresolvable) {
		return http.StatusBadGateway, causeSecretUnresolvable,
			fmt.Sprintf("the secret %s this source needs is not available", unresolvable.Name)
	}

	switch result.Kind {
	case upstream.Unreachable:
		return http.StatusBadGateway, causeUnreachable, "the source could not be reached"
	case upstream.Timeout:
		return http.StatusGatewayTimeout, causeTimeout, "the source did not answer in time"
	case upstream.UpstreamStatus:
		return http.StatusBadGateway, causeUpstreamStatus, fmt.Sprintf("the source answered with status %d", result.Status)
	case upstream.Oversize:
		return http.StatusBadGateway, causeOversize, "the source's response was larger than this route accepts"
	}

	log.Printf("router: source %s carries no boundary cause for outcome %s", rt.entry.Source, result.Kind)
	return http.StatusBadGateway, causeUpstreamFailure, "this source could not be served"
}

// fail writes this module's upstream-failure body.
func (rt *Route) fail(w http.ResponseWriter, status int, cause, message string) {
	writeJSON(w, status, boundary.UpstreamFailure{Module: rt.entry.Source, Cause: cause, Message: message})
}

// NewFallback returns the handler for the API paths the schema's own routes did
// not take. It is mounted over the API path space beneath those routes, on the
// multiplexer they registered on.
func NewFallback(mux *http.ServeMux) http.Handler {
	return &fallback{mux: mux}
}

// fallback answers the API paths the schema's own routes did not take: a
// declared route reached by a method it does not serve, and anything else.
type fallback struct {
	mux *http.ServeMux
}

// ServeHTTP tells the two apart by asking the multiplexer the schema's routes
// registered on which pattern would take this path under the method a module
// data route is declared with. A pattern of its own is a route that exists and
// refused the method; this handler's own pattern is a path no route declares. So
// the set of sources is the schema's, rather than a list kept beside it naming
// every module.
func (f *fallback) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	probe := &http.Request{Method: moduleRouteMethod, URL: r.URL, Host: r.Host}
	if _, pattern := f.mux.Handler(probe); pattern != apiPrefix {
		w.Header().Set("Allow", allowedMethods)
		reject(w, http.StatusMethodNotAllowed, causeMethodNotAllowed, "this source answers "+allowedMethods+" only")
		return
	}
	reject(w, http.StatusNotFound, causeUnknownSource, "this backend serves no such source")
}

// maxRequestBytes is how much of a request body a module data route reads before
// refusing the rest. It is one of the two figures the framework holds rather than
// a module (docs/contracts/module-contract.md § Writing the module's
// requirements), and it is a free choice, argued in docs/ARCHITECTURE.md § Backend.
const maxRequestBytes = 1 << 10

// BoundBody caps what a module reads of a request before it decodes one, so a
// body past the cap fails the module's decode and is answered as a rejection.
func BoundBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
}

// Reject writes the rejection a module answers a request it will not carry
// upstream with. The text is the module's, so it says what that source accepts
// (ADR 0026 rev 2).
func Reject(w http.ResponseWriter, cause, message string) {
	reject(w, http.StatusBadRequest, cause, message)
}

// reject writes the client-rejection body (ADR 0026 rev 2).
func reject(w http.ResponseWriter, status int, cause, message string) {
	writeJSON(w, status, boundary.ClientRejection{Cause: cause, Message: message})
}

// writeJSON writes value as the response body under status.
func writeJSON(w http.ResponseWriter, status int, value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeEncoded(w, status, encoded)
}

// writeEncoded writes an already-encoded body under status.
func writeEncoded(w http.ResponseWriter, status int, encoded []byte) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}

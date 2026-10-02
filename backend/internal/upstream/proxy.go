// Package upstream runs one request to a data source through the response
// cache, the source's rate limit and a bounded outbound call, in that order.
package upstream

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/WisewareOrg/WiseKiosk/backend/internal/cache"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/ratelimit"
)

// Kind is what became of a request. The zero Kind is not an outcome: Do yields
// it only beside a non-nil error.
type Kind int

const (
	// Success is a response the upstream returned within every bound.
	Success Kind = iota + 1
	// Unreachable is an exchange that failed at the transport level, with no
	// response or with a body that could not be read.
	Unreachable
	// Timeout is an exchange the outbound deadline ended.
	Timeout
	// UpstreamStatus is a response carrying a status outside 200-299.
	UpstreamStatus
	// Oversize is a response body over the size bound.
	Oversize
	// RateLimited is a request the source's bucket had no token for, so no
	// outbound call was made.
	RateLimited
)

// String names a Kind for diagnosis and test output. These are not the
// boundary's cause values.
func (k Kind) String() string {
	switch k {
	case Success:
		return "success"
	case Unreachable:
		return "unreachable"
	case Timeout:
		return "timeout"
	case UpstreamStatus:
		return "upstream-status"
	case Oversize:
		return "oversize"
	case RateLimited:
		return "rate-limited"
	}
	return "no-outcome"
}

// Result is one request's outcome, in the terms a caller maps to the boundary.
type Result struct {
	// Kind is which outcome this is, and is what a caller distinguishes on.
	Kind Kind
	// Status is the upstream response status, carried by Success and
	// UpstreamStatus and zero otherwise.
	Status int
	// Body is the response body, carried by Success only. Every caller served
	// this result holds the same slice, so none of them may modify it.
	Body []byte
	// Err is what went wrong beneath Unreachable, Timeout and Oversize, for
	// diagnosis rather than for the wire.
	Err error
}

// Response is what one outbound call returned. The Proxy closes Body.
type Response struct {
	// Status is the upstream response status.
	Status int
	// Body is the response body, and is never nil.
	Body io.ReadCloser
}

// Fetcher makes one outbound call, returning either a Response or an error. The
// deadline and the size bound are the Proxy's, so a Fetcher applies neither and
// classifies nothing. It must be safe for concurrent use.
type Fetcher func(ctx context.Context) (*Response, error)

// The cache and rate-limit defaults a route's registration entry starts from,
// defined here so the figure and the reasoning docs/ARCHITECTURE.md § Cache and
// rate-limit defaults records for it are one value rather than two that agree
// today.
const (
	// DefaultSuccessTTL is the default SuccessTTL.
	DefaultSuccessTTL = 10 * time.Minute
	// DefaultNegativeTTL is the default NegativeTTL.
	DefaultNegativeTTL = 60 * time.Second
	// DefaultRequestsPerMinute is the default RequestsPerMinute.
	DefaultRequestsPerMinute = 10
)

// Config is the policy one Proxy runs, and every value is required. The three
// with a default above are still stated per entry, because a route refines them
// against its source. Burst, Timeout and MaxBytes have no default here or in
// the tree: no document names a value for them, so each entry decides its own.
type Config struct {
	// SuccessTTL is how long a Success is served from cache.
	SuccessTTL time.Duration
	// NegativeTTL is how long a failure is served from cache.
	NegativeTTL time.Duration
	// RequestsPerMinute is the rate each source's bucket refills at.
	RequestsPerMinute int
	// Burst is each source's bucket capacity.
	Burst int
	// Timeout bounds one outbound call.
	Timeout time.Duration
	// MaxBytes is the largest response body accepted. A body over it is a
	// failure rather than a truncation.
	MaxBytes int64
}

// errOversize is what a Result carries beneath Oversize.
var errOversize = errors.New("upstream response exceeds the size bound")

// flight is one outbound call in progress, and the result it produced.
type flight struct {
	done   chan struct{}
	result Result
}

// Proxy serves requests for every source under one policy.
type Proxy struct {
	cfg     Config
	results *cache.Cache[Result]
	limiter *ratelimit.Limiter

	mu      sync.Mutex
	flights map[string]*flight
}

// New returns a Proxy running cfg against the clock now, which must be non-nil.
func New(cfg Config, now func() time.Time) *Proxy {
	return &Proxy{
		cfg:     cfg,
		results: cache.New[Result](now),
		limiter: ratelimit.New(cfg.RequestsPerMinute, cfg.Burst, now),
		flights: make(map[string]*flight),
	}
}

// Do answers the request source and key identify, calling fetch only where the
// cache holds nothing and the source has a token to spend. Concurrent callers
// of one uncached (source, key) share a single outbound call and its result.
//
// The error is non-nil only where ctx ends before there is a result, which says
// this caller has gone rather than anything about the source: a caller reads
// Kind only where the error is nil. The exchange itself belongs to the Proxy
// rather than to any caller, so one caller leaving neither cancels it nor
// denies its result to the callers still waiting.
func (p *Proxy) Do(ctx context.Context, source, key string, fetch Fetcher) (Result, error) {
	held := source + "\x00" + key
	if result, ok := p.results.Get(held); ok {
		return result, nil
	}

	f := p.join(held, source, fetch)
	select {
	case <-f.done:
		return f.result, nil
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}

// join returns the flight already running for held, or starts one.
func (p *Proxy) join(held, source string, fetch Fetcher) *flight {
	p.mu.Lock()
	defer p.mu.Unlock()

	if f, running := p.flights[held]; running {
		return f
	}

	f := &flight{done: make(chan struct{})}
	p.flights[held] = f
	go p.fly(held, source, fetch, f)
	return f
}

// fly produces the result for one flight, caches it and hands it to everybody
// waiting. A rate-limited request is not cached: it is a fact about this
// moment's budget rather than about the source, and caching it would outlast
// the refill that ends it.
func (p *Proxy) fly(held, source string, fetch Fetcher, f *flight) {
	result := p.produce(source, fetch)
	if result.Kind != RateLimited {
		ttl := p.cfg.NegativeTTL
		if result.Kind == Success {
			ttl = p.cfg.SuccessTTL
		}
		p.results.Set(held, result, ttl)
	}

	p.mu.Lock()
	delete(p.flights, held)
	p.mu.Unlock()

	f.result = result
	close(f.done)
}

// produce spends one of the source's tokens and makes the outbound call. One
// flight spends one token however many callers it answers, which is what makes
// the bound a bound on outbound calls.
func (p *Proxy) produce(source string, fetch Fetcher) Result {
	if !p.limiter.Allow(source) {
		return Result{Kind: RateLimited}
	}
	return p.fetch(fetch)
}

// fetch makes the outbound call under the deadline and the size bound, and
// classifies what comes back.
func (p *Proxy) fetch(fetch Fetcher) Result {
	ctx, cancel := context.WithTimeout(context.Background(), p.cfg.Timeout)
	defer cancel()

	response, err := fetch(ctx)
	if err != nil {
		return Result{Kind: transportKind(ctx, err), Err: err}
	}
	defer func() { _ = response.Body.Close() }()

	if response.Status < 200 || response.Status > 299 {
		return Result{Kind: UpstreamStatus, Status: response.Status}
	}

	// Reading one byte past the bound is what tells a body at the bound from
	// one over it without holding the whole of an unbounded response.
	body, err := io.ReadAll(io.LimitReader(response.Body, p.cfg.MaxBytes+1))
	if err != nil {
		return Result{Kind: transportKind(ctx, err), Err: err}
	}
	if int64(len(body)) > p.cfg.MaxBytes {
		return Result{Kind: Oversize, Err: errOversize}
	}

	return Result{Kind: Success, Status: response.Status, Body: body}
}

// transportKind tells an exchange the deadline ended from one that failed on
// its own.
func transportKind(ctx context.Context, err error) Kind {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return Timeout
	}
	return Unreachable
}

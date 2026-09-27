package park_wait_times

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tjwise99/WiseKiosk/backend/internal/boundary"
	"github.com/tjwise99/WiseKiosk/backend/internal/router"
	"github.com/tjwise99/WiseKiosk/backend/internal/upstream"
)

// Captured Magic Kingdom live and schedule responses; no case reaches a network.
const (
	capturedLive     = "testdata/mk-live.json"
	capturedSchedule = "testdata/mk-schedule.json"
)

// epcotEntityID is derived through resolvePark, so it cannot drift from knownParks.
var _, epcotEntityID, _ = resolvePark("Epcot")

func liveResponseBytes(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.FromSlash(capturedLive))
	if err != nil {
		t.Fatalf("reading the captured live response: %v", err)
	}
	return body
}

func scheduleResponseBytes(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.FromSlash(capturedSchedule))
	if err != nil {
		t.Fatalf("reading the captured schedule response: %v", err)
	}
	return body
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func serve(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/park-wait-times", strings.NewReader(body))
	ParkWaitTimesRoute{}.PostApiParkWaitTimes(recorder, request)
	return recorder
}

// freshRoute gives the calling test its own route, so no cached answer or rate token leaks
// between tests.
func freshRoute(t *testing.T) {
	t.Helper()
	freshRouteWithConfig(t, Config())
}

// A short real interval stands in: the router's fake clock is package-private.
func freshRouteWithConfig(t *testing.T, cfg upstream.Config) {
	t.Helper()
	held := served
	served = router.NewRoute(router.Entry{
		Config: cfg,
		Source: Source,
		Shape:  func(body []byte) (any, error) { return shapeRides(body, noExclusion) },
	})
	t.Cleanup(func() { served = held })
}

var uuidShape = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// liveURL and scheduleURL restate route.go's upstream-URL shape.
func liveURL(entityID string) string     { return entityBaseURL + entityID + "/live" }
func scheduleURL(entityID string) string { return entityBaseURL + entityID + "/schedule" }

func noExclusion(liveRow) bool { return false }

func intp(n int) *int { return &n }

func sameWaitMinutes(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func showWaitMinutes(p *int) string {
	if p == nil {
		return "null"
	}
	return fmt.Sprintf("%d", *p)
}

func TestTST073_ShapingBuildsTheParksRidesFromTheCapturedResponse(t *testing.T) {
	rides, err := shapeRides(liveResponseBytes(t), noExclusion)
	if err != nil {
		t.Fatalf("shapeRides: unexpected error: %v", err)
	}

	want := []boundary.ParkWaitTimesRide{
		{Name: "The Hall of Presidents", State: boundary.Closed},
		{Name: "Mad Tea Party", State: boundary.Operating, WaitMinutes: intp(10)},
		{Name: "Casey Jr. Splash 'N' Soak Station", State: boundary.Closed},
		{Name: "Seven Dwarfs Mine Train", State: boundary.Operating, WaitMinutes: intp(25)},
		{Name: "Walt Disney's Carousel of Progress", State: boundary.Refurb},
		{Name: "The Barnstormer", State: boundary.Operating, WaitMinutes: intp(5)},
		{Name: "Cinderella Castle", State: boundary.Closed},
	}
	if len(rides) != len(want) {
		t.Fatalf("shaped %d rides, want %d: %+v", len(rides), len(want), rides)
	}
	for index, ride := range rides {
		w := want[index]
		if ride.Name != w.Name {
			t.Errorf("ride %d name = %q, want %q", index, ride.Name, w.Name)
			continue
		}
		if ride.State != w.State {
			t.Errorf("ride %d (%s) state = %q, want %q", index, ride.Name, ride.State, w.State)
		}
		if !sameWaitMinutes(ride.WaitMinutes, w.WaitMinutes) {
			t.Errorf("ride %d (%s) waitMinutes = %s, want %s",
				index, ride.Name, showWaitMinutes(ride.WaitMinutes), showWaitMinutes(w.WaitMinutes))
		}
	}
}

// Shaping must not write to the body: it is the cached response every caller holds.
func TestShapeRidesReadsTheBodyWithoutWritingToIt(t *testing.T) {
	body := liveResponseBytes(t)
	held := string(body)

	if _, err := shapeRides(body, noExclusion); err != nil {
		t.Fatalf("shapeRides: unexpected error: %v", err)
	}
	if string(body) != held {
		t.Error("shapeRides wrote to the body it was given")
	}
}

func TestShapeRidesRefusesABodyThatIsNotJSON(t *testing.T) {
	if _, err := shapeRides([]byte("not json"), noExclusion); err == nil {
		t.Error("shapeRides: no error for a body that is not JSON")
	}
}

// SRS061<!-- The park-wait-times module draws a wait as the time or the not-operating state it is handed -->
func TestShapeWaitReadsEveryStatusTheSourceDeclares(t *testing.T) {
	minutes := 12
	operating := &queueBlock{Standby: &standbyBlock{WaitTime: &minutes}}

	cases := []struct {
		name        string
		status      string
		queue       *queueBlock
		wantState   boundary.ParkWaitTimesState
		wantMinutes *int
		wantErr     bool
	}{
		{"an operating ride with a reported wait", "OPERATING", operating, boundary.Operating, intp(minutes), false},
		{"a down ride", "DOWN", nil, boundary.Down, nil, false},
		{"a closed ride", "CLOSED", nil, boundary.Closed, nil, false},
		{"a ride under refurbishment", "REFURBISHMENT", nil, boundary.Refurb, nil, false},
		{"an operating ride with no queue block at all", "OPERATING", nil, "", nil, true},
		{"an operating ride whose queue carries no standby line", "OPERATING", &queueBlock{}, "", nil, true},
		{"an operating ride whose standby line carries no wait", "OPERATING", &queueBlock{Standby: &standbyBlock{}}, "", nil, true},
		{"a status outside the source's own enum", "BOARDING_GROUP", nil, "", nil, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotState, gotMinutes, err := shapeWait(c.status, c.queue)
			if c.wantErr {
				if err == nil {
					t.Fatalf("shapeWait(%q): no error, want one", c.status)
				}
				return
			}
			if err != nil {
				t.Fatalf("shapeWait(%q): unexpected error: %v", c.status, err)
			}
			if gotState != c.wantState {
				t.Errorf("shapeWait(%q) state = %q, want %q", c.status, gotState, c.wantState)
			}
			if !sameWaitMinutes(gotMinutes, c.wantMinutes) {
				t.Errorf("shapeWait(%q) waitMinutes = %s, want %s",
					c.status, showWaitMinutes(gotMinutes), showWaitMinutes(c.wantMinutes))
			}
		})
	}
}

func TestTST073_AResponseMissingAValueTheRideNeedsIsNotShaped(t *testing.T) {
	cases := map[string]func(read map[string]any){
		"an attraction with no name": func(read map[string]any) {
			rows := read["liveData"].([]any)
			rows[1].(map[string]any)["name"] = nil
		},
		"an attraction with no status": func(read map[string]any) {
			rows := read["liveData"].([]any)
			rows[1].(map[string]any)["status"] = nil
		},
		"an attraction reporting a status this module does not recognise": func(read map[string]any) {
			rows := read["liveData"].([]any)
			rows[1].(map[string]any)["status"] = "BOARDING_GROUP"
		},
	}

	for name, break_ := range cases {
		t.Run(name, func(t *testing.T) {
			var read map[string]any
			if err := json.Unmarshal(liveResponseBytes(t), &read); err != nil {
				t.Fatalf("reading the captured response: %v", err)
			}
			break_(read)

			broken, err := json.Marshal(read)
			if err != nil {
				t.Fatalf("writing the broken response: %v", err)
			}

			rides, err := shapeRides(broken, noExclusion)
			if err == nil {
				t.Fatalf("shapeRides: shaped %+v, want an error", rides)
			}
			if err.Error() == "" {
				t.Error("the error says nothing about what could not be read")
			}
		})
	}
}

// SRS056<!-- The park-wait-times module puts each park's identity, hours, and ride waits across the boundary -->:
// an OPERATING row with no posted wait is a walk-through landmark.
func TestShapeRidesFiltersAnOperatingRowWithNoPostedWait(t *testing.T) {
	var read map[string]any
	if err := json.Unmarshal(liveResponseBytes(t), &read); err != nil {
		t.Fatalf("reading the captured response: %v", err)
	}
	rows := read["liveData"].([]any)
	landmark := rows[2].(map[string]any)
	landmarkName := landmark["name"].(string)
	landmark["status"] = "OPERATING"
	landmark["queue"] = map[string]any{}

	body, err := json.Marshal(read)
	if err != nil {
		t.Fatalf("writing the modified response: %v", err)
	}

	rides, err := shapeRides(body, noExclusion)
	if err != nil {
		t.Fatalf("shapeRides: unexpected error: %v", err)
	}
	for _, ride := range rides {
		if ride.Name == landmarkName {
			t.Errorf("rides carries %q, want it filtered out as not a ride", landmarkName)
		}
	}
	if len(rides) != 6 {
		t.Errorf("shaped %d rides, want 6 (the capture's 7 attractions minus the filtered landmark): %+v", len(rides), rides)
	}
}

func TestTST073_ShapingBuildsTheParksHoursFromTheCapturedResponse(t *testing.T) {
	// Read in the entry's own -04:00 offset.
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

	_, hours, err := shapeSchedule(scheduleResponseBytes(t), now)
	if err != nil {
		t.Fatalf("shapeSchedule: unexpected error: %v", err)
	}
	if hours == nil {
		t.Fatal("shapeSchedule: nil hours, want today's operating hours")
	}
	if hours.Open != "2026-09-13T08:00:00-04:00" {
		t.Errorf("Open = %q, want the day's OPERATING entry, not its early entry or its ticketed event", hours.Open)
	}
	if hours.Close != "2026-09-13T18:00:00-04:00" {
		t.Errorf("Close = %q, want the day's OPERATING entry", hours.Close)
	}
}

// A closed day is nil, not an error (boundary/openapi.yaml's ParkWaitTimesHours).
func TestShapeScheduleReturnsNilForADayTheSourceReportsNoOperatingEntry(t *testing.T) {
	// A day one past the capture's last OPERATING entry.
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	_, hours, err := shapeSchedule(scheduleResponseBytes(t), now)
	if err != nil {
		t.Fatalf("shapeSchedule: unexpected error: %v", err)
	}
	if hours != nil {
		t.Errorf("shapeSchedule = %+v, want nil hours for a day the source reports no OPERATING entry for", hours)
	}
}

func TestShapeScheduleRefusesAMalformedScheduleEntry(t *testing.T) {
	if _, _, err := shapeSchedule([]byte("not json"), time.Now()); err == nil {
		t.Error("shapeSchedule: no error for a body that is not JSON")
	}

	cases := map[string]func(read map[string]any){
		"an operating entry with no opening time": func(read map[string]any) {
			entries := read["schedule"].([]any)
			entries[1].(map[string]any)["openingTime"] = nil
		},
		"an operating entry with no closing time": func(read map[string]any) {
			entries := read["schedule"].([]any)
			entries[1].(map[string]any)["closingTime"] = nil
		},
		"an operating entry whose opening time this module cannot read": func(read map[string]any) {
			entries := read["schedule"].([]any)
			entries[1].(map[string]any)["openingTime"] = "13/09/2026 08:00"
		},
		"an operating entry whose closing time this module cannot read": func(read map[string]any) {
			entries := read["schedule"].([]any)
			entries[1].(map[string]any)["closingTime"] = "13/09/2026 18:00"
		},
	}

	for name, break_ := range cases {
		t.Run(name, func(t *testing.T) {
			var read map[string]any
			if err := json.Unmarshal(scheduleResponseBytes(t), &read); err != nil {
				t.Fatalf("reading the captured response: %v", err)
			}
			break_(read)

			broken, err := json.Marshal(read)
			if err != nil {
				t.Fatalf("writing the broken response: %v", err)
			}

			now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
			_, hours, err := shapeSchedule(broken, now)
			if err == nil {
				t.Fatalf("shapeSchedule: shaped hours %+v, want an error", hours)
			}
		})
	}
}

func TestSameDay(t *testing.T) {
	est := time.FixedZone("", -4*3600)

	cases := []struct {
		name string
		a, b time.Time
		want bool
	}{
		{
			"the same day, different times",
			time.Date(2026, 9, 13, 8, 0, 0, 0, est),
			time.Date(2026, 9, 13, 23, 59, 0, 0, est),
			true,
		},
		{
			"just before and just after midnight",
			time.Date(2026, 9, 13, 23, 59, 59, 0, est),
			time.Date(2026, 9, 14, 0, 0, 1, 0, est),
			false,
		},
		{
			"the same instant, read in two offsets that disagree about the date",
			time.Date(2026, 9, 14, 1, 0, 0, 0, time.UTC),
			time.Date(2026, 9, 13, 21, 0, 0, 0, est),
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sameDay(c.a, c.b); got != c.want {
				t.Errorf("sameDay(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestShapeScheduleReturnsNilForADayWhoseOnlyEntriesAreTicketed(t *testing.T) {
	body := []byte(`{"schedule":[{"type":"TICKETED_EVENT","openingTime":"2026-09-20T09:00:00-04:00","closingTime":"2026-09-20T22:00:00-04:00"}]}`)
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

	_, hours, err := shapeSchedule(body, now)
	if err != nil {
		t.Fatalf("shapeSchedule: unexpected error: %v", err)
	}
	if hours != nil {
		t.Errorf("shapeSchedule = %+v, want nil hours for a day whose only entry is ticketed", hours)
	}
}

func TestShapeScheduleReturnsNilForAnEmptySchedule(t *testing.T) {
	_, hours, err := shapeSchedule([]byte(`{"schedule":[]}`), time.Now())
	if err != nil {
		t.Fatalf("shapeSchedule: unexpected error: %v", err)
	}
	if hours != nil {
		t.Errorf("shapeSchedule = %+v, want nil hours for an empty schedule", hours)
	}
}

// "Today" is judged in the OPERATING entry's own offset
// (SRS056<!-- The park-wait-times module puts each park's identity, hours, and ride waits across the boundary -->).
func TestShapeScheduleSelectsTheDayInTheOperatingEntrysOwnOffset(t *testing.T) {
	cases := []struct {
		name              string
		open, close       string
		now               time.Time
		wantHoursSelected bool
	}{
		{
			name:  "a UTC-day/local-day boundary crossing: now is still 09-13 in UTC but already 09-14 in the entry's own +09:00 offset, and the entry is dated 09-14",
			open:  "2026-09-14T00:30:00+09:00",
			close: "2026-09-14T22:00:00+09:00",
			// 2026-09-13T16:30:00Z is 2026-09-14T01:30:00+09:00.
			now:               time.Date(2026, 9, 13, 16, 30, 0, 0, time.UTC),
			wantHoursSelected: true,
		},
		{
			name:              "the same instant against an entry dated 09-13 (in its own +09:00 offset, that day has already passed) is correctly excluded",
			open:              "2026-09-13T00:30:00+09:00",
			close:             "2026-09-13T22:00:00+09:00",
			now:               time.Date(2026, 9, 13, 16, 30, 0, 0, time.UTC),
			wantHoursSelected: false,
		},
		{
			name:  "a non -04:00 offset (post-DST-fallback US Eastern, -05:00) still selects its own day",
			open:  "2026-11-02T09:00:00-05:00",
			close: "2026-11-02T21:00:00-05:00",
			// 2026-11-02T15:00:00Z is 2026-11-02T10:00:00-05:00, the same day.
			now:               time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC),
			wantHoursSelected: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"schedule":[{"type":"OPERATING","openingTime":%q,"closingTime":%q}]}`, c.open, c.close))
			_, hours, err := shapeSchedule(body, c.now)
			if err != nil {
				t.Fatalf("shapeSchedule: unexpected error: %v", err)
			}
			if !c.wantHoursSelected {
				if hours != nil {
					t.Errorf("shapeSchedule = %+v, want nil — the entry's own day does not match now in its own offset", hours)
				}
				return
			}
			if hours == nil {
				t.Fatal("shapeSchedule = nil, want the entry selected in its own offset")
			}
			if hours.Open != c.open || hours.Close != c.close {
				t.Errorf("Open/Close = %q/%q, want %q/%q", hours.Open, hours.Close, c.open, c.close)
			}
		})
	}
}

// SRS065<!-- The park-wait-times module takes what it shows from one external wait-times source -->
func TestLiveAndScheduleURLsNameTheParksEntity(t *testing.T) {
	entityID := magicKingdomEntityID
	transport := &capturingTransport{body: liveResponseBytes(t)}
	held := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["Magic Kingdom"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	transport.mu.Lock()
	urls := append([]string(nil), transport.urls...)
	transport.mu.Unlock()

	wantLive, wantSchedule := liveURL(entityID), scheduleURL(entityID)
	if !slices.Contains(urls, wantLive) {
		t.Errorf("no request for %q among %v", wantLive, urls)
	}
	if !slices.Contains(urls, wantSchedule) {
		t.Errorf("no request for %q among %v", wantSchedule, urls)
	}
	for _, u := range urls {
		if u != wantLive && u != wantSchedule {
			t.Errorf("upstream request %q, want only the resolved entity's own live/schedule URLs", u)
		}
	}
}

func TestTST079_ThePolicyHoldsAnAnswerNoLongerThanTheFreshnessBound(t *testing.T) {
	policy := Config()

	const bound = 5 * time.Minute
	if policy.SuccessTTL > bound {
		t.Errorf("SuccessTTL = %s, want an answer held no longer than %s", policy.SuccessTTL, bound)
	}
}

func TestTST080_ThePolicyComesToOnceEveryFiveMinutesForAPark(t *testing.T) {
	policy := Config()

	const bound = 5 * time.Minute
	if policy.SuccessTTL > bound {
		t.Errorf("a %s cache interval asks for one park's live data oftener than once every %s, want no oftener than that",
			policy.SuccessTTL, bound)
	}
	if policy.RequestsPerMinute < 1 || policy.Burst < policy.RequestsPerMinute {
		t.Errorf("the bucket refills at %d a minute and holds %d, want a backstop that does not bind first",
			policy.RequestsPerMinute, policy.Burst)
	}
}

func TestTST081_ThePolicyComesToOnceEveryFiveMinutesForAFailingPark(t *testing.T) {
	policy := Config()

	const bound = 5 * time.Minute
	if policy.NegativeTTL < bound {
		t.Errorf("a %s failure interval asks for one park oftener than once every %s, want no oftener than that",
			policy.NegativeTTL, bound)
	}
}

// SuccessTTL and NegativeTTL share one figure, so no ordering between them is asserted.
func TestThePolicyIsComplete(t *testing.T) {
	policy := Config()

	if policy.SuccessTTL <= 0 || policy.NegativeTTL <= 0 || policy.RequestsPerMinute <= 0 ||
		policy.Burst <= 0 || policy.Timeout <= 0 || policy.MaxBytes <= 0 {
		t.Errorf("the policy leaves a value unset: %+v", policy)
	}
}

// Mutex-guarded: route.go's fan-out calls it concurrently.
type successTransport struct {
	mu      sync.Mutex
	calls   map[string]*atomic.Int64
	bodyFor func(url string) []byte
}

func (s *successTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	url := r.URL.String()
	s.mu.Lock()
	counter, ok := s.calls[url]
	if !ok {
		counter = &atomic.Int64{}
		s.calls[url] = counter
	}
	s.mu.Unlock()
	counter.Add(1)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(s.bodyFor(url))),
		Header:     make(http.Header),
	}, nil
}

func TestTST080_IntegrationParkKeysAreCachedIndependently(t *testing.T) {
	live := liveResponseBytes(t)
	schedule := scheduleResponseBytes(t)

	transport := successTransport{
		calls: make(map[string]*atomic.Int64),
		bodyFor: func(url string) []byte {
			if strings.HasSuffix(url, "/live") {
				return live
			}
			return schedule
		},
	}
	held := http.DefaultTransport
	http.DefaultTransport = &transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	ctx := context.Background()

	if _, err := fetchPark(ctx, "Magic Kingdom", noExclusion); err != nil {
		t.Fatalf("fetchPark(Magic Kingdom) #1: unexpected error: %v", err)
	}
	if _, err := fetchPark(ctx, "Magic Kingdom", noExclusion); err != nil {
		t.Fatalf("fetchPark(Magic Kingdom) #2: unexpected error: %v", err)
	}

	magicKingdomCalls := int64(0)
	for url, counter := range transport.calls {
		if strings.Contains(url, magicKingdomEntityID) {
			magicKingdomCalls += counter.Load()
		}
	}
	if magicKingdomCalls != 2 {
		t.Errorf("magic kingdom's own two endpoints cost %d upstream calls across two fetches, want 2 (one live, one schedule, the second fetch served from cache)", magicKingdomCalls)
	}

	if _, err := fetchPark(ctx, "Epcot", noExclusion); err != nil {
		t.Fatalf("fetchPark(Epcot): unexpected error: %v", err)
	}
	epcotCalls := int64(0)
	for url, counter := range transport.calls {
		if strings.Contains(url, epcotEntityID) {
			epcotCalls += counter.Load()
		}
	}
	if epcotCalls != 2 {
		t.Errorf("epcot's own two endpoints cost %d upstream calls, want 2 — a park's cache key must not be shared with another park's", epcotCalls)
	}
}

type failingTransport struct {
	calls atomic.Int64
}

func (f *failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	f.calls.Add(1)
	return &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Header:     make(http.Header),
	}, nil
}

func TestTST081_IntegrationAFailingParkIsRetriedNoOftenerThanTheNegativeInterval(t *testing.T) {
	transport := &failingTransport{}
	held := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = held })

	// A short real interval: the router's fake clock is package-private.
	const shrunkBound = 100 * time.Millisecond
	cfg := Config()
	cfg.NegativeTTL = shrunkBound
	freshRouteWithConfig(t, cfg)

	ctx := context.Background()

	first, err := fetchPark(ctx, "Magic Kingdom", noExclusion)
	if err != nil {
		t.Fatalf("fetchPark #1: unexpected error: %v", err)
	}
	if first.Available {
		t.Fatalf("fetchPark #1: available = true against a failing source, want false")
	}

	second, err := fetchPark(ctx, "Magic Kingdom", noExclusion)
	if err != nil {
		t.Fatalf("fetchPark #2: unexpected error: %v", err)
	}
	if second.Available {
		t.Fatalf("fetchPark #2: available = true against a failing source, want false")
	}

	if calls := transport.calls.Load(); calls != 1 {
		t.Errorf("a park failing twice in a row inside its held interval cost %d upstream calls, want 1", calls)
	}

	time.Sleep(shrunkBound + 150*time.Millisecond)

	third, err := fetchPark(ctx, "Magic Kingdom", noExclusion)
	if err != nil {
		t.Fatalf("fetchPark #3: unexpected error: %v", err)
	}
	if third.Available {
		t.Fatalf("fetchPark #3: available = true against a failing source, want false")
	}
	if calls := transport.calls.Load(); calls != 2 {
		t.Errorf("a park asked again once its held interval elapsed cost %d upstream calls, want 2", calls)
	}
}

// SRS054<!-- The park-wait-times module reports on the parks its configuration names -->
func TestPostApiParkWaitTimesFansOutOverEveryConfiguredPark(t *testing.T) {
	live := liveResponseBytes(t)
	transport := successTransport{
		calls:   make(map[string]*atomic.Int64),
		bodyFor: func(string) []byte { return live },
	}
	held := http.DefaultTransport
	http.DefaultTransport = &transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["Epcot","Magic Kingdom"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	var payload boundary.ParkWaitTimesPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("reading the served payload: %v", err)
	}
	if len(payload.Parks) != 2 {
		t.Fatalf("parks = %d, want 2: %+v", len(payload.Parks), payload.Parks)
	}
	if payload.Parks[0].Name != "Epcot" || payload.Parks[1].Name != "Magic Kingdom" {
		t.Errorf("parks = [%s, %s], want the request's own order [Epcot, Magic Kingdom]",
			payload.Parks[0].Name, payload.Parks[1].Name)
	}
	_, wantFirst, _ := resolvePark("Epcot")
	_, wantSecond, _ := resolvePark("Magic Kingdom")
	transport.mu.Lock()
	_, sawFirst := transport.calls[liveURL(wantFirst)]
	_, sawSecond := transport.calls[liveURL(wantSecond)]
	transport.mu.Unlock()
	if !sawFirst || !sawSecond {
		t.Errorf("no /live call carried the resolved identity for both Epcot (%s) and Magic Kingdom (%s)", wantFirst, wantSecond)
	}
	for _, park := range payload.Parks {
		if !park.Available {
			t.Errorf("park %s: available = false, want true against a serving source", park.Name)
		}
		if park.Rides == nil || len(*park.Rides) == 0 {
			t.Errorf("park %s: carries no rides", park.Name)
		}
	}
}

// TestTheProductRouteReachesTheSourceRatherThanAnEmbeddedFixture checks that the
// route issues an upstream request and serves what it answered
// (SRS065<!-- The park-wait-times module takes what it shows from one external wait-times source -->).
func TestTheProductRouteReachesTheSourceRatherThanAnEmbeddedFixture(t *testing.T) {
	const sentinelRide = "Sentinel Upstream-Only Attraction"
	sentinelLive := []byte(`{"liveData":[{"id":"sentinel-upstream-only","name":"` + sentinelRide +
		`","entityType":"ATTRACTION","status":"OPERATING","queue":{"STANDBY":{"waitTime":42}}}]}`)

	transport := successTransport{
		calls:   make(map[string]*atomic.Int64),
		bodyFor: func(string) []byte { return sentinelLive },
	}
	held := http.DefaultTransport
	http.DefaultTransport = &transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["Magic Kingdom"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	transport.mu.Lock()
	reached := len(transport.calls)
	transport.mu.Unlock()
	if reached == 0 {
		t.Errorf("the route answered without issuing one upstream request, want it to reach the source rather than short-circuit to an embedded fixture: %s", recorder.Body)
	}

	var payload boundary.ParkWaitTimesPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("reading the served payload: %v", err)
	}
	if len(payload.Parks) != 1 {
		t.Fatalf("parks = %d, want 1: %+v", len(payload.Parks), payload.Parks)
	}
	park := payload.Parks[0]
	if park.Rides == nil {
		t.Fatalf("park %s carries no rides despite its own live call serving", park.Name)
	}
	servedRides := make([]string, 0, len(*park.Rides))
	for _, ride := range *park.Rides {
		servedRides = append(servedRides, ride.Name)
	}
	if !slices.Equal(servedRides, []string{sentinelRide}) {
		t.Errorf("rides = %v, want exactly the source's own %q — a roster this transport never wrote was served from somewhere other than the source", servedRides, sentinelRide)
	}
}

// Read off the raw JSON, not the generated struct.
func TestPostApiParkWaitTimesPayloadCarriesNoParkID(t *testing.T) {
	live := liveResponseBytes(t)
	held := http.DefaultTransport
	http.DefaultTransport = liveTransport(live)
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["Magic Kingdom"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	var raw struct {
		Parks []map[string]json.RawMessage `json:"parks"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("reading the served payload as raw JSON: %v", err)
	}
	if len(raw.Parks) != 1 {
		t.Fatalf("parks = %d, want 1: %+v", len(raw.Parks), raw.Parks)
	}
	if _, present := raw.Parks[0]["id"]; present {
		t.Errorf(`park carries an "id" key on the wire, want none: %s`, recorder.Body)
	}
}

func TestAParksOwnFailureDoesNotFailTheWholeRequest(t *testing.T) {
	live := liveResponseBytes(t)
	transport := roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.String(), magicKingdomEntityID) {
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(live)), Header: make(http.Header)}, nil
	})
	held := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["Magic Kingdom","epcot"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s) — one park's failure must not fail the whole request", recorder.Code, http.StatusOK, recorder.Body)
	}

	var payload boundary.ParkWaitTimesPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("reading the served payload: %v", err)
	}
	if len(payload.Parks) != 2 {
		t.Fatalf("parks = %d, want 2: %+v", len(payload.Parks), payload.Parks)
	}

	magicKingdom, epcot := payload.Parks[0], payload.Parks[1]
	if magicKingdom.Available {
		t.Error("Magic Kingdom: available = true against a failing source, want false")
	}
	if magicKingdom.Message == nil || *magicKingdom.Message == "" {
		t.Error("Magic Kingdom: carries no message explaining its own failure")
	}
	if magicKingdom.Rides != nil {
		t.Error("Magic Kingdom: carries rides despite being unavailable")
	}
	if !epcot.Available {
		t.Error("epcot: available = false, want true — the other park's failure must not reach it")
	}
	if epcot.Rides == nil || len(*epcot.Rides) == 0 {
		t.Error("epcot: carries no rides despite its own source serving")
	}
}

// barrierTransport holds every call until n arrive at once, so a sequential fan-out times out
// rather than passes.
type barrierTransport struct {
	mu       sync.Mutex
	arrived  int
	n        int
	release  chan struct{}
	timedOut atomic.Bool
}

func newBarrierTransport(n int, safety time.Duration) *barrierTransport {
	bt := &barrierTransport{n: n, release: make(chan struct{})}
	time.AfterFunc(safety, func() {
		bt.mu.Lock()
		defer bt.mu.Unlock()
		select {
		case <-bt.release:
		default:
			bt.timedOut.Store(true)
			close(bt.release)
		}
	})
	return bt
}

func (bt *barrierTransport) RoundTrip(*http.Request) (*http.Response, error) {
	bt.mu.Lock()
	bt.arrived++
	if bt.arrived >= bt.n {
		select {
		case <-bt.release:
		default:
			close(bt.release)
		}
	}
	bt.mu.Unlock()

	<-bt.release
	return nil, errors.New("barrierTransport: simulated failure, released once every park's call was in flight at once")
}

func TestColdStartFetchesEveryParkConcurrentlyNotSequentially(t *testing.T) {
	parks := []string{"Magic Kingdom", "Epcot", "Hollywood Studios"}
	barrier := newBarrierTransport(len(parks), 2*time.Second)

	held := http.DefaultTransport
	http.DefaultTransport = barrier
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["Magic Kingdom","Epcot","Hollywood Studios"]}`)

	if barrier.timedOut.Load() {
		t.Fatal("fewer than three parks' own calls were ever in flight at once — parks were fetched one at a time, not concurrently")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s) — every park's own failure is that park's, not the whole request's",
			recorder.Code, http.StatusOK, recorder.Body)
	}

	var payload boundary.ParkWaitTimesPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("reading the served payload: %v", err)
	}
	if len(payload.Parks) != len(parks) {
		t.Fatalf("parks = %d, want %d: %+v", len(payload.Parks), len(parks), payload.Parks)
	}
	for _, park := range payload.Parks {
		if park.Available {
			t.Errorf("%s: available = true against a source that never answered, want false", park.Name)
		}
	}
}

func TestAParkWhoseResponseCannotBeShapedIsUnavailable(t *testing.T) {
	transport := roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte("not json"))), Header: make(http.Header)}, nil
	})
	held := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	ctx := context.Background()
	park, err := fetchPark(ctx, "Magic Kingdom", noExclusion)
	if err != nil {
		t.Fatalf("fetchPark: unexpected error: %v", err)
	}
	if park.Available {
		t.Error("available = true for a response that could not be shaped, want false")
	}
	if park.Message == nil || *park.Message != errMalformedPayload.Error() {
		t.Errorf("Message = %v, want %q", park.Message, errMalformedPayload.Error())
	}
	if park.Name != "Magic Kingdom" {
		t.Errorf("Name = %q, want the pretty name %q carried through the failure", park.Name, "Magic Kingdom")
	}
}

func TestAParkWithNoOperatingScheduleEntryStillAnswersWithItsRides(t *testing.T) {
	live := liveResponseBytes(t)
	transport := roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/live") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(live)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
	})
	held := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	ctx := context.Background()
	park, err := fetchPark(ctx, "Magic Kingdom", noExclusion)
	if err != nil {
		t.Fatalf("fetchPark: unexpected error: %v", err)
	}
	if !park.Available {
		t.Fatalf("available = false, want true — the rides answered even though the schedule did not")
	}
	if park.Hours != nil {
		t.Errorf("Hours = %+v, want nil for a schedule call that failed", park.Hours)
	}
	if park.Rides == nil || len(*park.Rides) == 0 {
		t.Error("carries no rides despite its own live call serving")
	}
}

func TestAParkWithANoWaitLandmarkStaysAvailable(t *testing.T) {
	var read map[string]any
	if err := json.Unmarshal(liveResponseBytes(t), &read); err != nil {
		t.Fatalf("reading the captured response: %v", err)
	}
	rows := read["liveData"].([]any)
	landmark := rows[2].(map[string]any)
	landmarkName := landmark["name"].(string)
	landmark["status"] = "OPERATING"
	landmark["queue"] = map[string]any{}
	live, err := json.Marshal(read)
	if err != nil {
		t.Fatalf("writing the modified response: %v", err)
	}

	transport := roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/live") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(live)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
	})
	held := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	ctx := context.Background()
	park, err := fetchPark(ctx, "Magic Kingdom", noExclusion)
	if err != nil {
		t.Fatalf("fetchPark: unexpected error: %v", err)
	}
	if !park.Available {
		t.Fatalf("available = false, want true — a no-wait landmark must not cascade the whole park to unavailable")
	}
	if park.Rides == nil {
		t.Fatal("carries no rides despite its own live call serving")
	}
	for _, ride := range *park.Rides {
		if ride.Name == landmarkName {
			t.Errorf("rides carries %q, want it filtered out as not a ride", landmarkName)
		}
	}
	if len(*park.Rides) != 6 {
		t.Errorf("shaped %d rides, want 6 (the capture's 7 attractions minus the filtered landmark): %+v", len(*park.Rides), *park.Rides)
	}
}

// The OPERATING entry spans today's whole UTC day, so no day boundary is crossed mid-test.
func TestFetchParkDeliversHoursForATodayOperatingEntry(t *testing.T) {
	today := time.Now().UTC()
	open := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	closes := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 0, 0, time.UTC).Format(time.RFC3339)
	schedule := []byte(fmt.Sprintf(`{"schedule":[{"type":"OPERATING","openingTime":%q,"closingTime":%q}]}`, open, closes))
	live := liveResponseBytes(t)

	transport := roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/live") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(live)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(schedule)), Header: make(http.Header)}, nil
	})
	held := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	ctx := context.Background()
	park, err := fetchPark(ctx, "Magic Kingdom", noExclusion)
	if err != nil {
		t.Fatalf("fetchPark: unexpected error: %v", err)
	}
	if !park.Available {
		t.Fatalf("available = false, want true")
	}
	if park.Hours == nil {
		t.Fatal("Hours = nil, want today's OPERATING entry delivered through fetchPark's real clock")
	}
	if park.Hours.Open != open || park.Hours.Close != closes {
		t.Errorf("Hours = %+v, want Open=%q Close=%q", park.Hours, open, closes)
	}
}

func TestFetchParkAnswersWithNilHoursWhenTheScheduleAnswersButCannotBeShaped(t *testing.T) {
	live := liveResponseBytes(t)
	transport := roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/live") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(live)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte("not json"))), Header: make(http.Header)}, nil
	})
	held := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	ctx := context.Background()
	park, err := fetchPark(ctx, "Magic Kingdom", noExclusion)
	if err != nil {
		t.Fatalf("fetchPark: unexpected error: %v", err)
	}
	if !park.Available {
		t.Fatalf("available = false, want true — an unshapeable schedule body must not cascade to the whole park")
	}
	if park.Hours != nil {
		t.Errorf("Hours = %+v, want nil for a schedule body that answered but could not be shaped", park.Hours)
	}
	if park.Rides == nil || len(*park.Rides) == 0 {
		t.Error("carries no rides despite its own live call serving")
	}
}

// The caller's context ending maps to the same 503 router.Route.Serve answers.
func TestPostApiParkWaitTimesAnswersShuttingDownWhenTheCallersContextEnds(t *testing.T) {
	// Never answers. upstream.Proxy.Do runs on context.Background(), so cleanup waits for the flight
	// before restoring http.DefaultTransport.
	block := make(chan struct{})
	released := make(chan struct{})
	transport := roundTrip(func(*http.Request) (*http.Response, error) {
		defer close(released)
		<-block
		return nil, errors.New("unreachable: this test never lets the call finish")
	})
	held := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() {
		close(block)
		select {
		case <-released:
		case <-time.After(5 * time.Second):
			t.Error("the flight this test released never returned")
		}
		http.DefaultTransport = held
	})
	freshRoute(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(
		http.MethodPost, "/api/park-wait-times", strings.NewReader(`{"parks":["Magic Kingdom"]}`),
	).WithContext(ctx)
	recorder := httptest.NewRecorder()

	ParkWaitTimesRoute{}.PostApiParkWaitTimes(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusServiceUnavailable, recorder.Body)
	}
	var failure boundary.UpstreamFailure
	if err := json.Unmarshal(recorder.Body.Bytes(), &failure); err != nil {
		t.Fatalf("reading the failure body %q: %v", recorder.Body, err)
	}
	// The framework's exported constant, not this package's copy.
	if failure.Cause != router.CauseShuttingDown {
		t.Errorf("Cause = %q, want router.CauseShuttingDown (%q)", failure.Cause, router.CauseShuttingDown)
	}
}

func TestDecodeRequestRejectsWhatItMustAndAdmitsTrailingBytes(t *testing.T) {
	rejected := []string{
		`{}`,
		`{"parks":[]}`,
		`{"parks":["epcot"],"region":"orlando"}`,
		`not json`,
		``,
		`[]`,
	}
	for _, body := range rejected {
		t.Run(fmt.Sprintf("rejects %q", body), func(t *testing.T) {
			if _, err := decodeRequest([]byte(body)); err == nil {
				t.Errorf("decodeRequest(%q): no error, want one", body)
			}
		})
	}

	admitted, err := decodeRequest([]byte(`{"parks":["epcot"]}trailing`))
	if err != nil {
		t.Fatalf("decodeRequest: unexpected error: %v", err)
	}
	if len(admitted.Parks) != 1 || admitted.Parks[0] != "epcot" {
		t.Errorf("decodeRequest = %+v, want one park named epcot", admitted)
	}
}

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) {
	return 0, errors.New("errReadCloser: synthetic read failure")
}

func (errReadCloser) Close() error { return nil }

func TestARequestBodyThatCannotBeReadIsRejected(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/park-wait-times", errReadCloser{})

	ParkWaitTimesRoute{}.PostApiParkWaitTimes(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d (%s)", recorder.Code, http.StatusBadRequest, recorder.Body)
	}
}

// Unreachable through the handler, so driven directly.
func TestWriteJSONAnswersInternalServerErrorForAValueThatCannotBeEncoded(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeJSON(recorder, http.StatusOK, make(chan int))

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

const fuzzHangBudget = time.Second

// fn runs on its own goroutine: FailNow may be called only from the test's.
func runWithin(t *testing.T, budget time.Duration, name string, fn func() error) {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	case <-time.After(budget):
		t.Fatalf("%s did not return within %s", name, budget)
	}
}

// check-fuzz (docs/CI.md § Backend fuzz).
func FuzzShapeRides(f *testing.F) {
	seed, err := os.ReadFile(filepath.FromSlash(capturedLive))
	if err != nil {
		f.Fatalf("reading the captured response: %v", err)
	}
	f.Add(seed)

	f.Fuzz(func(t *testing.T, body []byte) {
		runWithin(t, fuzzHangBudget, "FuzzShapeRides", func() error {
			first, firstErr := shapeRides(body, noExclusion)
			second, secondErr := shapeRides(body, noExclusion)
			if (firstErr == nil) != (secondErr == nil) {
				return fmt.Errorf("shapeRides is not deterministic: first = %v, second = %v", firstErr, secondErr)
			}
			if firstErr != nil {
				return nil
			}

			firstJSON, err := json.Marshal(first)
			if err != nil {
				return fmt.Errorf("a non-error result did not marshal: %v", err)
			}
			secondJSON, err := json.Marshal(second)
			if err != nil {
				return fmt.Errorf("a non-error result did not marshal: %v", err)
			}
			if string(firstJSON) != string(secondJSON) {
				return fmt.Errorf("shapeRides is not deterministic: %s vs %s", firstJSON, secondJSON)
			}
			return nil
		})
	})
}

// check-fuzz (docs/CI.md § Backend fuzz).
func FuzzDecodeRequest(f *testing.F) {
	for _, body := range []string{
		`{}`, `{"parks":[]}`, `{"parks":["epcot"],"region":"orlando"}`,
		`not json`, ``, `[]`, `{"parks":["epcot"]}trailing`,
	} {
		f.Add([]byte(body))
	}

	f.Fuzz(func(t *testing.T, body []byte) {
		runWithin(t, fuzzHangBudget, "FuzzDecodeRequest", func() error {
			_, _ = decodeRequest(body)
			return nil
		})
	})
}

// check-fuzz (docs/CI.md § Backend fuzz).
func FuzzResolvePark(f *testing.F) {
	for _, p := range knownParks {
		f.Add(p.name)
		f.Add(p.id)
	}
	for _, name := range []string{"", "disney-world", "Magic-Kingdom", "magic-kingdom "} {
		f.Add(name)
	}

	f.Fuzz(func(t *testing.T, name string) {
		runWithin(t, fuzzHangBudget, "FuzzResolvePark", func() error {
			_, _, _ = resolvePark(name)
			return nil
		})
	})
}

func TestFailureMessageNamesEachOutcomeThePipelineDistinguishes(t *testing.T) {
	for _, kind := range []upstream.Kind{
		upstream.Unreachable,
		upstream.Timeout,
		upstream.UpstreamStatus,
		upstream.Oversize,
		upstream.RateLimited,
		upstream.Success,
	} {
		message := failureMessage(upstream.Result{Kind: kind})
		if message == "" {
			t.Errorf("failureMessage(%s): empty text, want a reason a viewer can read", kind)
		}
	}
}

func TestFailureMessageNamesTheUpstreamsOwnStatus(t *testing.T) {
	message := failureMessage(upstream.Result{Kind: upstream.UpstreamStatus, Status: http.StatusServiceUnavailable})
	if !strings.Contains(message, fmt.Sprintf("%d", http.StatusServiceUnavailable)) {
		t.Errorf("failureMessage = %q, want it to name the upstream's own status %d", message, http.StatusServiceUnavailable)
	}
}

// --- Blacklist exclusion (boundary/openapi.yaml ParkWaitTimesRequest;
// TST083<!-- The blacklist pre-filter excludes named entities from a park's rides -->),
// driven through the handler ---

func rideNames(park boundary.ParkWaitTimesPark) map[string]bool {
	names := make(map[string]bool)
	if park.Rides == nil {
		return names
	}
	for _, ride := range *park.Rides {
		names[ride.Name] = true
	}
	return names
}

func blacklistFixture(t *testing.T, extra ...map[string]any) []byte {
	t.Helper()

	var read map[string]any
	if err := json.Unmarshal(liveResponseBytes(t), &read); err != nil {
		t.Fatalf("reading the captured response: %v", err)
	}
	rows := read["liveData"].([]any)
	for _, row := range extra {
		rows = append(rows, row)
	}
	read["liveData"] = rows

	body, err := json.Marshal(read)
	if err != nil {
		t.Fatalf("writing the modified response: %v", err)
	}
	return body
}

func operatingRowWithWait(id, name string, minutes int) map[string]any {
	return map[string]any{
		"id": id, "name": name, "entityType": "ATTRACTION", "status": "OPERATING",
		"queue": map[string]any{"STANDBY": map[string]any{"waitTime": minutes}},
	}
}

func rowWithStatus(id, name, status string) map[string]any {
	return map[string]any{"id": id, "name": name, "entityType": "ATTRACTION", "status": status}
}

func nilNameRow(id, status string) map[string]any {
	return map[string]any{"id": id, "name": nil, "entityType": "ATTRACTION", "status": status}
}

// liveTransport answers /live with body and fails every /schedule call.
func liveTransport(body []byte) http.RoundTripper {
	return roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/live") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
	})
}

func TestSupportedParksResolveWellFormedDistinctEntityIDs(t *testing.T) {
	if len(knownParks) == 0 {
		t.Fatal("knownParks is empty, want the module's curated park roster")
	}
	seenIDs := make(map[string]bool, len(knownParks))
	seenNames := make(map[string]bool, len(knownParks))
	for _, p := range knownParks {
		if !uuidShape.MatchString(p.id) {
			t.Errorf("%q: id %q is not shaped like a themeparks.wiki entity id", p.name, p.id)
		}
		if seenIDs[p.id] {
			t.Errorf("%q: id %q appears more than once in knownParks", p.name, p.id)
		}
		seenIDs[p.id] = true
		if seenNames[p.name] {
			t.Errorf("%q appears more than once in knownParks", p.name)
		}
		seenNames[p.name] = true
	}
}

func TestDefaultBlacklistIDsAreCuratedAndWellFormed(t *testing.T) {
	if len(defaultBlacklistIDs) == 0 {
		t.Fatal("defaultBlacklistIDs is empty, want the module's curated junk-entity ids")
	}
	seen := make(map[string]bool, len(defaultBlacklistIDs))
	for _, id := range defaultBlacklistIDs {
		if !uuidShape.MatchString(id) {
			t.Errorf("%q is not shaped like a themeparks.wiki entity id", id)
		}
		if seen[id] {
			t.Errorf("%q appears more than once in defaultBlacklistIDs", id)
		}
		seen[id] = true
	}
}

// No OPERATING-with-no-wait case: errNoPostedWait drops that row regardless, so it cannot tell.
func TestBlacklistPreFilterDropsAPrebuiltIDAcrossEveryStatus(t *testing.T) {
	junkID := defaultBlacklistIDs[0]

	cases := []struct {
		name string
		row  map[string]any
	}{
		{"an OPERATING row with a posted wait", operatingRowWithWait(junkID, "Junk (operating)", 15)},
		{"a DOWN row", rowWithStatus(junkID, "Junk (down)", "DOWN")},
		{"a CLOSED row", rowWithStatus(junkID, "Junk (closed)", "CLOSED")},
		{"a REFURBISHMENT row", rowWithStatus(junkID, "Junk (refurb)", "REFURBISHMENT")},
		{"a row carrying no name at all", nilNameRow(junkID, "OPERATING")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			held := http.DefaultTransport
			http.DefaultTransport = liveTransport(blacklistFixture(t, c.row))
			t.Cleanup(func() { http.DefaultTransport = held })
			freshRoute(t)

			recorder := serve(t, `{"parks":["Magic Kingdom"]}`)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d (%s) — a blacklisted entity must not fail the park's shaping", recorder.Code, http.StatusOK, recorder.Body)
			}

			var payload boundary.ParkWaitTimesPayload
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("reading the served payload: %v", err)
			}
			park := payload.Parks[0]
			if !park.Available {
				t.Fatalf("available = false, want true — a blacklisted entity must not cascade the whole park to unavailable")
			}
			if park.Rides == nil {
				t.Fatal("carries no rides")
			}
			for _, ride := range *park.Rides {
				if strings.HasPrefix(ride.Name, "Junk") {
					t.Errorf("rides carries %q, want the blacklisted entity filtered out", ride.Name)
				}
			}
			// The seven attractions, less the four the prebuilt list excludes, and the junk row.
			if len(*park.Rides) != 3 {
				t.Errorf("shaped %d rides, want 3: %+v", len(*park.Rides), *park.Rides)
			}
		})
	}
}

func TestUseDefaultBlacklistFalseIgnoresThePrebuiltList(t *testing.T) {
	junkID := defaultBlacklistIDs[0]
	live := blacklistFixture(t,
		operatingRowWithWait(junkID, "Prebuilt Junk Ride", 3),
		operatingRowWithWait("11111111-1111-1111-1111-111111111111", "User Blacklisted Ride", 4),
	)

	held := http.DefaultTransport
	http.DefaultTransport = liveTransport(live)
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["Magic Kingdom"],"useDefaultBlacklist":false,"blacklist":["User Blacklisted Ride"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusOK, recorder.Body)
	}
	var payload boundary.ParkWaitTimesPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("reading the served payload: %v", err)
	}

	names := rideNames(payload.Parks[0])
	if !names["Prebuilt Junk Ride"] {
		t.Error(`rides omits "Prebuilt Junk Ride", want it admitted — useDefaultBlacklist:false must ignore the prebuilt list`)
	}
	if names["User Blacklisted Ride"] {
		t.Error(`rides carries "User Blacklisted Ride", want it dropped — the user list applies regardless of the toggle`)
	}
}

func TestBlacklistIsTheAdditiveUnionOfThePrebuiltAndUserLists(t *testing.T) {
	junkID := defaultBlacklistIDs[0]
	live := blacklistFixture(t, operatingRowWithWait(junkID, "Prebuilt Junk Ride", 3))

	held := http.DefaultTransport
	http.DefaultTransport = liveTransport(live)
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["Magic Kingdom"],"blacklist":["Mad Tea Party"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusOK, recorder.Body)
	}
	var payload boundary.ParkWaitTimesPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("reading the served payload: %v", err)
	}

	names := rideNames(payload.Parks[0])
	if names["Prebuilt Junk Ride"] {
		t.Error(`rides carries "Prebuilt Junk Ride", want it dropped — the prebuilt list is active by default`)
	}
	if names["Mad Tea Party"] {
		t.Error(`rides carries "Mad Tea Party", want it dropped — the user list adds to the prebuilt list rather than replacing it`)
	}
	if !names["Seven Dwarfs Mine Train"] {
		t.Error(`rides omits "Seven Dwarfs Mine Train", want a ride neither list names left untouched`)
	}
}

// No NFKC case: it needs golang.org/x/text (ADR 0008 rev 6).
func TestUserBlacklistNameMatchIsNormalizedExact(t *testing.T) {
	cases := []struct {
		name        string
		blacklist   string
		rideName    string
		wantDropped bool
	}{
		{"an exact match", "Test Blacklist Ride", "Test Blacklist Ride", true},
		{"case differs", "TEST BLACKLIST RIDE", "Test Blacklist Ride", true},
		{"surrounding whitespace on the list entry", "  Test Blacklist Ride  ", "Test Blacklist Ride", true},
		{
			"a curly apostrophe on the ride against a straight one on the list",
			"Peter Pan's Flight", "Peter Pan’s Flight", true,
		},
		{
			"curly double quotes on the ride against straight ones on the list",
			`the "Wishes" fireworks stage`, "the “Wishes” fireworks stage", true,
		},
		{"a substring of the ride's name is not a whole-name match", "Blacklist Ride", "Test Blacklist Ride", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			live := blacklistFixture(t, operatingRowWithWait("22222222-2222-2222-2222-222222222222", c.rideName, 6))

			held := http.DefaultTransport
			http.DefaultTransport = liveTransport(live)
			t.Cleanup(func() { http.DefaultTransport = held })
			freshRoute(t)

			body, err := json.Marshal(map[string]any{
				"parks":     []string{"Magic Kingdom"},
				"blacklist": []string{c.blacklist},
			})
			if err != nil {
				t.Fatalf("writing the request body: %v", err)
			}
			recorder := serve(t, string(body))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusOK, recorder.Body)
			}
			var payload boundary.ParkWaitTimesPayload
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("reading the served payload: %v", err)
			}

			dropped := !rideNames(payload.Parks[0])[c.rideName]
			if dropped != c.wantDropped {
				t.Errorf("dropped = %v, want %v (blacklist %q against ride %q)", dropped, c.wantDropped, c.blacklist, c.rideName)
			}
		})
	}
}

// --- Park resolution
// (SRS067<!-- The park-wait-times module confines a failure to the part of its own response the failure touches -->) ---
//
// magicKingdomEntityID is derived through resolvePark, so it cannot drift from knownParks.
var _, magicKingdomEntityID, _ = resolvePark("Magic Kingdom")

// capturingTransport is liveTransport that also records every request URL.
type capturingTransport struct {
	mu   sync.Mutex
	body []byte
	urls []string
}

func (c *capturingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.mu.Lock()
	c.urls = append(c.urls, r.URL.String())
	c.mu.Unlock()
	if strings.HasSuffix(r.URL.Path, "/live") {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(c.body)), Header: make(http.Header)}, nil
	}
	return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
}

func (c *capturingTransport) sawURLContaining(substr string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, u := range c.urls {
		if strings.Contains(u, substr) {
			return true
		}
	}
	return false
}

func TestPrettyNameResolvesToItsUUID(t *testing.T) {
	transport := &capturingTransport{body: liveResponseBytes(t)}
	held := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["Magic Kingdom"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s) — a pretty name must resolve, not be rejected", recorder.Code, http.StatusOK, recorder.Body)
	}

	var payload boundary.ParkWaitTimesPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("reading the served payload: %v", err)
	}
	if len(payload.Parks) != 1 {
		t.Fatalf("parks = %d, want 1: %+v", len(payload.Parks), payload.Parks)
	}
	park := payload.Parks[0]
	if !park.Available {
		t.Errorf("available = false, want true — the pretty name should have resolved to a fetchable park: %+v", park)
	}
	if !transport.sawURLContaining(magicKingdomEntityID) {
		t.Errorf("no upstream call carried the pretty name's own entity id %q — want its /live and /schedule calls to use the id, not the pretty name", magicKingdomEntityID)
	}
}

func TestPrettyNameMatchIsNormalized(t *testing.T) {
	variants := []string{
		"magic kingdom",
		"MAGIC KINGDOM",
		"  Magic Kingdom  ",
	}
	for _, name := range variants {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			transport := &capturingTransport{body: liveResponseBytes(t)}
			held := http.DefaultTransport
			http.DefaultTransport = transport
			t.Cleanup(func() { http.DefaultTransport = held })
			freshRoute(t)

			body, err := json.Marshal(boundary.ParkWaitTimesRequest{Parks: []string{name}})
			if err != nil {
				t.Fatalf("encoding the request: %v", err)
			}
			recorder := serve(t, string(body))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d (%s) — %q should still match the pretty name despite its case/whitespace", recorder.Code, http.StatusOK, recorder.Body, name)
			}
			var payload boundary.ParkWaitTimesPayload
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("reading the served payload: %v", err)
			}
			if len(payload.Parks) != 1 || !payload.Parks[0].Available {
				t.Fatalf("parks = %+v, want one available park", payload.Parks)
			}
			if !transport.sawURLContaining(magicKingdomEntityID) {
				t.Errorf("%q did not resolve to the pretty name's own entity id %q", name, magicKingdomEntityID)
			}
		})
	}
}

func TestUnrecognizedConfigStringPassesThroughAsIs(t *testing.T) {
	const raw = "not-a-known-pretty-name"

	transport := &capturingTransport{body: liveResponseBytes(t)}
	held := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["`+raw+`"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s) — an unrecognized park must not fail the request", recorder.Code, http.StatusOK, recorder.Body)
	}
	if !transport.sawURLContaining(raw) {
		t.Errorf("no upstream call carried %q verbatim — want a config string matching no pretty name fetched as the identifier as-is", raw)
	}
}

// SRS067<!-- The park-wait-times module confines a failure to the part of its own response the failure touches -->
func TestUpstream404IsUnsupportedDistinctFromATransientFailure(t *testing.T) {
	const notFoundBody = `{"success":false,"error":{"message":"entity not found","code":404}}`

	transport := roundTrip(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(r.URL.String(), "unsupported-park"):
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(notFoundBody)), Header: make(http.Header)}, nil
		case strings.Contains(r.URL.String(), "transient-park"):
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
		default:
			t.Fatalf("unexpected upstream call: %s", r.URL)
			return nil, nil
		}
	})
	held := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["unsupported-park","transient-park"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s) — neither park's own failure may fail the whole request", recorder.Code, http.StatusOK, recorder.Body)
	}

	var payload boundary.ParkWaitTimesPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("reading the served payload: %v", err)
	}
	if len(payload.Parks) != 2 {
		t.Fatalf("parks = %d, want 2: %+v", len(payload.Parks), payload.Parks)
	}
	unsupported, transientPark := payload.Parks[0], payload.Parks[1]

	if unsupported.Available {
		t.Error("unsupported-park: available = true against a 404, want false")
	}
	if transientPark.Available {
		t.Error("transient-park: available = true against a 503, want false")
	}
	if unsupported.Message == nil || *unsupported.Message == "" {
		t.Fatal("unsupported-park: carries no message")
	}
	if transientPark.Message == nil || *transientPark.Message == "" {
		t.Fatal("transient-park: carries no message")
	}

	const wantUnsupportedMessage = "the source has no such park"
	if *unsupported.Message != wantUnsupportedMessage {
		t.Errorf("unsupported-park's message = %q, want the locked unsupported wording %q", *unsupported.Message, wantUnsupportedMessage)
	}
	wantTransientMessage := fmt.Sprintf("the source answered with status %d", http.StatusServiceUnavailable)
	if *transientPark.Message != wantTransientMessage {
		t.Errorf("transient-park's message = %q, want the existing transient wording %q unchanged", *transientPark.Message, wantTransientMessage)
	}
	if *unsupported.Message == *transientPark.Message {
		t.Error("the 404 (unsupported) and the 503 (transient) park share the same message, want them distinct")
	}
}

// A succeeding schedule with a different name is what pins the !known guard.
func TestAKnownParkShowsItsPrettyNameNotTheUpstreams(t *testing.T) {
	live := liveResponseBytes(t)
	var read map[string]any
	if err := json.Unmarshal(live, &read); err != nil {
		t.Fatalf("reading the captured response: %v", err)
	}
	parkRow := read["liveData"].([]any)[0].(map[string]any)
	if parkRow["entityType"] != "PARK" {
		t.Fatalf("fixture's first row is entityType %v, want PARK — this test reads the capture's own upstream park name", parkRow["entityType"])
	}
	upstreamName, ok := parkRow["name"].(string)
	if !ok || upstreamName == "" {
		t.Fatal("the fixture's PARK row carries no upstream name to contrast against")
	}
	const prettyName = "Magic Kingdom"
	const scheduleName = "Schedule Sentinel Name"
	if upstreamName == prettyName {
		t.Fatalf("fixture's upstream name %q equals the pretty name — the test needs them to differ", upstreamName)
	}

	schedule := []byte(`{"name":"` + scheduleName + `","schedule":[]}`)
	transport := successTransport{
		calls: make(map[string]*atomic.Int64),
		bodyFor: func(url string) []byte {
			if strings.HasSuffix(url, "/live") {
				return live
			}
			return schedule
		},
	}
	held := http.DefaultTransport
	http.DefaultTransport = &transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["Magic Kingdom"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusOK, recorder.Body)
	}
	var payload boundary.ParkWaitTimesPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("reading the served payload: %v", err)
	}
	if len(payload.Parks) != 1 {
		t.Fatalf("parks = %d, want 1: %+v", len(payload.Parks), payload.Parks)
	}
	if payload.Parks[0].Name != prettyName {
		t.Errorf("Name = %q, want the module's own pretty name %q, not the upstream's %q", payload.Parks[0].Name, prettyName, upstreamName)
	}
}

// A real Universal capture with no PARK row; the guard below fails if it gains one.
func TestAKnownParkStaysAvailableWhenTheLiveResponseHasNoParkRow(t *testing.T) {
	live, err := os.ReadFile(filepath.FromSlash("testdata/us-live.json"))
	if err != nil {
		t.Fatalf("reading the Universal live fixture: %v", err)
	}
	var read struct {
		LiveData []struct {
			EntityType string `json:"entityType"`
		} `json:"liveData"`
	}
	if err := json.Unmarshal(live, &read); err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	for _, row := range read.LiveData {
		if row.EntityType == "PARK" {
			t.Fatal("the Universal fixture carries a PARK row — it must not, to stand for the parks whose live response omits one")
		}
	}

	held := http.DefaultTransport
	http.DefaultTransport = liveTransport(live)
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["Universal Studios"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusOK, recorder.Body)
	}
	var payload boundary.ParkWaitTimesPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("reading the served payload: %v", err)
	}
	if len(payload.Parks) != 1 {
		t.Fatalf("parks = %d, want 1: %+v", len(payload.Parks), payload.Parks)
	}
	park := payload.Parks[0]
	if !park.Available {
		t.Errorf("available = false for a recognized park whose live response has no PARK row, want true (message %v)", park.Message)
	}
	if park.Name != "Universal Studios" {
		t.Errorf("Name = %q, want the pretty name %q", park.Name, "Universal Studios")
	}
	if park.Rides == nil || len(*park.Rides) == 0 {
		t.Error("rides empty, want the fixture's attractions carried")
	}
}

func TestAPassThroughParkRecognizedByItsEntityIDShowsThePrettyName(t *testing.T) {
	transport := &capturingTransport{body: liveResponseBytes(t)}
	held := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["`+magicKingdomEntityID+`"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusOK, recorder.Body)
	}
	var payload boundary.ParkWaitTimesPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("reading the served payload: %v", err)
	}
	if len(payload.Parks) != 1 {
		t.Fatalf("parks = %d, want 1: %+v", len(payload.Parks), payload.Parks)
	}
	park := payload.Parks[0]
	if park.Name != "Magic Kingdom" {
		t.Errorf("Name = %q, want the pretty name %q for a park configured by its own entity id", park.Name, "Magic Kingdom")
	}
	if !transport.sawURLContaining(magicKingdomEntityID) {
		t.Errorf("no upstream call carried the entity id %q the park was configured by", magicKingdomEntityID)
	}
}

// The live fixture has no PARK row and the schedule's name is a sentinel, so the name can only
// come from the schedule.
func TestAPassThroughParkIsNamedFromTheSchedule(t *testing.T) {
	const configured = "some-unknown-entity-id"
	const scheduleName = "Sentinel Park Name"
	live, err := os.ReadFile(filepath.FromSlash("testdata/us-live.json"))
	if err != nil {
		t.Fatalf("reading the Universal live fixture: %v", err)
	}
	schedule := []byte(`{"name":"` + scheduleName + `","schedule":[]}`)
	if bytes.Contains(live, []byte(scheduleName)) {
		t.Fatalf("the sentinel schedule name %q appears in the live fixture — it must not, or the test could not tell the schedule from the live response", scheduleName)
	}

	transport := successTransport{
		calls: make(map[string]*atomic.Int64),
		bodyFor: func(url string) []byte {
			if strings.HasSuffix(url, "/live") {
				return live
			}
			return schedule
		},
	}
	held := http.DefaultTransport
	http.DefaultTransport = &transport
	t.Cleanup(func() { http.DefaultTransport = held })
	freshRoute(t)

	recorder := serve(t, `{"parks":["`+configured+`"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusOK, recorder.Body)
	}
	var payload boundary.ParkWaitTimesPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("reading the served payload: %v", err)
	}
	if len(payload.Parks) != 1 {
		t.Fatalf("parks = %d, want 1: %+v", len(payload.Parks), payload.Parks)
	}
	if payload.Parks[0].Name != scheduleName {
		t.Errorf("Name = %q, want the source's own schedule name %q for a pass-through park", payload.Parks[0].Name, scheduleName)
	}
	transport.mu.Lock()
	_, sawConfigured := transport.calls[liveURL(configured)]
	transport.mu.Unlock()
	if !sawConfigured {
		t.Errorf("no /live call carried the configured string %q fetched through unchanged", configured)
	}
}

func TestAPassThroughParkFallsBackToTheConfiguredNameWhenTheSourceSuppliesNone(t *testing.T) {
	const configured = "another-unknown-entity-id"
	live, err := os.ReadFile(filepath.FromSlash("testdata/us-live.json"))
	if err != nil {
		t.Fatalf("reading the Universal live fixture: %v", err)
	}
	cases := map[string]http.RoundTripper{
		"the schedule call fails": roundTrip(func(r *http.Request) (*http.Response, error) {
			if strings.HasSuffix(r.URL.Path, "/live") {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(live)), Header: make(http.Header)}, nil
			}
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
		}),
		"the schedule carries no name": roundTrip(func(r *http.Request) (*http.Response, error) {
			body := live
			if strings.HasSuffix(r.URL.Path, "/schedule") {
				body = []byte(`{"schedule":[]}`)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
		}),
		"the schedule answers with nothing readable": roundTrip(func(r *http.Request) (*http.Response, error) {
			body := live
			if strings.HasSuffix(r.URL.Path, "/schedule") {
				body = []byte("not json")
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
		}),
	}
	for name, transport := range cases {
		t.Run(name, func(t *testing.T) {
			held := http.DefaultTransport
			http.DefaultTransport = transport
			t.Cleanup(func() { http.DefaultTransport = held })
			freshRoute(t)

			recorder := serve(t, `{"parks":["`+configured+`"]}`)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusOK, recorder.Body)
			}
			var payload boundary.ParkWaitTimesPayload
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("reading the served payload: %v", err)
			}
			if len(payload.Parks) != 1 {
				t.Fatalf("parks = %d, want 1: %+v", len(payload.Parks), payload.Parks)
			}
			if payload.Parks[0].Name != configured {
				t.Errorf("Name = %q, want the configured string %q when the source supplies no name", payload.Parks[0].Name, configured)
			}
		})
	}
}

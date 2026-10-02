// Package park_wait_times shapes the module's upstream answers into boundary values:
// the route's policy, the park constraint, the upstream requests, and the response reads.
// Pure — exercised against captured responses (docs/contracts/module-contract.md § The six parts,
// part 4).
package park_wait_times

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/WisewareOrg/WiseKiosk/backend/internal/boundary"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/upstream"
)

// Source names the cache namespace and rate bucket (the boundary path's final segment).
const Source = "park_wait_times"

// Config is the route's policy. SuccessTTL and NegativeTTL read the same figure
// (SRS062<!-- The wait a viewer sees is no more than five minutes behind its source -->,
// SRS064<!-- The park-wait-times module asks a failing source no more often than once every five
// minutes -->). RequestsPerMinute, Burst, Timeout, MaxBytes: see
// docs/contracts/module-contract.md § Writing the module's requirements.
func Config() upstream.Config {
	return upstream.Config{
		SuccessTTL:        5 * time.Minute,
		NegativeTTL:       5 * time.Minute,
		RequestsPerMinute: 10,
		Burst:             20,
		Timeout:           5 * time.Second,
		MaxBytes:          256 << 10,
	}
}

// knownParks is the small offline set this module ships pretty-name
// resolution for: each park's canonical display name beside its upstream entity
// id. A configured park matching one — its pretty name, case- and space-folded,
// or its entity id — is shown under this name and fetched by this id; the id is
// backend-only and never crosses the boundary.
var knownParks = []struct {
	name string
	id   string
}{
	{"Magic Kingdom", "75ea578a-adc8-4116-a54d-dccb60765ef9"},
	{"Epcot", "47f90d2c-e191-4239-a466-5892ef59a88b"},
	{"Hollywood Studios", "288747d1-8b4f-4a64-867e-ea7c9b27bad8"},
	{"Animal Kingdom", "1c84a229-8862-4648-9c71-378ddd2c7693"},
	{"Universal Studios", "eb3f4560-2383-4a36-9152-6b3e5ed6bc57"},
	{"Islands of Adventure", "267615cc-8943-4c2a-ae2c-5da728ca591f"},
}

// resolvePark maps a configured park string to the identity its card shows and
// the identifier its requests fetch. A value matching a known park — its pretty
// name (case- and space-folded) or its upstream entity id — resolves to that
// park's canonical pretty name and entity id, with known true; the shown name
// is this module's own, never the upstream's. Anything else passes through
// unexamined: entityID is the configured string fetched as-is, known is false,
// and name is left empty for the caller to fill from the upstream's own answer
// (SRS068<!-- The park-wait-times module resolves a park it recognizes and
// passes through one it does not -->). No tier rejects the request; a
// passed-through identifier stands or falls on the upstream's own answer.
func resolvePark(configured string) (name, entityID string, known bool) {
	norm := normalizeName(configured)
	trimmed := strings.TrimSpace(configured)
	for _, p := range knownParks {
		if normalizeName(p.name) == norm || strings.EqualFold(trimmed, p.id) {
			return p.name, p.id, true
		}
	}
	return "", configured, false
}

// The upstream requests — two per park, no credential
// (SRS065<!-- The park-wait-times module takes what it shows from one external wait-times
// source -->).
const (
	// entityBaseURL is the source's per-entity endpoint root.
	entityBaseURL = "https://api.themeparks.wiki/v1/entity/"
)

type liveResponse struct {
	LiveData []liveRow `json:"liveData"`
}

// liveRow is one entity the response reports live data for — a ride, a
// show, a restaurant or the park itself. Every value is a pointer so an
// absent one is told from a zero the source meant.
type liveRow struct {
	Id         *string     `json:"id"`
	Name       *string     `json:"name"`
	EntityType *string     `json:"entityType"`
	Status     *string     `json:"status"`
	Queue      *queueBlock `json:"queue"`
}

type queueBlock struct {
	Standby *standbyBlock `json:"STANDBY"`
}

type standbyBlock struct {
	WaitTime *int `json:"waitTime"`
}

// exclusion reports whether one live-data row is dropped before it is
// read as a ride.
type exclusion func(row liveRow) bool

// defaultBlacklistIDs are entity ids the source tags ATTRACTION but the display omits by default
// (SRS066<!-- The park-wait-times module excludes entities its configuration or its own defaults
// name -->). The curation and its rulings: see SRS066.
var defaultBlacklistIDs = []string{
	// Fixed theater/film "shows" typed ATTRACTION.
	"8183f3f2-1b59-4b9c-b634-6a863bdf8d84", // Walt Disney's Carousel of Progress (Magic Kingdom)
	"2ebfb38c-5cb5-4de1-86c0-f7af14188022", // The Hall of Presidents (Magic Kingdom)
	"0f57cecf-5502-4503-8bc3-ba84d3708ace", // Country Bear Musical Jamboree (Magic Kingdom)
	"7c5e1e02-3a44-4151-9005-44066d5ba1da", // Mickey's PhilharMagic (Magic Kingdom)
	"e8f0b426-7645-4ea3-8b41-b94ae7091a41", // Monsters, Inc. Laugh Floor (Magic Kingdom)
	"e76c93df-31af-49a5-8e2f-752c76c937c9", // Enchanted Tales with Belle (Magic Kingdom)
	"6fd1e225-53a0-4a80-a577-4bbc9a471075", // Walt Disney's Enchanted Tiki Room (Magic Kingdom)
	"8c8cd77d-97f6-4309-b285-42aad90e9f15", // Beauty and the Beast Sing-Along (Epcot)
	"35ed719b-f7f0-488f-8346-4fbf8055d373", // Disney and Pixar Short Film Festival (Epcot)
	"ee070d46-6a64-41c0-9f12-69dcfcca10a0", // Reflections of China (Epcot)
	"482169b9-2889-4747-8aef-f9d13a37d940", // Awesome Planet (Epcot)
	"61fb49f8-e62f-4e1c-ae0e-8ab9929037bc", // Canada Far and Wide in Circle-Vision 360 (Epcot)
	"00666fe9-7774-4b53-9fb7-3d333f8aa503", // Impressions de France (Epcot)
	"1f542745-cda1-4786-a536-5fff373e5964", // The American Adventure (Epcot)
	"57acb522-a6fc-4aa4-a80e-21f21f317250", // Turtle Talk With Crush (Epcot)
	"d7669edc-eaa1-4af2-bbb5-6e98df564166", // Walt Disney Presents (Hollywood Studios)
	"9211adc9-b296-4667-8e97-b40cf76108e4", // Vacation Fun - An Original Animated Short with Mickey & Minnie (Hollywood Studios)
	"d5dfc051-f951-40ac-8774-3e9961331ab9", // Bluey's Wild World at Conservation Station (Animal Kingdom)
	"1b15c77b-0311-4171-8e59-7f38e6d60754", // Zootopia: Better Zoogether! (Animal Kingdom)
	// Self-paced exhibits, galleries and walkthroughs.
	"f010bc01-b450-4476-a5f3-a5f2813104b2", // Casey Jr. Splash 'N' Soak Station (Magic Kingdom)
	"30fe3c64-af71-4c66-a54b-aa61fd7af177", // Swiss Family Treehouse (Magic Kingdom)
	"dae68dee-dfba-4128-b594-6aa12add1070", // Journey of Water, Inspired by Moana (Epcot)
	"3e5f26ee-c02d-47fd-891e-5e4479073444", // ImageWorks - The "What If" Labs (Epcot)
	"8f8746cb-c714-4c60-848d-e2dc4e6f586b", // Mexico Folk Art Gallery (Epcot)
	"07dbeaea-85fa-45f2-872f-02f9e7510419", // Gallery of Arts and History (Epcot)
	"0f40274d-420a-425a-9377-29fd6e49484f", // House of the Whispering Willows (Epcot)
	"7969166f-feef-4350-b26e-6a6c745528f4", // SeaBase Aquarium (Epcot)
	"2ecc4fff-2994-476f-9926-24a4af173838", // Bruce's Shark World (Epcot)
	"3d8f8f8f-f984-4d2e-8dea-5a79432bdf05", // Advanced Training Lab (Epcot)
	"3ace01d1-15fc-4fbb-99e4-81a696cb2d05", // Kidcot Fun Stops (Epcot)
	"18c533d6-a395-4ae4-9488-80fce9c497fe", // Palais du Cinéma (Epcot)
	"9053240b-7f7f-44fe-970b-bd7956cd5d4f", // Project Tomorrow: Inventing the Wonders of the Future (Epcot)
	"66ff36de-9cb3-4d9a-b891-1665d19ffb3e", // Stave Church Gallery (Epcot)
	"4f0df9e7-d4c1-45b5-93e2-4a7bc92547b0", // American Heritage Gallery (Epcot)
	"6f1d3b25-42c9-4e99-9dce-6c20d7a5deea", // Bijutsu-kan Gallery (Epcot)
	"6ef1b126-5b0b-46a1-8608-4fcf98ab92c8", // Wilderness Explorers (Animal Kingdom)
	"4d27b0d7-2b0a-4569-90fa-e79f117ec7ef", // Discovery Island Trails (Animal Kingdom)
	"e7976e25-4322-4587-8ded-fb1d9dcbb83c", // Gorilla Falls Exploration Trail (Animal Kingdom)
	"6fbe6d02-4057-43bb-80a3-047b1e8a50ca", // Animal Care at Conservation Station (Animal Kingdom)
	"1a8ea967-229a-42a0-8290-59b036c84e14", // Maharajah Jungle Trek (Animal Kingdom)
	"bc997600-fcc0-4f6f-b908-a1419b26cfd8", // The Oasis Exhibits (Animal Kingdom)
	"f6dba1c6-6f5a-4743-8470-f741ecac555d", // Camp Jurassic™ (Islands of Adventure)
	"22dcd29e-d76a-45cd-b44a-dd6350dc3f8a", // Jurassic Park Discovery Center (Islands of Adventure)
	"391dea99-303d-42a1-aa86-a846d1c1fa1f", // If I Ran The Zoo™ (Islands of Adventure)
	"8babd50d-c570-423b-bd53-d040cad3e087", // Me Ship, The Olive® (Islands of Adventure)
	// Continuous-load transport.
	"e40ac396-cbac-43f4-8752-764ed60ccceb", // Walt Disney World Railroad - Fantasyland (Magic Kingdom)
	"e39b831b-7731-49bb-815b-289b4f49a9fd", // Walt Disney World Railroad - Main Street, U.S.A. (Magic Kingdom)
	"888fb4a4-7adf-47a1-8ba2-c258cc64fd75", // Main Street Vehicles (Magic Kingdom)
	// Named landmarks and self-guided programs.
	"90d79335-c907-4069-a021-d0fe1ec73ae2", // Cinderella Castle (Magic Kingdom)
	"bc1ffa86-9b1a-4ce9-84a5-b479dfa3cb53", // Tree of Life (Animal Kingdom)
	"de737ffc-306b-4f32-8bbb-34e5d370ec8f", // A Pirate's Adventure ~ Treasures of the Seven Seas (Magic Kingdom)
	// Halloween Horror Nights (Universal Studios): a seasonal overlay.
	"48f99577-cfc3-40b4-8161-844300c823d4", // Ozzy Osbourne: Prince of Darkness (Universal Studios)
	"41c4491c-f3d9-41e5-963c-21a10a255b39", // Cybergoria (Universal Studios)
	"6b57dae1-a6e9-42ba-b59d-8ef4ee8ae11e", // MADLANDS: Caged Cannibals (Universal Studios)
	"12a4b7d4-3a27-48dd-b355-99fe9f8aab37", // Stranger Things 5 (Universal Studios)
	"be4bc36e-e2d8-474b-9ea7-74c0e3e6c82d", // Evil Dead Burn (Universal Studios)
	"99accf82-ca8d-425d-abc8-33e69aee75e9", // H.R. Bloodengutz Presents: A Halloween Fright-Tacular! (Universal Studios)
	"98ac6a78-0320-4949-8824-934d0e73e4e2", // Hellraiser (Universal Studios)
	"53ac1ddd-51b1-4b5b-a72b-3a7b3e25397a", // INVASION: Alien Abduction (Universal Studios)
	"709e0baf-dd74-4b4e-9d19-7092336c0846", // Jack & Oddfellow: Chaos & Control (Universal Studios)
	"24712410-a3a8-4ee0-b2f7-df889424ae76", // Sinners (Universal Studios)
}

// normalizeName folds a name — a ride's, or a config-named park's — to its
// comparison key (ADR 0008 rev 6; #309 build spec decision 3, the same
// convention reused for the pretty-name map).
func normalizeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	quoteFold := strings.NewReplacer(
		"‘", "'", "’", "'",
		"“", `"`, "”", `"`,
	)
	return quoteFold.Replace(name)
}

// newExclusion builds the request's exclusion: the default id list —
// unless the request turns it off (UseDefaultBlacklist, default on where
// omitted) — unioned with the request's own blacklist names (Blacklist,
// empty where omitted).
func newExclusion(request boundary.ParkWaitTimesRequest) exclusion {
	ids := make(map[string]bool)
	if request.UseDefaultBlacklist == nil || *request.UseDefaultBlacklist {
		for _, id := range defaultBlacklistIDs {
			ids[id] = true
		}
	}
	var userBlacklist []string
	if request.Blacklist != nil {
		userBlacklist = *request.Blacklist
	}
	names := make(map[string]bool, len(userBlacklist))
	for _, name := range userBlacklist {
		names[normalizeName(name)] = true
	}
	return func(row liveRow) bool {
		if row.Id != nil && ids[*row.Id] {
			return true
		}
		return row.Name != nil && names[normalizeName(*row.Name)]
	}
}

// shapeRides reads the source's live-data response into the park's ride
// list, attractions only and in the order the source gives them
// (SRS056<!-- The park-wait-times module puts each park's identity, hours,
// and ride waits across the boundary -->), dropping any row excluded
// carries away first. A response missing a value a kept row needs is an
// error rather than a payload carrying a zero nobody reported, except an
// attraction reporting OPERATING with no posted wait, which is left out of
// the list rather than failing the whole park's shaping.
func shapeRides(body []byte, excluded exclusion) ([]boundary.ParkWaitTimesRide, error) {
	var read liveResponse
	if err := json.Unmarshal(body, &read); err != nil {
		return nil, fmt.Errorf("reading the source's response: %w", err)
	}

	rides := make([]boundary.ParkWaitTimesRide, 0, len(read.LiveData))
	for _, row := range read.LiveData {
		if excluded(row) {
			continue
		}
		// A show, a restaurant or the park's own row is not carried — attractions only
		// (SRS056<!-- The park-wait-times module puts each park's identity, hours, and ride waits
		// across the boundary -->).
		if row.EntityType == nil || *row.EntityType != "ATTRACTION" {
			continue
		}
		if row.Name == nil {
			return nil, errors.New("an attraction's response carries no name")
		}
		if row.Status == nil {
			return nil, fmt.Errorf("%q's response carries no status", *row.Name)
		}

		state, waitMinutes, err := shapeWait(*row.Status, row.Queue)
		if errors.Is(err, errNoPostedWait) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%q %w", *row.Name, err)
		}
		rides = append(rides, boundary.ParkWaitTimesRide{Name: *row.Name, State: state, WaitMinutes: waitMinutes})
	}
	return rides, nil
}

// errNoPostedWait is shapeWait's sentinel for an OPERATING row with nothing
// to draw a wait from.
var errNoPostedWait = errors.New("is operating but the response reports no wait")

// shapeWait reads one ride's status into its boundary state and, when
// operating, its posted standby wait in minutes
// (SRS061<!-- The park-wait-times module draws a wait as the time or the
// not-operating state it is handed -->; boundary/openapi.yaml's
// ParkWaitTimesState and waitMinutes). waitMinutes is nil in every state but
// Operating.
func shapeWait(status string, queue *queueBlock) (boundary.ParkWaitTimesState, *int, error) {
	switch status {
	case "OPERATING":
		if queue == nil || queue.Standby == nil || queue.Standby.WaitTime == nil {
			return "", nil, errNoPostedWait
		}
		return boundary.Operating, queue.Standby.WaitTime, nil
	case "DOWN":
		return boundary.Down, nil, nil
	case "CLOSED":
		return boundary.Closed, nil, nil
	case "REFURBISHMENT":
		return boundary.Refurb, nil, nil
	default:
		return "", nil, fmt.Errorf("reports a status this module does not recognise: %q", status)
	}
}

type scheduleResponse struct {
	Name     *string         `json:"name"`
	Schedule []scheduleEntry `json:"schedule"`
}

type scheduleEntry struct {
	Type        *string `json:"type"`
	OpeningTime *string `json:"openingTime"`
	ClosingTime *string `json:"closingTime"`
}

// shapeSchedule reads one schedule response, parsed once, into the two
// things it carries: name, the park's own display name from the response's
// top-level name — the one name every park carries, the Universal parks
// included, whose live response has no park-identity row to read; used only
// to name a park this module does not recognize, a known park shows its
// configured pretty name instead (resolvePark). Empty where the response
// carries none. hours is today's operating hours, today read against `now`
// to stay pure
// (SRS056<!-- The park-wait-times module puts each park's identity, hours,
// and ride waits across the boundary -->; boundary/openapi.yaml's
// ParkWaitTimesHours) — nil where the source reports no OPERATING entry, or
// where err reports that entry's own times could not be read; name is
// returned regardless, the two being independent parts of the one response.
func shapeSchedule(body []byte, now time.Time) (name string, hours *boundary.ParkWaitTimesHours, err error) {
	var read scheduleResponse
	if err := json.Unmarshal(body, &read); err != nil {
		return "", nil, fmt.Errorf("reading the source's response: %w", err)
	}
	if read.Name != nil {
		name = *read.Name
	}

	for _, entry := range read.Schedule {
		if entry.Type == nil || *entry.Type != "OPERATING" {
			continue
		}
		if entry.OpeningTime == nil || entry.ClosingTime == nil {
			return name, nil, errors.New("an operating schedule entry carries no opening or closing time")
		}
		opens, err := time.Parse(time.RFC3339, *entry.OpeningTime)
		if err != nil {
			return name, nil, fmt.Errorf("the schedule's opening time is not one this module can read: %w", err)
		}
		closes, err := time.Parse(time.RFC3339, *entry.ClosingTime)
		if err != nil {
			return name, nil, fmt.Errorf("the schedule's closing time is not one this module can read: %w", err)
		}
		if !sameDay(now.In(opens.Location()), opens) {
			continue
		}
		return name, &boundary.ParkWaitTimesHours{
			Open:  opens.Format(time.RFC3339),
			Close: closes.Format(time.RFC3339),
		}, nil
	}
	return name, nil, nil
}

// sameDay reports whether a and b fall on the same calendar day, each read
// in its own location.
func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

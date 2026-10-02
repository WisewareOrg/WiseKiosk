// Package registry holds the boundary schema's module data routes: one embedded
// field per upstream-backed module (ADR 0021 rev 4, ADR 0008 rev 6).
package registry

import (
	"github.com/WisewareOrg/WiseKiosk/backend/internal/modules/park_wait_times"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/modules/weather"
)

// Modules serves every module data route the boundary schema declares. Each
// field is one module's route, carrying that module's registration entry in the
// module's own package; a module is registered by adding a field and by nothing
// else, which is the one crossover the module contract admits from shared code
// into a module's.
//
// A schema route no field serves is a compile error where this value meets the
// generated server interface.
type Modules struct {
	weather.WeatherRoute
	park_wait_times.ParkWaitTimesRoute
}

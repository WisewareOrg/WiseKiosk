// Package weather is the weather module's shaping library: the policy its route
// runs under, the constraint its location must satisfy, the upstream request it
// builds and the boundary payload it reads the answer into. Every function here
// is pure — no I/O, no clock, no secret — so the whole of it is exercisable
// against a captured response (the module contract, part 4).
package weather

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/WisewareOrg/WiseKiosk/backend/internal/boundary"
	"github.com/WisewareOrg/WiseKiosk/backend/internal/upstream"
)

// Source names this module's cache namespace and its rate bucket. It is the same
// word the boundary schema's path ends in.
const Source = "weather"

// The bounds a point on the earth's surface lies within
// (SRS043<!-- The weather module declares the known-good constraint the location it is asked about must satisfy -->).
const (
	maxLatitude  = 90
	maxLongitude = 180
)

// Config is the policy this route runs under, carried here so the registration
// entry assembles it from the module rather than restating it.
//
// SuccessTTL is how long an answer is served from cache however often the
// display asks, which is where
// SRS047<!-- The weather module asks an answering source at most four times an hour for a location -->'s
// figure is met; NegativeTTL is the failing path's own bound
// (SRS049<!-- The weather module asks a failing source no more often than once every five minutes -->).
// RequestsPerMinute, Burst, Timeout and MaxBytes are this module's free choices,
// recorded in docs/contracts/module-contract.md § Writing the module's
// requirements.
func Config() upstream.Config {
	return upstream.Config{
		SuccessTTL:        15 * time.Minute,
		NegativeTTL:       5 * time.Minute,
		RequestsPerMinute: 4,
		Burst:             8,
		Timeout:           5 * time.Second,
		MaxBytes:          16 << 10,
	}
}

// validate judges the point a request names against the constraint this module
// declares: a latitude and a longitude, each within its range
// (SRS043<!-- The weather module declares the known-good constraint the location it is asked about must satisfy -->).
// How the two are spelled is the boundary schema's, refused by the generated
// body's own decoding before anything here is reached.
// The returned error's text is what the rejection renders, so it says what is
// accepted and never echoes what was sent.
func validate(request boundary.WeatherRequest) error {
	if err := inRange("latitude", request.Lat, maxLatitude); err != nil {
		return err
	}
	return inRange("longitude", request.Lon, maxLongitude)
}

// inRange refuses a coordinate outside bound degrees either side of zero.
func inRange(name string, given, bound float64) error {
	// Written as the range it must be inside rather than the two it must not,
	// which is what refuses a value that compares false against both.
	if !(given >= -bound && given <= bound) {
		return fmt.Errorf("the %s must be between -%g and %g degrees", name, bound, bound)
	}
	return nil
}

// degrees writes a coordinate the one way this module writes one, so the upstream
// request and the key an answer is held under agree on what a point is called.
func degrees(given float64) string {
	return strconv.FormatFloat(given, 'f', -1, 64)
}

// The upstream request. One call carries the present conditions and both forward
// ranges, which is what lets this module read its source through the one URL and
// one body the framework gives it
// (SRS041<!-- The weather module takes what it shows from one external weather source -->).
const (
	// forecastURL is the source's forecast endpoint. It needs no credential.
	forecastURL = "https://api.open-meteo.com/v1/forecast"
	// currentFields, hourlyFields and dailyFields are the measurements each part
	// of the payload is read from, in the source's own vocabulary.
	currentFields = "temperature_2m,apparent_temperature,relative_humidity_2m,wind_speed_10m,weather_code,is_day"
	hourlyFields  = "temperature_2m,weather_code,precipitation_probability,is_day"
	dailyFields   = "weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max"
)

// How far ahead the two forward ranges run — the figures are
// SRS044<!-- The weather module puts the present conditions and the near-term outlook across the boundary -->'s,
// obliged again over what is drawn
// (SRS045<!-- The weather module shows the present weather and the outlook apart from each other -->).
const (
	// hoursShown is how many hours the payload carries.
	hoursShown = 12
	// hoursRequested is one more, because the source's hourly range starts at the
	// hour in progress — which is behind rather than ahead, and is dropped.
	hoursRequested = hoursShown + 1
	// daysShown is how many days the payload carries, all of them after today:
	// what today is doing is the present section's, and a day already begun is
	// not one of the days next to come.
	daysShown = 5
	// daysRequested is one more, because the source's daily range starts at today
	// — which is the day the payload drops.
	daysRequested = daysShown + 1
)

// buildURL builds the upstream request for the point a request named. It asks
// for imperial units, which is what the boundary schema declares the payload in,
// and for the source's own local time so a timestamp can be given the offset it
// belongs to.
// It is total: the constraint is judged before this is reached, and every other
// value here is this module's own.
func buildURL(request boundary.WeatherRequest) string {
	values := url.Values{
		"latitude":           {degrees(request.Lat)},
		"longitude":          {degrees(request.Lon)},
		"current":            {currentFields},
		"hourly":             {hourlyFields},
		"daily":              {dailyFields},
		"temperature_unit":   {"fahrenheit"},
		"wind_speed_unit":    {"mph"},
		"precipitation_unit": {"inch"},
		"forecast_hours":     {strconv.Itoa(hoursRequested)},
		"forecast_days":      {strconv.Itoa(daysRequested)},
		"timezone":           {"auto"},
	}
	return forecastURL + "?" + values.Encode()
}

// The units the request above asks for, as the source spells them back. They are
// asserted rather than assumed: a source that answered in another unit would
// otherwise be rendered as a temperature the schema says is Fahrenheit.
const (
	unitFahrenheit   = "°F"
	unitMilesPerHour = "mp/h"
	unitPercent      = "%"
)

// The layouts the source writes a time in. Neither carries a zone, so both are
// read in the offset the response reports for the location.
const (
	hourLayout = "2006-01-02T15:04"
	dayLayout  = "2006-01-02"
)

// forecast is the source's response, in the source's own vocabulary and read no
// further than this package. Every value is a pointer or a slice so an absent
// one is told from a zero the source meant, which is the difference between a
// payload this module cannot build and one carrying a temperature of nought.
type forecast struct {
	UTCOffsetSeconds *int          `json:"utc_offset_seconds"`
	CurrentUnits     *unitsBlock   `json:"current_units"`
	Current          *currentBlock `json:"current"`
	HourlyUnits      *unitsBlock   `json:"hourly_units"`
	Hourly           *seriesBlock  `json:"hourly"`
	DailyUnits       *unitsBlock   `json:"daily_units"`
	Daily            *seriesBlock  `json:"daily"`
}

// unitsBlock is every unit any of the three parts reports. Each part reports the
// subset its own measurements have, so a field absent from one is nil there.
type unitsBlock struct {
	Temperature                 *string `json:"temperature_2m"`
	ApparentTemperature         *string `json:"apparent_temperature"`
	RelativeHumidity            *string `json:"relative_humidity_2m"`
	WindSpeed                   *string `json:"wind_speed_10m"`
	PrecipitationProbability    *string `json:"precipitation_probability"`
	TemperatureMax              *string `json:"temperature_2m_max"`
	TemperatureMin              *string `json:"temperature_2m_min"`
	PrecipitationProbabilityMax *string `json:"precipitation_probability_max"`
}

// currentBlock is the present conditions, one value each.
type currentBlock struct {
	Temperature         *float64 `json:"temperature_2m"`
	ApparentTemperature *float64 `json:"apparent_temperature"`
	RelativeHumidity    *float64 `json:"relative_humidity_2m"`
	WindSpeed           *float64 `json:"wind_speed_10m"`
	WeatherCode         *int     `json:"weather_code"`
	IsDay               *int     `json:"is_day"`
}

// seriesBlock is a forward range, which the source returns as one array per
// measurement rather than one object per step. Both ranges are read from this
// shape; the fields a given range does not carry are nil.
type seriesBlock struct {
	Time                        []string   `json:"time"`
	Temperature                 []*float64 `json:"temperature_2m"`
	WeatherCode                 []*int     `json:"weather_code"`
	PrecipitationProbability    []*float64 `json:"precipitation_probability"`
	IsDay                       []*int     `json:"is_day"`
	TemperatureMax              []*float64 `json:"temperature_2m_max"`
	TemperatureMin              []*float64 `json:"temperature_2m_min"`
	PrecipitationProbabilityMax []*float64 `json:"precipitation_probability_max"`
}

// Shape reads the source's response into the payload the boundary schema
// declares: the present conditions in each of the five respects, and both
// forward ranges
// (SRS044<!-- The weather module puts the present conditions and the near-term outlook across the boundary -->).
// A response missing a value the payload requires is an error rather than a
// payload carrying a zero nobody reported. The body is the cached response every
// caller it is served to holds, and nothing here writes to it.
func Shape(body []byte) (boundary.WeatherPayload, error) {
	var read forecast
	if err := json.Unmarshal(body, &read); err != nil {
		return boundary.WeatherPayload{}, fmt.Errorf("reading the source's response: %w", err)
	}

	if read.UTCOffsetSeconds == nil {
		return boundary.WeatherPayload{}, errors.New(
			"the response reports no UTC offset for the location",
		)
	}
	// Nameless, because the abbreviation is the source's presentation and what a
	// timestamp needs is the offset.
	local := time.FixedZone("", *read.UTCOffsetSeconds)

	current, err := shapeCurrent(read.Current, read.CurrentUnits)
	if err != nil {
		return boundary.WeatherPayload{}, err
	}
	hourly, err := shapeHourly(read.Hourly, read.HourlyUnits, local)
	if err != nil {
		return boundary.WeatherPayload{}, err
	}
	daily, err := shapeDaily(read.Daily, read.DailyUnits, local)
	if err != nil {
		return boundary.WeatherPayload{}, err
	}

	return boundary.WeatherPayload{Current: current, Hourly: hourly, Daily: daily}, nil
}

// shapeCurrent reads the present conditions.
func shapeCurrent(block *currentBlock, units *unitsBlock) (boundary.WeatherCurrent, error) {
	if block == nil || units == nil {
		return boundary.WeatherCurrent{}, errors.New("the response carries no present conditions")
	}

	for _, checked := range []struct {
		measurement string
		reported    *string
		want        string
	}{
		{"temperature", units.Temperature, unitFahrenheit},
		{"apparent temperature", units.ApparentTemperature, unitFahrenheit},
		{"humidity", units.RelativeHumidity, unitPercent},
		{"wind speed", units.WindSpeed, unitMilesPerHour},
	} {
		if err := sameUnit(checked.measurement, checked.reported, checked.want); err != nil {
			return boundary.WeatherCurrent{}, err
		}
	}

	temperature, err := value("the present temperature", block.Temperature)
	if err != nil {
		return boundary.WeatherCurrent{}, err
	}
	apparent, err := value("the present apparent temperature", block.ApparentTemperature)
	if err != nil {
		return boundary.WeatherCurrent{}, err
	}
	humidity, err := value("the present humidity", block.RelativeHumidity)
	if err != nil {
		return boundary.WeatherCurrent{}, err
	}
	wind, err := value("the present wind speed", block.WindSpeed)
	if err != nil {
		return boundary.WeatherCurrent{}, err
	}
	code, err := value("the present weather code", block.WeatherCode)
	if err != nil {
		return boundary.WeatherCurrent{}, err
	}
	daylight, err := value("whether it is daylight", block.IsDay)
	if err != nil {
		return boundary.WeatherCurrent{}, err
	}

	return boundary.WeatherCurrent{
		Temp:         temperature,
		ApparentTemp: apparent,
		Humidity:     humidity,
		WindSpeed:    wind,
		WeatherCode:  code,
		IsDay:        daylight == 1,
	}, nil
}

// shapeHourly reads the hours next to come, dropping the hour in progress that
// the source's range opens with.
func shapeHourly(
	block *seriesBlock,
	units *unitsBlock,
	local *time.Location,
) ([]boundary.WeatherHour, error) {
	if block == nil || units == nil {
		return nil, errors.New("the response carries no hourly range")
	}
	if err := sameUnit("hourly temperature", units.Temperature, unitFahrenheit); err != nil {
		return nil, err
	}
	if err := sameUnit(
		"hourly precipitation probability",
		units.PrecipitationProbability,
		unitPercent,
	); err != nil {
		return nil, err
	}

	steps := len(block.Time)
	if steps != len(block.Temperature) || steps != len(block.WeatherCode) ||
		steps != len(block.PrecipitationProbability) || steps != len(block.IsDay) {
		return nil, errors.New("the hourly range's measurements do not run to the same length")
	}
	if steps <= 1 {
		return nil, errors.New("the hourly range carries no hour that has not begun")
	}

	hours := make([]boundary.WeatherHour, 0, hoursShown)
	for step := 1; step < steps && len(hours) < hoursShown; step++ {
		at, err := stamp("an hourly", hourLayout, block.Time[step], local)
		if err != nil {
			return nil, err
		}
		temperature, err := value("an hourly temperature", block.Temperature[step])
		if err != nil {
			return nil, err
		}
		code, err := value("an hourly weather code", block.WeatherCode[step])
		if err != nil {
			return nil, err
		}
		chance, err := value(
			"an hourly precipitation probability",
			block.PrecipitationProbability[step],
		)
		if err != nil {
			return nil, err
		}
		// Read per hour rather than from the present reading: the hours to come
		// cross the location's own sunrise or sunset
		// (SRS050<!-- The weather module draws day and night apart -->).
		daylight, err := value("whether an hour is in daylight", block.IsDay[step])
		if err != nil {
			return nil, err
		}

		hours = append(hours, boundary.WeatherHour{
			Time:              at,
			Temp:              temperature,
			WeatherCode:       code,
			PrecipProbability: chance,
			IsDay:             daylight == 1,
		})
	}
	return hours, nil
}

// shapeDaily reads the days next to come, dropping today that the source's range
// opens with: what today is doing is the present section's.
func shapeDaily(
	block *seriesBlock,
	units *unitsBlock,
	local *time.Location,
) ([]boundary.WeatherDay, error) {
	if block == nil || units == nil {
		return nil, errors.New("the response carries no daily range")
	}
	if err := sameUnit(
		"daily maximum temperature",
		units.TemperatureMax,
		unitFahrenheit,
	); err != nil {
		return nil, err
	}
	if err := sameUnit(
		"daily minimum temperature",
		units.TemperatureMin,
		unitFahrenheit,
	); err != nil {
		return nil, err
	}
	if err := sameUnit(
		"daily precipitation probability",
		units.PrecipitationProbabilityMax,
		unitPercent,
	); err != nil {
		return nil, err
	}

	steps := len(block.Time)
	if steps != len(block.WeatherCode) || steps != len(block.TemperatureMax) ||
		steps != len(block.TemperatureMin) || steps != len(block.PrecipitationProbabilityMax) {
		return nil, errors.New("the daily range's measurements do not run to the same length")
	}
	if steps <= 1 {
		return nil, errors.New("the daily range carries no day after today")
	}

	days := make([]boundary.WeatherDay, 0, daysShown)
	for step := 1; step < steps && len(days) < daysShown; step++ {
		at, err := stamp("a daily", dayLayout, block.Time[step], local)
		if err != nil {
			return nil, err
		}
		code, err := value("a daily weather code", block.WeatherCode[step])
		if err != nil {
			return nil, err
		}
		warmest, err := value("a daily maximum temperature", block.TemperatureMax[step])
		if err != nil {
			return nil, err
		}
		coldest, err := value("a daily minimum temperature", block.TemperatureMin[step])
		if err != nil {
			return nil, err
		}
		chance, err := value(
			"a daily precipitation probability",
			block.PrecipitationProbabilityMax[step],
		)
		if err != nil {
			return nil, err
		}

		days = append(days, boundary.WeatherDay{
			Time: at, WeatherCode: code, Max: warmest, Min: coldest, PrecipProbability: chance,
		})
	}
	return days, nil
}

// value reads a measurement the payload requires, naming what was missing where
// the source did not report it.
func value[T any](what string, reported *T) (T, error) {
	if reported == nil {
		var absent T
		return absent, fmt.Errorf("the response reports no value for %s", what)
	}
	return *reported, nil
}

// sameUnit reads the unit the source reported a measurement in, refusing one the
// request did not ask for.
func sameUnit(measurement string, reported *string, want string) error {
	if reported == nil {
		return fmt.Errorf("the response reports no unit for %s", measurement)
	}
	if *reported != want {
		return fmt.Errorf(
			"the response reports %s in a unit this module did not ask for",
			measurement,
		)
	}
	return nil
}

// stamp reads one of the source's local times and writes it with the location's
// offset, which is what makes the moment unambiguous wherever the payload is
// read.
func stamp(what, layout, written string, local *time.Location) (string, error) {
	at, err := time.ParseInLocation(layout, written, local)
	if err != nil {
		return "", fmt.Errorf("the response's %s time is not one this module can read", what)
	}
	return at.Format(time.RFC3339), nil
}

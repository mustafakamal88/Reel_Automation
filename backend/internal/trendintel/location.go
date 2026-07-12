package trendintel

import (
	"context"
	"regexp"
	"strings"
)

type LocationResolver interface {
	Resolve(ctx context.Context, input string) (ResolvedLocation, error)
}

type UKPostcodeResolver struct{}

var ukPostcodeRe = regexp.MustCompile(`(?i)^\s*([A-Z]{1,2}\d[A-Z\d]?)\s*(\d[A-Z]{2})?\s*$`)

func (UKPostcodeResolver) Resolve(ctx context.Context, input string) (ResolvedLocation, error) {
	_ = ctx
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ResolvedLocation{Input: input, Status: "empty"}, nil
	}
	match := ukPostcodeRe.FindStringSubmatch(trimmed)
	if match == nil {
		return ResolvedLocation{Input: input, Status: "location_resolution_not_configured", Message: "Only UK postcode format normalization is available in local development until a resolver dataset or geocoder is configured."}, nil
	}
	outward := strings.ToUpper(match[1])
	normalized := outward
	if match[2] != "" {
		normalized = outward + " " + strings.ToUpper(match[2])
	}
	region := ukRegionForOutward(outward)
	lat, lon := approximateUKOutwardCoordinates(outward)
	return ResolvedLocation{
		Input:     normalized,
		Country:   "GB",
		Region:    region,
		City:      region,
		Latitude:  lat,
		Longitude: lon,
		Status:    "resolved",
		Message:   "Resolved with a local-development UK outward-postcode approximation. Country-level trend geography is used; local video filtering only matches videos with available geographic metadata.",
	}, nil
}

func ukRegionForOutward(outward string) string {
	switch {
	case strings.HasPrefix(outward, "SW") || strings.HasPrefix(outward, "SE") || strings.HasPrefix(outward, "W") || strings.HasPrefix(outward, "E") || strings.HasPrefix(outward, "N"):
		return "London"
	case strings.HasPrefix(outward, "M"):
		return "Manchester"
	case strings.HasPrefix(outward, "B"):
		return "Birmingham"
	case strings.HasPrefix(outward, "L"):
		return "Liverpool"
	case strings.HasPrefix(outward, "G"):
		return "Glasgow"
	case strings.HasPrefix(outward, "EH"):
		return "Edinburgh"
	default:
		return "United Kingdom"
	}
}

func approximateUKOutwardCoordinates(outward string) (*float64, *float64) {
	points := map[string][2]float64{
		"SW": {51.5007, -0.1246},
		"SE": {51.4800, -0.0500},
		"W":  {51.5110, -0.2050},
		"E":  {51.5400, -0.0100},
		"N":  {51.5600, -0.1100},
		"M":  {53.4808, -2.2426},
		"B":  {52.4862, -1.8904},
		"L":  {53.4084, -2.9916},
		"G":  {55.8642, -4.2518},
		"EH": {55.9533, -3.1883},
	}
	for prefix, point := range points {
		if strings.HasPrefix(outward, prefix) {
			lat, lon := point[0], point[1]
			return &lat, &lon
		}
	}
	lat, lon := 54.5, -3.0
	return &lat, &lon
}

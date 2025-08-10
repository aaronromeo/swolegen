package id

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cespare/xxhash/v2"
)

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]+`)
var multiDash = regexp.MustCompile(`-+`)

// Slug converts an exercise name to a compact uppercase slug (A–Z0–9–), max 12 chars.
func Slug(name string) string {
	s := strings.ToUpper(name)
	s = nonAlnum.ReplaceAllString(s, "-")
	s = multiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 12 {
		s = s[:12]
		// If we truncated mid-token, cap the last token to at most 2 chars after the dash.
		if i := strings.LastIndex(s, "-"); i > 0 {
			letters := len(s) - (i + 1)
			if letters == 0 {
				// avoid trailing dash
				s = s[:i]
			} else if letters > 2 {
				s = s[:i+1+2]
			}
		}
	}
	return s
}

// WorkoutID builds YYYY-MM-DD-<kebab-location>-NN where NN is xxhash(seed)%100.
func WorkoutID(dateISO, location string, seedInput []byte) string {
	loc := strings.ToLower(location)
	loc = nonAlnum.ReplaceAllString(loc, "-")
	loc = multiDash.ReplaceAllString(loc, "-")
	loc = strings.Trim(loc, "-")
	h := xxhash.Sum64(seedInput) % 100
	return fmt.Sprintf("%s-%s-%02d", dateISO, loc, h)
}

// SetID builds <TIER>-<SLUG>-(WU#|#)
func SetID(tier, slug string, n int, warmup bool) string {
	if warmup {
		return fmt.Sprintf("%s-%s-WU%d", strings.ToUpper(tier), Slug(slug), n)
	}
	return fmt.Sprintf("%s-%s-%d", strings.ToUpper(tier), Slug(slug), n)
}

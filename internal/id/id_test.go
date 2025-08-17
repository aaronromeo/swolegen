package id

import (
	"fmt"
	"testing"

	"github.com/cespare/xxhash/v2"
)

func TestSlug_Table(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Romanian Deadlift (Barbell)", "ROMANIAN-DE"},
		{"db incline press", "DB-INCLINE-P"},
		{"  -- pull-up ** ", "PULL-UP"},
		{"abc def ghi jkl", "ABC-DEF-GHI"}, // ensure no trailing dash after truncation
		{"A---B", "A-B"},                   // collapse multiple dashes
	}
	for _, tc := range cases {
		got := Slug(tc.in)
		if got != tc.want {
			t.Fatalf("Slug(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestWorkoutID_Table(t *testing.T) {
	cases := []struct {
		name        string
		date        string
		location    string
		seed        string
		expectedLoc string
	}{
		{"simple", "2025-08-09", "home", "seed", "home"},
		{"kebab location", "2025-01-02", "Home Gym (Basement)", "abc", "home-gym-basement"},
		{"empty after cleanup", "2025-03-04", "@@@", "xyz", ""},
	}
	for _, tc := range cases {
		// Deterministic 2-digit suffix
		suffix := int(xxhash.Sum64([]byte(tc.seed)) % 100)
		expected := fmt.Sprintf("%s-%s-%02d", tc.date, tc.expectedLoc, suffix)

		got1 := WorkoutID(tc.date, tc.location, []byte(tc.seed))
		got2 := WorkoutID(tc.date, tc.location, []byte(tc.seed))

		if got1 != expected {
			t.Fatalf("%s: WorkoutID(...) = %q; want %q", tc.name, got1, expected)
		}
		if got2 != expected {
			t.Fatalf("%s: non-deterministic: second call %q; want %q", tc.name, got2, expected)
		}
	}
}

func TestSetID_Table(t *testing.T) {
	cases := []struct {
		tier string
		slug string
		n    int
		wu   bool
		want string
	}{
		{"A", "RDL", 1, true, "A-RDL-WU1"},
		{"B", "DBIP", 3, false, "B-DBIP-3"},
		{"a", "romanian deadlift (barbell)", 2, false, "A-ROMANIAN-DE-2"},
	}
	for _, tc := range cases {
		got := SetID(tc.tier, tc.slug, tc.n, tc.wu)
		if got != tc.want {
			t.Fatalf("SetID(%q,%q,%d,%v) = %q; want %q", tc.tier, tc.slug, tc.n, tc.wu, got, tc.want)
		}
	}
}

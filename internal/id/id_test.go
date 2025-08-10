package id

import "testing"

func TestSlug(t *testing.T) {
	if got := Slug("Romanian Deadlift (Barbell)"); got != "ROMANIAN-DE" {
		t.Fatalf("unexpected slug: %s", got)
	}
}

func TestWorkoutIDDeterminism(t *testing.T) {
	a := WorkoutID("2025-08-09", "home", []byte("seed"))
	b := WorkoutID("2025-08-09", "home", []byte("seed"))
	if a != b {
		t.Fatalf("ids differ: %s vs %s", a, b)
	}
}

func TestSetIDFormats(t *testing.T) {
	if got := SetID("A", "RDL", 1, true); got != "A-RDL-WU1" {
		t.Fatalf("unexpected warmup id: %s", got)
	}
	if got := SetID("B", "DBIP", 3, false); got != "B-DBIP-3" {
		t.Fatalf("unexpected work id: %s", got)
	}
}

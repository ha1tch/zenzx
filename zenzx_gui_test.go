//go:build !headless

package main

import "testing"

func TestResolveExplicitOrPersistedUsesExplicitWhenFlagPassed(t *testing.T) {
	explicit := map[string]bool{"theme": true}
	got := resolveExplicitOrPersisted(explicit, "theme", "Light", "Dark")
	if got != "Light" {
		t.Errorf("got %q, want %q (explicit flag should win)", got, "Light")
	}
}

func TestResolveExplicitOrPersistedFallsBackWhenFlagNotPassed(t *testing.T) {
	explicit := map[string]bool{} // "theme" not present -- not passed
	got := resolveExplicitOrPersisted(explicit, "theme", "Light", "Dark")
	if got != "Dark" {
		t.Errorf("got %q, want %q (persisted value should be used when the flag wasn't explicitly passed)", got, "Dark")
	}
}

func TestResolveExplicitOrPersistedIgnoresUnrelatedExplicitFlags(t *testing.T) {
	// "scale" was passed explicitly, but "theme" was not -- only
	// "theme" should matter here.
	explicit := map[string]bool{"scale": true}
	got := resolveExplicitOrPersisted(explicit, "theme", "Light", "Dark")
	if got != "Dark" {
		t.Errorf("got %q, want %q (an unrelated explicit flag should not affect this one)", got, "Dark")
	}
}

func TestResolveExplicitOrPersistedIntUsesExplicitWhenFlagPassed(t *testing.T) {
	explicit := map[string]bool{"scale": true}
	got := resolveExplicitOrPersistedInt(explicit, "scale", 4, 2)
	if got != 4 {
		t.Errorf("got %d, want 4 (explicit flag should win)", got)
	}
}

func TestResolveExplicitOrPersistedIntFallsBackWhenFlagNotPassed(t *testing.T) {
	explicit := map[string]bool{}
	got := resolveExplicitOrPersistedInt(explicit, "scale", 4, 2)
	if got != 2 {
		t.Errorf("got %d, want 2 (persisted value should be used when the flag wasn't explicitly passed)", got)
	}
}

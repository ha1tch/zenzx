package main

import "testing"

func TestLookupVideoRenderer(t *testing.T) {
	r, err := LookupVideoRenderer("")
	if err != nil {
		t.Fatalf("standard renderer should always be registered: %v", err)
	}
	if r.Name() != "" {
		t.Errorf("standard renderer Name() = %q, want \"\"", r.Name())
	}
	w, h := r.Dimensions()
	if w != ScreenWidth || h != ScreenHeight {
		t.Errorf("standard renderer Dimensions() = (%d,%d), want (%d,%d)", w, h, ScreenWidth, ScreenHeight)
	}
	bl, br, bt, bb := r.BorderMargins()
	if bl != BorderLeft || br != BorderRight || bt != BorderTop || bb != BorderBottom {
		t.Errorf("standard renderer BorderMargins() = (%d,%d,%d,%d), want (%d,%d,%d,%d)",
			bl, br, bt, bb, BorderLeft, BorderRight, BorderTop, BorderBottom)
	}
}

func TestLookupVideoRendererUnregistered(t *testing.T) {
	// These are valid -ns-graphics flag values (nonstandard.go) but have no
	// renderer yet (T-09) -- selection must fail loudly, not fall back to
	// standard. mode-timex-001-hicolour is no longer in this list: it has
	// its own renderer and its own tests (videorender_hicolour_test.go).
	for _, mode := range []string{
		NSGraphicsZenZX01,
		NSGraphicsZenZX02,
		"mode-does-not-exist",
	} {
		if _, err := LookupVideoRenderer(mode); err == nil {
			t.Errorf("LookupVideoRenderer(%q) = nil error, want an error (not yet registered)", mode)
		}
	}
}

func TestRegisterVideoRendererDuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected RegisterVideoRenderer to panic on a duplicate name")
		}
	}()
	RegisterVideoRenderer(standardVideoRenderer{}) // "" is already registered by init()
}

func TestZenZXSelectVideoRenderer(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)

	if err := zx.SelectVideoRenderer(NSGraphicsZenZX01); err == nil {
		t.Fatal("selecting an unregistered mode should error")
	}
	// A failed selection must not have changed the active renderer.
	if zx.videoRenderer.Name() != "" {
		t.Errorf("videoRenderer.Name() = %q after a failed selection, want unchanged \"\"", zx.videoRenderer.Name())
	}

	if err := zx.SelectVideoRenderer(""); err != nil {
		t.Fatalf("selecting the standard renderer should never fail: %v", err)
	}
}

package main

import "testing"

func TestParseNonStandardConfig(t *testing.T) {
	tests := []struct {
		name              string
		master, gfx, stor string
		wantErr           bool
		wantEnabled       bool
		wantGfx, wantStor string
	}{
		{"default off, nothing set", "off", "", "", false, false, "", ""},
		{"on, nothing engaged", "on", "", "", false, true, "", ""},
		{"on, graphics timex hicolour", "on", NSGraphicsTimex001HiColour, "", false, true, NSGraphicsTimex001HiColour, ""},
		{"on, graphics zenzx-01", "on", NSGraphicsZenZX01, "", false, true, NSGraphicsZenZX01, ""},
		{"on, graphics zenzx-02", "on", NSGraphicsZenZX02, "", false, true, NSGraphicsZenZX02, ""},
		{"on, storage posix", "on", "", NSStoragePosix, false, true, "", NSStoragePosix},
		{"on, graphics and storage together", "on", NSGraphicsZenZX01, NSStoragePosix, false, true, NSGraphicsZenZX01, NSStoragePosix},

		{"invalid master value", "maybe", "", "", true, false, "", ""},
		{"empty master value", "", "", "", true, false, "", ""},
		{"on, invalid graphics value", "on", "mode-bogus", "", true, false, "", ""},
		{"on, invalid storage value", "on", "", "storage-bogus", true, false, "", ""},

		{"off, graphics set: gated", "off", NSGraphicsTimex001HiColour, "", true, false, "", ""},
		{"off, storage set: gated", "off", "", NSStoragePosix, true, false, "", ""},
		{"off, both set: gated", "off", NSGraphicsZenZX02, NSStoragePosix, true, false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseNonStandardConfig(tt.master, tt.gfx, tt.stor)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got none (cfg=%+v)", cfg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Enabled != tt.wantEnabled {
				t.Errorf("Enabled = %v, want %v", cfg.Enabled, tt.wantEnabled)
			}
			if cfg.Graphics != tt.wantGfx {
				t.Errorf("Graphics = %q, want %q", cfg.Graphics, tt.wantGfx)
			}
			if cfg.Storage != tt.wantStor {
				t.Errorf("Storage = %q, want %q", cfg.Storage, tt.wantStor)
			}
		})
	}
}

func TestNonStandardConfigSummary(t *testing.T) {
	tests := []struct {
		name string
		cfg  NonStandardConfig
		want string
	}{
		{"disabled", NonStandardConfig{Enabled: false}, ""},
		{"enabled, nothing engaged", NonStandardConfig{Enabled: true}, "non-standard features: on (no sub-features engaged)"},
		{"enabled, graphics only", NonStandardConfig{Enabled: true, Graphics: NSGraphicsZenZX01}, "non-standard features: on, graphics=mode-zenzx-01"},
		{"enabled, graphics timex hicolour", NonStandardConfig{Enabled: true, Graphics: NSGraphicsTimex001HiColour}, "non-standard features: on, graphics=mode-timex-001-hicolour"},
		{"enabled, storage only", NonStandardConfig{Enabled: true, Storage: NSStoragePosix}, "non-standard features: on, storage=storage-zenzx-posix"},
		{"enabled, both", NonStandardConfig{Enabled: true, Graphics: NSGraphicsTimex001HiColour, Storage: NSStoragePosix}, "non-standard features: on, graphics=mode-timex-001-hicolour, storage=storage-zenzx-posix"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.Summary(); got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

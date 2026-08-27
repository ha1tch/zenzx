package settingsconfig

import (
	"encoding/json"
	"fmt"
	"os"

	qf "github.com/ha1tch/queryfy"
)

// Load resolves settings.json: diskPath if it exists and is valid,
// otherwise embedded (also validated, as a safety net against this
// package's own built-in default ever drifting out of sync with
// validThemes/validFonts -- a bug in the embedded default should fail
// loudly, not silently serve broken settings).
//
// diskPath may be empty (no override configured), in which case only
// embedded is tried.
func Load(diskPath string, embedded []byte, validThemes, validFonts []string) (*Result, error) {
	res := &Result{DiskPath: diskPath}

	if diskPath != "" {
		if data, err := os.ReadFile(diskPath); err == nil {
			s, verr := parseAndValidate(data, validThemes, validFonts)
			if verr == nil {
				res.Settings = s
				res.FromDisk = true
				return res, nil
			}
			res.Warning = fmt.Sprintf("%s: %v (using the built-in default instead)", diskPath, verr)
		}
		// A missing file is not a warning -- diskPath is an optional
		// override, and its absence is the expected, common case.
	}

	s, err := parseAndValidate(embedded, validThemes, validFonts)
	if err != nil {
		return nil, fmt.Errorf("built-in default settings.json is itself invalid: %w", err)
	}
	res.Settings = s
	return res, nil
}

// parseAndValidate parses data as JSON and validates it against
// Schema.
func parseAndValidate(data []byte, validThemes, validFonts []string) (*Settings, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	schema := Schema(validThemes, validFonts)
	if err := qf.Validate(raw, schema); err != nil {
		return nil, fmt.Errorf("schema validation failed: %w", err)
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		// Should be unreachable if the schema validation above passed --
		// kept as a defensive check rather than a silent zero-value
		// Settings, the same reasoning machineconfig's own
		// parseAndValidate uses.
		return nil, fmt.Errorf("internal error: valid-schema JSON failed to unmarshal into Settings: %w", err)
	}
	return &s, nil
}

// Save writes s to diskPath as JSON, atomically (write to a temp file
// in the same directory, then rename over the target) so a crash or
// power loss mid-write can never leave a half-written, corrupt
// settings.json behind -- the previous file (or none) is what's on
// disk right up until the rename, which is a single filesystem
// operation.
func Save(diskPath string, s *Settings) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("could not encode settings: %w", err)
	}
	data = append(data, '\n')

	tmp := diskPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("could not write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, diskPath); err != nil {
		os.Remove(tmp) // best-effort cleanup; the original error is what matters
		return fmt.Errorf("could not replace %s: %w", diskPath, err)
	}
	return nil
}

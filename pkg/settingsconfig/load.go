package settingsconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	qf "github.com/ha1tch/queryfy"
)

// Load resolves settings.json, and always returns settings the program can start with: an
// error is only for the built-in default being itself invalid, which is a bug in the program.
//
//   - A valid file at diskPath is used as it is, and a file that has some of the keys has the
//     rest of them from embedded, in memory (a file is not rewritten for being short).
//   - A file with nothing in it -- zero bytes, spaces, null or {} -- is recreated from
//     embedded, which is then used; it is what a file that someone has emptied, or that was
//     cut short when it was written, looks like, and is not worth a warning.
//   - A file that cannot be used -- not JSON, not an object, or a value that is not one of
//     the allowed ones -- is moved aside to diskPath + ".bad" and recreated from embedded, so
//     that what was in it is still there to be looked at, and Warning says so.
//   - A file that is not there is not an error and is not created: it is the usual case, and
//     the first setting that is changed saves it.
//
// embedded is also validated, as a safety net against this package's own built-in default ever
// drifting out of sync with validThemes/validFonts -- a bug in the embedded default should fail
// loudly, not silently serve broken settings.
//
// diskPath may be empty (no override configured), in which case only embedded is used.
func Load(diskPath string, embedded []byte, validThemes, validFonts []string) (*Result, error) {
	res := &Result{DiskPath: diskPath}

	def, err := parseAndValidate(embedded, validThemes, validFonts)
	if err != nil {
		return nil, fmt.Errorf("built-in default settings.json is itself invalid: %w", err)
	}
	res.Settings = def
	if diskPath == "" {
		return res, nil
	}

	data, err := os.ReadFile(diskPath)
	if err != nil {
		if !os.IsNotExist(err) {
			res.Warning = fmt.Sprintf("%s: %v (using the built-in default instead)", diskPath, err)
		}
		return res, nil
	}

	var raw interface{}
	if jerr := json.Unmarshal(data, &raw); jerr != nil {
		if len(strings.TrimSpace(string(data))) == 0 {
			recreate(res, def, false, "was empty")
			return res, nil
		}
		recreate(res, def, true, fmt.Sprintf("is not valid JSON: %v", jerr))
		return res, nil
	}
	obj, isObject := raw.(map[string]interface{})
	if raw == nil || (isObject && len(obj) == 0) {
		recreate(res, def, false, "had no settings in it")
		return res, nil
	}
	if !isObject {
		recreate(res, def, true, "is not a JSON object")
		return res, nil
	}

	// Some keys: the ones that are missing are the default's.
	var base map[string]interface{}
	if jerr := json.Unmarshal(embedded, &base); jerr != nil {
		return nil, fmt.Errorf("built-in default settings.json is itself invalid: %w", jerr)
	}
	for k, v := range obj {
		base[k] = v
	}
	merged, _ := json.Marshal(base)
	s, verr := parseAndValidate(merged, validThemes, validFonts)
	if verr != nil {
		recreate(res, def, true, fmt.Sprintf("is not usable: %v", verr))
		return res, nil
	}
	res.Settings = s
	res.FromDisk = true
	return res, nil
}

// recreate writes def over the file at res.DiskPath, having moved the file aside first when
// keep is set, and says what it did in res.Note, or res.Warning when the file had something
// in it that is now not used. Settings stay def whether or not the writing succeeds: a
// folder that cannot be written to is no reason not to start.
func recreate(res *Result, def *Settings, keep bool, why string) {
	path := res.DiskPath
	var kept string
	if keep {
		bad := path + ".bad"
		if err := os.Rename(path, bad); err == nil {
			kept = bad
		}
	}
	msg := fmt.Sprintf("%s %s: using the built-in default", path, why)
	if err := Save(path, def); err != nil {
		res.Warning = fmt.Sprintf("%s (and it could not be recreated: %v)", msg, err)
		return
	}
	res.Recreated = true
	msg += ", and recreated the file"
	switch {
	case kept != "":
		res.Warning = fmt.Sprintf("%s; the old one is %s", msg, kept)
	case keep:
		res.Warning = msg + " (the old one could not be moved aside, and is overwritten)"
	default:
		res.Note = msg
	}
}

// Parse is data read as settings and validated as a settings.json is: what a standalone
// program carries is checked as a file would be, by the ZenZX that starts it.
func Parse(data []byte, validThemes, validFonts []string) (*Settings, error) {
	return parseAndValidate(data, validThemes, validFonts)
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

	if dir := filepath.Dir(diskPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("could not create %s: %w", dir, err)
		}
	}
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

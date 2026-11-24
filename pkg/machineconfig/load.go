package machineconfig

import (
	"encoding/json"
	"fmt"
	"os"

	qf "github.com/ha1tch/queryfy"
)

// Load resolves machines.json: diskPath if it exists and is valid,
// otherwise embedded (which is also validated, as a safety net
// against this package's own built-in default ever drifting out of
// sync with validModelIDs -- a bug in the embedded default should
// fail loudly, not silently serve a broken menu).
//
// diskPath may be empty (no disk override configured at all), in
// which case only embedded is tried.
func Load(diskPath string, embedded []byte, validModelIDs []string) (*Result, error) {
	res := &Result{DiskPath: diskPath}

	if diskPath != "" {
		if data, err := os.ReadFile(diskPath); err == nil {
			cfg, verr := parseAndValidate(data, validModelIDs)
			if verr == nil {
				res.Config = cfg
				res.FromDisk = true
				return res, nil
			}
			res.Warning = fmt.Sprintf("%s: %v (using the built-in default instead)", diskPath, verr)
		}
		// A missing file is not a warning -- diskPath is an optional
		// override, and its absence is the expected, common case.
	}

	cfg, err := parseAndValidate(embedded, validModelIDs)
	if err != nil {
		return nil, fmt.Errorf("built-in default machines.json is itself invalid: %w", err)
	}
	res.Config = cfg
	return res, nil
}

// parseAndValidate parses data as JSON, validates it against
// ConfigSchema, and (only if that passes) checks that every entry in
// validModelIDs appears exactly once across the whole document --
// a whole-document completeness check queryfy's own per-node schema
// can't express, since it validates each node's own shape, not
// relationships between nodes.
func parseAndValidate(data []byte, validModelIDs []string) (*Config, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	schema := ConfigSchema(validModelIDs)
	if err := qf.Validate(raw, schema); err != nil {
		return nil, fmt.Errorf("schema validation failed: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Should be unreachable if the schema validation above passed,
		// since the schema already confirms the shape json.Unmarshal
		// needs -- kept as a defensive check rather than a silent
		// zero-value Config.
		return nil, fmt.Errorf("internal error: valid-schema JSON failed to unmarshal into Config: %w", err)
	}

	if err := checkCompleteness(cfg.Items, validModelIDs); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// checkCompleteness walks the whole node tree (recursing into
// Submenu.Items) and confirms every entry in validModelIDs appears
// exactly once -- catching a model accidentally duplicated (would
// give it two menu rows, one presumably a copy-paste mistake) or
// accidentally omitted (would make that model permanently
// unreachable from the Machine menu) before either becomes a
// confusing runtime symptom.
func checkCompleteness(items []Node, validModelIDs []string) error {
	seen := map[string]int{}
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			if n.Type == Model {
				seen[n.ID]++
			}
			if n.Type == Submenu {
				walk(n.Items)
			}
		}
	}
	walk(items)

	var missing, duplicated []string
	for _, id := range validModelIDs {
		switch seen[id] {
		case 0:
			missing = append(missing, id)
		case 1:
			// correct, exactly once
		default:
			duplicated = append(duplicated, id)
		}
	}
	if len(missing) > 0 || len(duplicated) > 0 {
		msg := "model coverage"
		if len(missing) > 0 {
			msg += fmt.Sprintf("; missing: %v", missing)
		}
		if len(duplicated) > 0 {
			msg += fmt.Sprintf("; duplicated: %v", duplicated)
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// Package machineconfig loads and validates the Machine menu's own
// configured layout from machines.json -- which groups, separators,
// titles, and (single-level) submenus it's organised into, and which
// display label each model gets. The -model flag identifiers
// themselves (48k, 128k, spanish48k, and so on) are never configured
// here; they're a closed set defined in zenzx_headless.go, and
// machines.json only says how to group and label them for display.
package machineconfig

// NodeType discriminates which of the four shapes a Node is. Exactly
// the fields relevant to a node's own Type should be set; a node's
// other fields are ignored (validated as absent by the queryfy schema
// in schema.go, not merely undocumented).
type NodeType string

const (
	// Separator is a thin divider line -- no other field is used.
	Separator NodeType = "separator"
	// Title is a small, non-selectable group heading -- uses Label and
	// optionally Indent.
	Title NodeType = "title"
	// Model is a selectable menu entry for one -model identifier --
	// uses ID (must be one of the closed set of known identifiers),
	// Label, and optionally Indent.
	Model NodeType = "model"
	// Submenu is a hover-opened nested menu -- uses Label and Items
	// (its own nested list, of Separator/Title/Model only: a second
	// level of Submenu is rejected by the schema, since
	// zenui.MenuBarSelection's own SubIndex reports only one level of
	// submenu selection and a second level would have nowhere to
	// report through).
	Submenu NodeType = "submenu"
)

// Node is one entry in the Machine menu's own configured layout.
type Node struct {
	Type   NodeType `json:"type"`
	ID     string   `json:"id,omitempty"`
	Label  string   `json:"label,omitempty"`
	Indent int      `json:"indent,omitempty"`
	Items  []Node   `json:"items,omitempty"`
}

// Config is the top-level shape of machines.json.
type Config struct {
	Version int    `json:"version"`
	Items   []Node `json:"items"`
}

// Result is what Load returns: the resolved config, which source it
// actually came from, and (if a disk file was present but rejected) a
// human-readable reason -- left for the caller to report however it
// reports other startup diagnostics, rather than this package writing
// to stderr itself.
type Result struct {
	Config   *Config
	FromDisk bool
	// DiskPath is the path Load was asked to check, whether or not it
	// was actually used -- useful for a caller's own log message
	// either way ("loaded from X" or "X was invalid, using built-in
	// default").
	DiskPath string
	// Warning is non-empty only when a file existed at DiskPath but was
	// rejected (unreadable, malformed JSON, failed schema validation,
	// or failed the completeness check), explaining why the built-in
	// default was used instead.
	Warning string
}

package zenui

// FieldType is the kind of control a FormField represents.
type FieldType int

const (
	FieldLabel    FieldType = iota // non-interactive text -- a heading or message body
	FieldText                      // an editable text box
	FieldCheckbox                  // a checkbox with its own label
)

// FormField is one control in a Form's grid.
type FormField struct {
	Label string
	Type  FieldType

	// Row, Col place this field in the grid, in grid cells (not
	// pixels) -- 0-indexed. ColSpan/RowSpan (treated as 1 if zero) let
	// a field occupy more than one cell in either direction.
	Row, Col         int
	ColSpan, RowSpan int

	// Text is FieldText's current value; the Form edits *Text in
	// place (append/backspace, the same convention Dialog's own
	// filename field already uses -- no mid-string cursor). Unused for
	// other field types.
	Text *string
	// Checked is FieldCheckbox's current state; the Form toggles
	// *Checked in place. Unused for other field types.
	Checked *bool

	Disabled bool
	// MinWidthCells, if non-zero, is this field's minimum width in
	// grid cells, overriding the auto-computed column width when
	// larger -- a text box usually needs more room than its own label
	// alone implies.
	MinWidthCells int
}

func (f FormField) focusable() bool {
	return !f.Disabled && (f.Type == FieldText || f.Type == FieldCheckbox)
}

func (f FormField) colSpan() int {
	if f.ColSpan <= 0 {
		return 1
	}
	return f.ColSpan
}

func (f FormField) rowSpan() int {
	if f.RowSpan <= 0 {
		return 1
	}
	return f.RowSpan
}

// FormConfig sets up a Form.
type FormConfig struct {
	Title  string
	Fields []FormField
	// GridSize is the pixel granularity every field's position and
	// size snaps to -- both column widths and row heights round up to
	// the nearest multiple of this. Zero means 8 (the package
	// default).
	GridSize int
	// Scale is the text/layout scale, matching MenuConfig.Scale's own
	// convention: zero means the package default (dlgScale).
	Scale int
	// Anchor, if non-zero, positions the form's top-left there instead
	// of centring it on screen -- for a caller (Modal) that wants to
	// own its own centring/backdrop and treat Form as pure content.
	Anchor Rect
}

// Form is a grid-laid-out collection of fields. Construct with NewForm,
// then each frame call Draw(renderer, ...) followed by Update(input) --
// the same calling convention as every other zenui widget. Reuses the
// package's Status type: Active while open, Accepted/Cancelled are set
// by a host wrapping Form (Modal) in response to its own button clicks --
// Form itself has no OK/Cancel buttons of its own, only fields.
type Form struct {
	cfg      FormConfig
	scale    int
	gridSize int

	status Status
	focus  int // index into cfg.Fields of the focused field, or -1

	bounds     Rect
	fieldRects []Rect // one per field, screen coordinates, cached from the last Draw
}

// NewForm creates a form from cfg. It never returns nil.
func NewForm(cfg FormConfig) *Form {
	scale := cfg.Scale
	if scale <= 0 {
		scale = dlgScale
	}
	gridSize := cfg.GridSize
	if gridSize <= 0 {
		gridSize = 8
	}
	f := &Form{cfg: cfg, scale: scale, gridSize: gridSize, focus: -1}
	f.focusFirst()
	return f
}

func (f *Form) focusFirst() {
	for i, fld := range f.cfg.Fields {
		if fld.focusable() {
			f.focus = i
			return
		}
	}
	f.focus = -1
}

// Status returns the form's current lifecycle state. Form itself only
// ever reports Active -- Accepted/Cancelled are a hosting widget's own
// decision (see Modal), not something Form decides for itself.
func (f *Form) Status() Status { return f.status }

// FieldRect returns the screen rect of field i, valid after the most
// recent Draw. Returns the zero Rect for an out-of-range index.
func (f *Form) FieldRect(i int) Rect {
	if i < 0 || i >= len(f.fieldRects) {
		return Rect{}
	}
	return f.fieldRects[i]
}

// Bounds returns the form's own outer rect, valid after the most recent
// Draw -- for a host (Modal) that needs to know how much space the form
// actually occupied.
func (f *Form) Bounds() Rect { return f.bounds }

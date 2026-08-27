package zenui

// ModalConfig sets up a Modal.
type ModalConfig struct {
	// Form is the field configuration for the wrapped Form -- Modal
	// owns positioning it (Form.Anchor is set internally; a caller-
	// supplied Anchor here is ignored), leaving room below it for the
	// button row.
	Form FormConfig
	// Buttons are the button labels, left to right. Must be non-empty
	// -- a modal with no way to close itself via a button can only be
	// dismissed via CancelButtonIndex's Escape handling, if that's
	// set, which is rarely what's wanted.
	Buttons []string
	// CancelButtonIndex, if >= 0, is the button Escape also triggers
	// (as if it had been clicked) -- typically whichever button means
	// "Cancel" or "No". -1 (the default) means Escape does nothing;
	// the modal can only be closed via an explicit button click.
	CancelButtonIndex int
	GridSize          int
	Scale             int
}

// Modal is a generic dialog: a Form's fields, a backdrop, and a row of
// buttons beneath it. Construct with NewModal, then each frame call
// Draw(renderer, ...) followed by Update(input) -- the same convention
// every zenui widget uses. Reuses the package's Status type: Active while
// open, Accepted once a button is clicked (Result holds which one),
// Cancelled via CancelButtonIndex's Escape handling if configured.
type Modal struct {
	cfg   ModalConfig
	form  *Form
	scale int

	status Status
	result int

	bounds      Rect
	buttonRects []Rect
}

// NewModal creates a modal from cfg. It never returns nil.
func NewModal(cfg ModalConfig) *Modal {
	scale := cfg.Scale
	if scale <= 0 {
		scale = dlgScale
	}
	formCfg := cfg.Form
	formCfg.Scale = scale
	if formCfg.GridSize <= 0 {
		formCfg.GridSize = cfg.GridSize
	}
	return &Modal{
		cfg:    cfg,
		form:   NewForm(formCfg),
		scale:  scale,
		result: -1,
	}
}

// Status returns the modal's current lifecycle state.
func (m *Modal) Status() Status { return m.status }

// Result returns the index into cfg.Buttons of the button that was
// clicked (valid once Status() == Accepted or Cancelled).
func (m *Modal) Result() int { return m.result }

// Form returns the wrapped Form, for a caller that needs to read field
// values directly rather than only through the pointers already passed
// into FormConfig.
func (m *Modal) Form() *Form { return m.form }

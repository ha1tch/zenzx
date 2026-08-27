package zenui

// Item is one row in a Menu.
type Item struct {
	Label    string
	Disabled bool
	// Checked, if non-nil, marks this item as having a checkmark/
	// checkbox indicator drawn before its label, reflecting the
	// pointed-to bool's current value. nil means no indicator at all --
	// a menu reserves indicator gutter space only if at least one of
	// its items sets this, so checkable and non-checkable items in the
	// same menu still align consistently.
	Checked *bool
	// Toggle, if true, means clicking this item flips *Checked (which
	// must be non-nil when Toggle is true) and keeps the menu open
	// (Update returns Toggled, not Accepted) rather than closing it --
	// a checkbox, not a checkmark. false (the default) is the existing
	// select-and-close behaviour every item has always had, checkmark
	// or not.
	Toggle bool
	// SubItems, if non-empty, makes this item open a nested submenu on
	// hover (matching how MenuBar's own top-level labels open their
	// dropdowns) instead of being itself selectable -- hovering it
	// opens the submenu; Accepted/Toggled/Result then refer to
	// whichever item was chosen within that submenu, not this parent
	// row. Only one level of nesting is supported: a submenu item's own
	// SubItems, if set, are ignored.
	SubItems []Item
	// Separator, if true, makes this "item" a thin horizontal divider
	// line rather than a selectable row -- Label and every other field
	// are ignored. Never hoverable, clickable, or keyboard-navigable
	// (itemEnabled excludes it the same way Disabled does), and drawn
	// at a shorter row height than a normal item, not the full
	// text-plus-padding height.
	Separator bool
	// Title, if true, makes this a small, dim, non-selectable group
	// heading -- Label is drawn (in theme.DimText, at one scale step
	// smaller than the menu's own), but every other field is ignored.
	// Never hoverable, clickable, or keyboard-navigable, the same way
	// Separator isn't. Meant to be combined with Separator to divide a
	// menu into labelled groups; indenting the items that belong to a
	// group is the caller's own choice (leading spaces in their
	// Label), not something this field does automatically.
	Title bool
}

// MenuConfig sets up a dropdown menu.
type MenuConfig struct {
	Items []Item
	// Anchor is the screen rect of the control that triggered the menu (for
	// example, the frame button that was right-clicked). The menu is
	// positioned just below it: its left edge aligns to Anchor's left edge if
	// there is room for the menu to the right; otherwise its right edge
	// aligns to Anchor's right edge, provided there is room to the left.
	Anchor Rect
	// Scale is the text/layout scale this menu draws at. Zero means "use
	// the package default" (dlgScale, matching every menu's behaviour
	// before this field existed) -- a caller only sets this to offer a
	// magnification choice independent of that default.
	Scale int
}

// Menu is a dropdown context menu anchored to a triggering control. Construct
// with NewMenu, then each frame call Draw(renderer, ...) followed by
// Update(input) — Draw computes and caches the layout that Update hit-tests
// against, the same calling convention as Dialog.
//
// Menu reuses the package's Status type: Active while open, Accepted once an
// item is picked (Result holds its index), Cancelled on Escape or a click
// outside the menu.
type Menu struct {
	cfg      MenuConfig
	hover    int // index of the item under the pointer this frame, or -1
	selected int // keyboard-selected item, persists until moved, or -1
	status   Status
	result   int
	scale    int // resolved from cfg.Scale at construction -- see MenuConfig.Scale

	// layout cache from the last Draw, used by Update's hit-testing and
	// by Draw's own item-text positioning -- padX is computed once in
	// layout() from the active theme's ItemPadXPercent and reused
	// there, rather than each place that needs it recomputing its own
	// copy and risking the two drifting apart.
	bounds      Rect
	itemRects   []Rect
	padX        int
	checkGutter int  // extra left space reserved for a checkmark/checkbox indicator; 0 if no item in this menu uses Checked
	hasSubItems bool // true if any item has SubItems, reserving right-edge arrow space

	// Submenu state: at most one open at a time, keyed by which parent
	// item (by index) currently has it open. subResult holds the
	// submenu's own chosen index once it accepts or toggles -- see
	// SubResult's doc comment for why this is separate from Result.
	subOpen   int
	subMenu   *Menu
	subResult int

	// isSubmenu is true only for a Menu instance created by
	// openSubmenu, never for a top-level dropdown MenuBar opens
	// directly. Affects border drawing: theme.DropdownBorderSkipTop
	// means "this dropdown opens directly under the bar, so its top
	// edge would just double the bar's own bottom edge" -- a
	// statement about a top-level dropdown's relationship to the bar,
	// which is never true for a submenu (it opens beside a parent
	// item, not under the bar), so a submenu always draws its full
	// border regardless of this theme setting.
	isSubmenu bool
}

// NewMenu creates a menu from cfg. It never returns nil.
func NewMenu(cfg MenuConfig) *Menu {
	scale := cfg.Scale
	if scale <= 0 {
		scale = dlgScale
	}
	return &Menu{cfg: cfg, hover: -1, selected: -1, result: -1, scale: scale, subOpen: -1, subResult: -1}
}

// Result returns the chosen item's index (valid once Status() ==
// Accepted or Toggled). For an item with SubItems, this is the parent
// item's own index -- the index of the item actually chosen inside its
// submenu is SubResult, not this.
func (m *Menu) Result() int { return m.result }

// SubResult returns the index chosen within the submenu of the item
// Result() identifies, valid only when that item has SubItems and
// Status() is Accepted or Toggled as a result of a submenu choice.
// Returns -1 when the accepted/toggled item had no submenu.
func (m *Menu) SubResult() int { return m.subResult }

// Status returns the menu's current lifecycle state.
func (m *Menu) Status() Status { return m.status }

func (m *Menu) itemEnabled(i int) bool {
	return i >= 0 && i < len(m.cfg.Items) && !m.cfg.Items[i].Disabled && !m.cfg.Items[i].Separator && !m.cfg.Items[i].Title
}

// titleScale is the scale a Title item's text draws at -- one step
// smaller than the menu's own scale, floored at 1 so it's never zero
// or negative regardless of what the menu's own scale is.
func (m *Menu) titleScale() int {
	s := m.scale - 1
	if s < 1 {
		s = 1
	}
	return s
}

// ItemRect returns the screen rect of item i, valid after the most recent
// Draw. Returns the zero Rect for an out-of-range index.
func (m *Menu) ItemRect(i int) Rect {
	if i < 0 || i >= len(m.itemRects) {
		return Rect{}
	}
	return m.itemRects[i]
}

// Items returns this menu's own item list, the way MenuBar.ItemsFor
// exposes a bar item's dropdown contents -- for a host that needs to
// inspect a standalone Menu's configuration (its own tests, mainly)
// rather than just render it.
func (m *Menu) Items() []Item {
	return m.cfg.Items
}

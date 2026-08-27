package zenui

// MenuBarItem is one top-level label in a MenuBar, with the items shown in
// its dropdown when opened.
type MenuBarItem struct {
	Label string
	Items []Item
}

// MenuBarConfig sets up a MenuBar.
type MenuBarConfig struct {
	Items []MenuBarItem
	// Height is the strip's height in pixels. Callers wanting a bar that
	// isn't a fixed height (e.g. a slide-in animation, drawn at a shrinking
	// height mid-transition) pass the desired height each Draw/Update call
	// instead -- see those methods' own barY/height parameters, which take
	// precedence over this default.
	Height int
	// Scale is the text/layout scale every dropdown this bar opens draws
	// at -- the bar's own labels always draw at scale 1 regardless (see
	// Draw), only the dropdowns themselves are affected. Zero means "use
	// the package default" (dlgScale), matching every MenuBar's
	// behaviour before this field existed. Call SetScale to change this
	// live, after construction.
	Scale int
}

// MenuBarSelection identifies an accepted choice: which top-level item,
// and which row within its dropdown.
type MenuBarSelection struct {
	BarIndex  int
	ItemIndex int
	// SubIndex is the chosen item's index within ItemIndex's own
	// SubItems, valid only when that item had SubItems set. -1
	// otherwise -- mirrors Menu.SubResult one level up, the same way
	// ItemIndex already mirrors Menu.Result.
	SubIndex int
}

// MenuBar is a horizontal strip of labels, each opening a Menu (dropdown)
// of Items when engaged. Once any dropdown is open, hovering a different
// label switches directly to it -- no click needed, the standard behaviour
// of a traditional desktop menu bar once any menu is already active.
// Construct with NewMenuBar, then each frame call Draw(renderer, ...)
// followed by Update(input, ...) -- Draw computes and caches the label
// layout that Update hit-tests against, the same calling convention Menu
// and Dialog already use.
//
// MenuBar itself has no opinion about how or when it appears on screen --
// it draws at whatever Y coordinate and height it's given each frame. A
// slide-in-on-idle reveal, an always-visible bar, or any other presentation
// policy belongs in the host, not here.
type MenuBar struct {
	cfg   MenuBarConfig
	scale int // resolved from cfg.Scale at construction -- see MenuBarConfig.Scale

	openIndex int // which top-level item's dropdown is open, or -1
	menu      *Menu

	// layout cache from the last Draw, used by Update's hit-testing (and,
	// for labelsEndX, by a host wanting to know how much space remains
	// after the last label -- see LabelsEndX).
	labelRects []Rect
	labelsEndX int

	// logoElapsed accumulates Input.DeltaTime across Update calls,
	// driving the ZSP logo's colour rotation (theme.ShowZSPLogo) --
	// runs regardless of whether that theme is currently active, so
	// switching to it mid-session doesn't reset the phase to whatever
	// arrangement happened to be first.
	logoElapsed float64
}

// NewMenuBar creates a menu bar from cfg. It never returns nil.
func NewMenuBar(cfg MenuBarConfig) *MenuBar {
	scale := cfg.Scale
	if scale <= 0 {
		scale = dlgScale
	}
	return &MenuBar{cfg: cfg, scale: scale, openIndex: -1}
}

// Active reports whether a dropdown is currently open.
func (mb *MenuBar) Active() bool { return mb.openIndex >= 0 }

// SetItems replaces the dropdown contents of the top-level item at index i
// -- for a host whose menu contents change at runtime (a file listing, a
// mode-dependent set of choices) rather than being fixed at construction.
// A no-op for an out-of-range index. If item i's dropdown is currently
// open, it's rebuilt immediately against the new items so what's on
// screen never shows stale content.
func (mb *MenuBar) SetItems(i int, items []Item) {
	if i < 0 || i >= len(mb.cfg.Items) {
		return
	}
	mb.cfg.Items[i].Items = items
	if mb.openIndex == i && mb.menu != nil {
		// Update the already-open Menu's items in place, rather than
		// recreating it via openLabel. Recreating wipes m.selected
		// (keyboard-selection state) and m.itemRects (only populated
		// by the next Draw call) every single time this fires -- fatal
		// when a host calls SetItems every frame a menu might be open,
		// the established pattern (refreshCustomROMItems,
		// refreshViewItems): every frame's fresh Menu has empty
		// itemRects until Draw catches up, so hit-testing against it
		// during that same frame's own Update always misses, making
		// the dropdown feel entirely unresponsive the whole time it's
		// open. layout() recomputes itemRects fresh from cfg.Items on
		// every Draw regardless of item count, so an in-place items
		// swap is picked up correctly on the very next Draw; an
		// out-of-range m.selected/m.hover left over from a shorter new
		// list is already handled safely elsewhere (itemEnabled bounds-
		// checks, hover is recomputed fresh every Update from current
		// mouse position anyway).
		mb.menu.cfg.Items = items
	}
}

// ItemsFor returns the item list configured for top-level bar index i --
// for a host that needs to inspect an item's Checked pointer, Toggle
// flag, or other configuration directly rather than only through
// MenuBarSelection's own after-the-fact result. Returns nil for an
// out-of-range index, matching SetItems' own bounds handling.
func (mb *MenuBar) ItemsFor(i int) []Item {
	if i < 0 || i >= len(mb.cfg.Items) {
		return nil
	}
	return mb.cfg.Items[i].Items
}

// Close dismisses any open dropdown without making a selection -- for a
// host that needs to force the bar closed (e.g. hiding it while something
// else takes over input).
func (mb *MenuBar) Close() {
	mb.openIndex = -1
	mb.menu = nil
}

func (mb *MenuBar) openLabel(i int) {
	mb.openIndex = i
	mb.menu = NewMenu(MenuConfig{
		Items:  mb.cfg.Items[i].Items,
		Anchor: mb.labelRects[i],
		Scale:  mb.scale,
	})
}

// SetScale changes the scale every future dropdown opens at. Takes effect
// the next time a dropdown is opened -- an already-open one keeps
// whatever scale it was constructed with until it's closed and reopened,
// rather than being resized out from under an in-progress hover/click.
func (mb *MenuBar) SetScale(scale int) {
	if scale <= 0 {
		scale = dlgScale
	}
	mb.scale = scale
}

// Update processes one frame of input. Returns (selection, true) the frame
// an item is accepted; otherwise a zero MenuBarSelection and false.
func (mb *MenuBar) Update(input Input) (MenuBarSelection, bool) {
	mb.logoElapsed += float64(input.DeltaTime)

	hovered := -1
	for i, r := range mb.labelRects {
		if pointInRect(input.MouseX, input.MouseY, r) {
			hovered = i
			break
		}
	}

	// Hovering a different label than the one currently open switches
	// directly to it, matching a traditional menu bar once any dropdown
	// is already engaged. Hovering the same label that's already open, or
	// hovering nothing, changes nothing here -- the open dropdown's own
	// Update (below) handles its own dismissal via click-outside/Escape.
	if hovered >= 0 && hovered != mb.openIndex {
		mb.openLabel(hovered)
	}

	if mb.openIndex < 0 {
		return MenuBarSelection{}, false
	}

	switch mb.menu.Update(input) {
	case Accepted:
		sel := MenuBarSelection{BarIndex: mb.openIndex, ItemIndex: mb.menu.Result(), SubIndex: mb.menu.SubResult()}
		mb.Close()
		return sel, true
	case Toggled:
		// Unlike Accepted, the dropdown stays open (mb.menu's own
		// status is still Active, not stored as Toggled -- see the
		// Toggled const's own doc comment) -- the host can react to
		// the selection if it wants to, but nothing here closes
		// anything.
		return MenuBarSelection{BarIndex: mb.openIndex, ItemIndex: mb.menu.Result(), SubIndex: mb.menu.SubResult()}, true
	case Cancelled:
		mb.Close()
	}
	return MenuBarSelection{}, false
}

func pointInRect(x, y int, r Rect) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Draw renders the bar's labels at the given Y and height, and whichever
// dropdown (if any) is currently open. Recomputes labelRects every call --
// a host animating the bar's position calls Draw every frame regardless of
// whether it's fully shown, so there's no stale-cache risk to guard against
// the way Menu/Dialog (drawn only while genuinely open) don't need to
// either.
// Draw renders the bar's labels at the given Y and height, and whichever
// dropdown (if any) is currently open. Recomputes labelRects every call --
// a host animating the bar's position calls Draw every frame regardless of
// whether it's fully shown, so there's no stale-cache risk to guard against
// the way Menu/Dialog (drawn only while genuinely open) don't need to
// either.
//
// The bar's own background/text use theme.Sidebar/SideText, not
// theme.Panel/Text -- a theme whose dropdown panel and bar strip need
// different treatments (SpectrumTheme's white menu panel vs. the real
// hardware's black title strip) has one already-distinct pair to draw
// each from, rather than the bar being forced to match whatever a
// dropdown's own panel colour happens to be.
//
// The border is drawn last, on top of the open label's selection fill,
// for the same reason Menu.Draw does: a label rect spans the bar's full
// height, the same span the border traces along the bottom edge, so
// drawing the border first let the fill overlap it there.
func (mb *MenuBar) Draw(r Renderer, screenW, screenH, barY, height int, theme Theme) {
	barRect := Rect{X: 0, Y: barY, W: screenW, H: height}
	if theme.UseBarGradient {
		r.FillRectGradientV(barRect, theme.BarGradientTop, theme.BarGradientBottom)
	} else {
		r.FillRect(barRect, theme.Sidebar)
	}
	if theme.UseGradientOverlay {
		// mb.labelsEndX is last frame's value at this point (this
		// frame's labels haven't been laid out yet, below) -- stable
		// in practice, since label positions don't change frame to
		// frame under normal use, and drawing this before the labels
		// (rather than after, using this frame's fresh value) is what
		// keeps the labels/rainbow/logo drawn on top from being
		// darkened by it too.
		overlayRect := Rect{X: mb.labelsEndX, Y: barY, W: screenW - mb.labelsEndX, H: height}
		r.FillRectGradientHMultiply(overlayRect, theme.GradientOverlayLeft, theme.GradientOverlayRight)
	}

	mb.labelRects = mb.labelRects[:0]
	x := 12
	for i, item := range mb.cfg.Items {
		w := r.TextWidth(item.Label, 1) + 16
		rect := Rect{X: x, Y: barY, W: w, H: height}
		mb.labelRects = append(mb.labelRects, rect)

		open := i == mb.openIndex
		textColour := theme.SideText
		if open {
			r.FillRect(rect, theme.BarSelFill)
			textColour = theme.BarSelText
		}
		textY := barY + (height-r.LineHeight(1))/2
		r.DrawText(item.Label, x+8, textY, 1, textColour)
		x += w
	}
	mb.labelsEndX = x

	if theme.ShowBarRainbow {
		drawRainbow(r, mb.labelsEndX, barY, screenW, height)
	}
	if theme.ShowZSPLogo {
		if x0, blockW, blockH, ok := zspLogoGeometry(mb.labelsEndX, screenW, height); ok {
			colourIdx := int(mb.logoElapsed / zspLogoRotationPeriod)
			drawZSPLogo(r, x0, barY, blockW, blockH, colourIdx)
		}
	}

	r.StrokeRect(Rect{X: 0, Y: barY + height - 1, W: screenW, H: 1}, theme.Border, 1)

	if mb.openIndex >= 0 && mb.menu != nil {
		mb.menu.Draw(r, screenW, screenH, theme)
	}
}

// LabelsEndX returns the X position just past the last drawn label, valid
// after the most recent Draw -- for a host that wants to know how much
// empty space remains on the right side of the bar (a decoration that
// should only appear when there's room for it, say).
func (mb *MenuBar) LabelsEndX() int { return mb.labelsEndX }

// LabelCount returns how many top-level labels this bar has.
func (mb *MenuBar) LabelCount() int { return len(mb.cfg.Items) }

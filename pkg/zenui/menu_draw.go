package zenui

// drawBorder draws a rectangular border around bounds at the given
// thickness, using four filled edge strips rather than a single
// StrokeRect call -- the only way to selectively omit one edge (skipTop)
// with the Renderer interface as it stands. Thickness is in device
// pixels, not scaled by any caller's zoom level (see
// Theme.BorderThickness's own doc comment for why).
func drawBorder(r Renderer, bounds Rect, colour Colour, thickness int, skipTop bool) {
	if thickness <= 0 {
		return
	}
	if !skipTop {
		r.FillRect(Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: thickness}, colour) // top
	}
	r.FillRect(Rect{X: bounds.X, Y: bounds.Y + bounds.H - thickness, W: bounds.W, H: thickness}, colour) // bottom
	r.FillRect(Rect{X: bounds.X, Y: bounds.Y, W: thickness, H: bounds.H}, colour)                        // left
	r.FillRect(Rect{X: bounds.X + bounds.W - thickness, Y: bounds.Y, W: thickness, H: bounds.H}, colour) // right
}

// layout computes the menu's bounds and each item's row rect, positioned just
// below cfg.Anchor. The menu's left edge aligns to the anchor's left edge if
// there is room for the menu to the right of that point; otherwise its right
// edge aligns to the anchor's right edge, provided there is room to the left.
// If neither fits (a very narrow screen), the menu is clamped inside bounds.
func (m *Menu) layout(r Renderer, screenW, screenH int, theme Theme) {
	lh := r.LineHeight(m.scale)
	padY := lh * theme.ItemPadYPercent / 100
	padX := lh * theme.ItemPadXPercent / 100
	itemH := lh + 2*padY
	m.padX = padX

	// checkGutter reserves left-edge space for a checkmark/checkbox
	// indicator, sized to one text cell -- but only if at least one
	// item in this menu actually uses Checked, so a menu with none
	// draws exactly as it did before this field existed.
	m.checkGutter = 0
	m.hasSubItems = false
	for _, it := range m.cfg.Items {
		if it.Checked != nil {
			m.checkGutter = lh
		}
		if len(it.SubItems) > 0 {
			m.hasSubItems = true
		}
	}
	// arrowGutter reserves right-edge space for a submenu arrow, same
	// sizing logic as checkGutter.
	arrowGutter := 0
	if m.hasSubItems {
		arrowGutter = lh
	}

	// separatorH is a thin divider row's own height -- short enough to
	// read as a dividing line, not a selectable row's worth of empty
	// space.
	separatorH := padY
	if separatorH < 3 {
		separatorH = 3
	}
	// titleH is a group heading's own row height, at the smaller
	// titleScale -- one padY (more compact than a normal item's 2*padY,
	// since a heading is meta content, not an interactive row).
	titleLH := r.LineHeight(m.titleScale())
	titleH := titleLH + padY

	const minMenuW = 80
	menuW := minMenuW
	rowH := make([]int, len(m.cfg.Items))
	menuH := theme.DropdownBottomPadding
	for i, it := range m.cfg.Items {
		switch {
		case it.Separator:
			rowH[i] = separatorH
			menuH += separatorH
			continue
		case it.Title:
			rowH[i] = titleH
			menuH += titleH
			w := r.TextWidth(it.Label, m.titleScale()) + 2*padX
			if w > menuW {
				menuW = w
			}
			continue
		}
		rowH[i] = itemH
		menuH += itemH
		w := r.TextWidth(it.Label, m.scale) + 2*padX + m.checkGutter + arrowGutter
		if w > menuW {
			menuW = w
		}
	}

	a := m.cfg.Anchor
	y := a.Y + a.H

	var x int
	switch {
	case a.X+menuW <= screenW:
		x = a.X
	case a.X+a.W-menuW >= 0:
		x = a.X + a.W - menuW
	default:
		x = screenW - menuW
		if x < 0 {
			x = 0
		}
	}

	m.bounds = Rect{x, y, menuW, menuH}
	m.itemRects = m.itemRects[:0]
	yOff := 0
	for i := range m.cfg.Items {
		m.itemRects = append(m.itemRects, Rect{x, y + yOff, menuW, rowH[i]})
		yOff += rowH[i]
	}
}

// Draw lays out and renders the menu. Call it before Update each frame — the
// same convention as Dialog. The border is drawn last, on top of every item
// row, not first -- an item row spans the menu's full width, the same width
// the border traces, so drawing the border first let a selection fill fully
// overlap it at the left/right edges for that row.
func (m *Menu) Draw(r Renderer, screenW, screenH int, theme Theme) {
	m.layout(r, screenW, screenH, theme)
	if m.status != Active {
		return
	}
	lh := r.LineHeight(m.scale)

	r.FillRect(m.bounds, theme.Panel)

	highlighted := m.hover
	if highlighted < 0 {
		highlighted = m.selected
	}

	for i, it := range m.cfg.Items {
		rec := m.itemRects[i]
		if it.Separator {
			lineY := rec.Y + rec.H/2
			r.DrawLine(rec.X+m.padX, lineY, rec.X+rec.W-m.padX, lineY, 1, theme.SeparatorColour)
			continue
		}
		if it.Title {
			ts := m.titleScale()
			tlh := r.LineHeight(ts)
			r.DrawText(it.Label, rec.X+m.padX, rec.Y+(rec.H-tlh)/2, ts, theme.DimText)
			continue
		}
		selected := i == highlighted && m.itemEnabled(i)
		if selected {
			r.FillRect(rec, theme.SelFill)
		}
		col := theme.Text
		switch {
		case it.Disabled:
			col = theme.Disabled
		case selected:
			col = theme.SelText
		}

		if it.Checked != nil {
			drawCheckIndicator(r, rec, m.padX, m.checkGutter, lh, *it.Checked, it.Toggle, col, theme)
		}
		textX := rec.X + m.padX + m.checkGutter
		r.DrawText(it.Label, textX, rec.Y+(rec.H-lh)/2, m.scale, col)

		if len(it.SubItems) > 0 {
			arrowX := rec.X + rec.W - m.padX - r.TextWidth(">", m.scale)
			r.DrawText(">", arrowX, rec.Y+(rec.H-lh)/2, m.scale, col)
		}
	}

	skipTop := theme.DropdownBorderSkipTop && !m.isSubmenu
	drawBorder(r, m.bounds, theme.Border, theme.BorderThickness, skipTop)

	if m.subOpen >= 0 && m.subMenu != nil {
		m.subMenu.Draw(r, screenW, screenH, theme)
	}
}

// drawCheckIndicator draws a checkmark/checkbox glyph -- a filled square
// when checked, an outlined one when not -- centred in the gutter space
// reserved at the left of rec. Built from FillRect/StrokeRect rather than
// a font character, since the bundled bitmap fonts don't reliably have a
// dedicated check glyph.
// drawCheckIndicator draws either a checkbox (an outlined box, with a
// cross mark drawn inside when checked -- for options that are NOT
// mutually exclusive, Item.Toggle true) or a checkmark (a tick mark
// alone, drawn only when checked and nothing otherwise -- for mutually
// exclusive options, Item.Toggle false). Centred in the gutter space
// reserved at the left of rec, itself positioned after padX so the
// indicator doesn't sit hard against the panel's own left border the
// way it did before padX was included here -- the same left margin
// every item's own text already gets. Built from DrawLine/StrokeRect
// rather than a font character, since the bundled bitmap fonts don't
// reliably have dedicated check/cross glyphs.
func drawCheckIndicator(r Renderer, rec Rect, padX, gutter, lh int, checked, toggle bool, col Colour, theme Theme) {
	size := lh / 2
	if size < 4 {
		size = 4
	}
	x := rec.X + padX + (gutter-size)/2
	y := rec.Y + (rec.H-size)/2

	if toggle {
		// Checkbox: the box itself is always visible, checked or not
		// -- it represents an independent on/off state, not "was this
		// picked at all". A cross, not a fill, marks checked. An
		// unchecked box always uses CheckboxEmptyColour (a neutral
		// grey, uniformly across every theme) rather than the item's
		// own computed text colour. A checked box, and its cross, use
		// theme.CheckboxColour if the theme opts in (Spectrum's own
		// fixed blue, distinct from the checkmark's green so the two
		// indicator kinds read as different at a glance, not just by
		// shape) -- otherwise the item's own computed colour, same as
		// before this field existed.
		checkedColour := col
		if theme.UseCheckboxColour {
			checkedColour = theme.CheckboxColour
		}
		boxColour := checkedColour
		if !checked {
			boxColour = theme.CheckboxEmptyColour
		}
		r.StrokeRect(Rect{X: x, Y: y, W: size, H: size}, boxColour, 1)
		if checked {
			r.DrawLine(x, y, x+size-1, y+size-1, 1, checkedColour)
			r.DrawLine(x+size-1, y, x, y+size-1, 1, checkedColour)
		}
		return
	}

	// Checkmark: nothing at all when not the current selection --
	// unlike a checkbox, there's no persistent "off" state worth
	// drawing for an item that's just not the one currently chosen
	// among mutually exclusive options. When checked, uses
	// theme.CheckmarkColour if the theme opts in (Spectrum's bright
	// green, a stable "current setting" indicator rather than one that
	// shifts with hover) -- otherwise the item's own computed colour,
	// same as before this field existed.
	if !checked {
		return
	}
	tickColour := col
	if theme.UseCheckmarkColour {
		tickColour = theme.CheckmarkColour
	}
	x1, y1 := x, y+size*5/10
	x2, y2 := x+size*35/100, y+size*85/100
	x3, y3 := x+size, y+size*15/100
	r.DrawLine(x1, y1, x2, y2, 2, tickColour)
	r.DrawLine(x2, y2, x3, y3, 2, tickColour)
}

// Update advances the menu's state from an input snapshot, hit-testing
// against the bounds cached by the last Draw. Escape or a click outside the
// menu cancels; Enter or a click on an enabled item accepts (Result holds its
// index); Up/Down move the keyboard-hover, skipping disabled items.
func (m *Menu) Update(in Input) Status {
	if m.status != Active {
		return m.status
	}

	// A submenu owns input exclusively while open.
	if m.subOpen >= 0 && m.subMenu != nil {
		switch st := m.subMenu.Update(in); st {
		case Accepted:
			m.result = m.subOpen
			m.subResult = m.subMenu.Result()
			m.subOpen = -1
			m.subMenu = nil
			m.status = Accepted
			return m.status
		case Toggled:
			// The submenu's own status stays Active (that's what makes
			// a checkbox item stay open); report the toggle this frame
			// without closing anything, parent included.
			m.result = m.subOpen
			m.subResult = m.subMenu.Result()
			return Toggled
		case Cancelled:
			m.subOpen = -1
			m.subMenu = nil
			// Return here rather than falling through to this menu's
			// own key handling below -- an Escape that just cancelled
			// the submenu must not also be seen by the parent's own
			// `if in.pressed(KeyEscape)` check in this same call,
			// which would incorrectly cancel the parent too. Hover
			// detection catching up costs one extra frame, which is
			// a better trade than double-consuming an Escape press.
			return m.status
		}
	}

	if in.pressed(KeyEscape) {
		m.status = Cancelled
		return m.status
	}
	if in.pressed(KeyUp) {
		m.moveSelected(-1)
	}
	if in.pressed(KeyDown) {
		m.moveSelected(+1)
	}
	if in.pressed(KeyEnter) {
		if m.itemEnabled(m.selected) {
			if len(m.cfg.Items[m.selected].SubItems) > 0 {
				m.openSubmenu(m.selected)
				return m.status
			}
			return m.accept(m.selected)
		}
		return m.status
	}

	m.hover = -1
	for i, rec := range m.itemRects {
		if rec.Contains(in.MouseX, in.MouseY) {
			m.hover = i
			break
		}
	}

	// Hovering a different item with SubItems opens its submenu,
	// matching MenuBar's own hover-switches-dropdowns behaviour once
	// something is already engaged.
	if m.hover >= 0 && m.itemEnabled(m.hover) && len(m.cfg.Items[m.hover].SubItems) > 0 && m.hover != m.subOpen {
		m.openSubmenu(m.hover)
	}

	if in.MousePressed {
		if m.bounds.Contains(in.MouseX, in.MouseY) {
			if m.itemEnabled(m.hover) && len(m.cfg.Items[m.hover].SubItems) == 0 {
				return m.accept(m.hover)
			}
			// Clicked inside the menu but on a disabled item, the
			// border, or a submenu-parent row (which opens on hover,
			// not click): stays Active.
		} else {
			m.status = Cancelled
		}
	}
	return m.status
}

// accept handles a click/Enter on item i: toggles it (Item.Toggle) and
// stays Active, or selects it and closes (Accepted) -- the existing
// behaviour every non-toggle item has always had. Returns the status for
// the caller to return directly, without that status necessarily being
// stored into m.status (Toggled never is -- see the Toggled const's own
// doc comment for why a checkbox has to stay interactive afterwards).
func (m *Menu) accept(i int) Status {
	it := m.cfg.Items[i]
	m.result = i
	if it.Toggle && it.Checked != nil {
		*it.Checked = !*it.Checked
		return Toggled
	}
	m.status = Accepted
	return m.status
}

// openSubmenu opens item i's SubItems as a nested Menu, positioned to the
// right of its row (or left, if there's no room -- reusing layout's own
// anchor-fallback logic unchanged via a zero-size anchor placed at the
// row's right edge, rather than a separate positioning branch).
// openSubmenu opens item i's SubItems as a nested Menu, positioned to the
// right of its row (or left, if there's no room -- reusing layout's own
// anchor-fallback logic unchanged via a zero-size anchor placed at the
// row's right edge, rather than a separate positioning branch). Shifted
// 4px left of the parent row's own right edge, so the submenu overlaps
// it slightly rather than starting exactly flush.
func (m *Menu) openSubmenu(i int) {
	const submenuOverlapX = 4
	rec := m.itemRects[i]
	m.subOpen = i
	m.subMenu = NewMenu(MenuConfig{
		Items:  m.cfg.Items[i].SubItems,
		Anchor: Rect{X: rec.X + rec.W - submenuOverlapX, Y: rec.Y, W: 0, H: 0},
		Scale:  m.scale,
	})
	m.subMenu.isSubmenu = true
}

// moveSelected shifts the keyboard-selected item by delta, wrapping, and
// skips disabled items. It is a no-op if every item is disabled.
func (m *Menu) moveSelected(delta int) {
	n := len(m.cfg.Items)
	if n == 0 {
		return
	}
	if m.selected < 0 {
		if delta > 0 {
			m.selected = 0
		} else {
			m.selected = n - 1
		}
		if m.itemEnabled(m.selected) {
			return
		}
	}
	next := m.selected
	for i := 0; i < n; i++ {
		next = (next + delta + n) % n
		if m.itemEnabled(next) {
			m.selected = next
			return
		}
	}
}

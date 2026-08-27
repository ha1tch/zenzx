package zenui

// layout runs the wrapped Form's own layout twice: once at the default
// centring (to learn its natural size), then again with an Anchor that
// accounts for the button row below it, so the form-plus-buttons
// combination as a whole ends up centred on screen, not just the form
// alone with the buttons trailing off wherever they land. Form.layout is
// pure computation (no drawing), so calling it twice has no visual side
// effects to worry about.
func (m *Modal) layout(r Renderer, screenW, screenH int, theme Theme) {
	lh := r.LineHeight(m.scale)
	gridSize := m.form.gridSize
	pad := gridSize / 2
	buttonH := lh + 2*pad
	gap := gridSize

	m.form.cfg.Anchor = Rect{}
	m.form.layout(r, screenW, screenH)
	formBounds := m.form.bounds

	totalH := formBounds.H + gap + buttonH
	originX := formBounds.X
	originY := (screenH - totalH) / 2

	m.form.cfg.Anchor = Rect{X: originX, Y: originY, W: 0, H: 0}
	m.form.layout(r, screenW, screenH)
	formBounds = m.form.bounds

	buttonRowY := formBounds.Y + formBounds.H + gap

	m.buttonRects = m.buttonRects[:0]
	x := formBounds.X + formBounds.W
	for _, label := range m.cfg.Buttons {
		w := r.TextWidth(label, m.scale) + 2*pad
		x -= w
		m.buttonRects = append(m.buttonRects, Rect{X: x, Y: buttonRowY, W: w, H: buttonH})
		x -= gap / 2
	}
	// buttonRects were appended right-to-left (each button positioned
	// leftward from the previous); reverse so index order matches
	// cfg.Buttons' own left-to-right order.
	for i, j := 0, len(m.buttonRects)-1; i < j; i, j = i+1, j-1 {
		m.buttonRects[i], m.buttonRects[j] = m.buttonRects[j], m.buttonRects[i]
	}

	m.bounds = Rect{
		X: formBounds.X,
		Y: formBounds.Y,
		W: formBounds.W,
		H: totalH,
	}
}

// Draw lays out and renders the modal: backdrop, form, button row. Call
// it before Update each frame -- the same convention every zenui widget
// uses.
func (m *Modal) Draw(r Renderer, screenW, screenH int, theme Theme) {
	if m.status != Active {
		return
	}
	m.layout(r, screenW, screenH, theme)

	r.FillRect(Rect{X: 0, Y: 0, W: screenW, H: screenH}, theme.Backdrop)

	m.form.Draw(r, screenW, screenH, theme)

	for i, label := range m.cfg.Buttons {
		rec := m.buttonRects[i]
		bg := theme.Button
		r.FillRect(rec, bg)
		drawBorder(r, rec, theme.Border, 1, false)
		lh := r.LineHeight(m.scale)
		tw := r.TextWidth(label, m.scale)
		r.DrawText(label, rec.X+(rec.W-tw)/2, rec.Y+(rec.H-lh)/2, m.scale, theme.ButtonText)
	}
}

// Update advances the modal's state: delegates to the wrapped Form first
// (text typing, checkbox toggling, tab), then handles button clicks and
// Escape (if CancelButtonIndex is set).
func (m *Modal) Update(in Input) Status {
	if m.status != Active {
		return m.status
	}

	m.form.Update(in)

	if in.pressed(KeyEscape) && m.cfg.CancelButtonIndex >= 0 {
		m.result = m.cfg.CancelButtonIndex
		m.status = Cancelled
		return m.status
	}

	if in.MousePressed {
		for i, rec := range m.buttonRects {
			if rec.Contains(in.MouseX, in.MouseY) {
				m.result = i
				if m.cfg.CancelButtonIndex >= 0 && i == m.cfg.CancelButtonIndex {
					m.status = Cancelled
				} else {
					m.status = Accepted
				}
				return m.status
			}
		}
	}

	return m.status
}

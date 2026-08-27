package zenui

// layout computes each column's width and each row's height from the
// fields that occupy them (snapped up to gridSize), then positions every
// field's rect from those cumulative offsets. A single-column field's
// natural width comes from its label/content; a field spanning multiple
// columns doesn't widen the columns it spans, it just gets the sum of
// whatever they already are -- a deliberate simplification (a full
// constraint solver is more machinery than a form of a handful of fields
// needs) documented here rather than left to be discovered by surprise.
func (f *Form) layout(r Renderer, screenW, screenH int) {
	lh := r.LineHeight(f.scale)
	cellPad := f.gridSize / 2

	numRows, numCols := 0, 0
	for _, fld := range f.cfg.Fields {
		if row := fld.Row + fld.rowSpan(); row > numRows {
			numRows = row
		}
		if col := fld.Col + fld.colSpan(); col > numCols {
			numCols = col
		}
	}

	colW := make([]int, numCols)
	rowH := make([]int, numRows)

	for _, fld := range f.cfg.Fields {
		if fld.colSpan() == 1 && fld.Col < numCols {
			w := f.naturalWidth(r, fld) + 2*cellPad
			if w > colW[fld.Col] {
				colW[fld.Col] = w
			}
		}
		if fld.rowSpan() == 1 && fld.Row < numRows {
			h := lh + 2*cellPad
			if h > rowH[fld.Row] {
				rowH[fld.Row] = h
			}
		}
	}

	snapUp := func(v int) int {
		if v <= 0 {
			return f.gridSize
		}
		return ((v + f.gridSize - 1) / f.gridSize) * f.gridSize
	}
	for i := range colW {
		colW[i] = snapUp(colW[i])
	}
	for i := range rowH {
		rowH[i] = snapUp(rowH[i])
	}

	colX := make([]int, numCols+1)
	for i := 0; i < numCols; i++ {
		colX[i+1] = colX[i] + colW[i]
	}
	rowY := make([]int, numRows+1)
	for i := 0; i < numRows; i++ {
		rowY[i+1] = rowY[i] + rowH[i]
	}

	titleH := 0
	if f.cfg.Title != "" {
		titleH = lh + 2*cellPad
	}

	formW := colX[numCols] + 2*cellPad
	formH := titleH + rowY[numRows] + 2*cellPad

	var originX, originY int
	if f.cfg.Anchor != (Rect{}) {
		originX, originY = f.cfg.Anchor.X, f.cfg.Anchor.Y
	} else {
		originX = (screenW - formW) / 2
		originY = (screenH - formH) / 2
	}

	f.bounds = Rect{X: originX, Y: originY, W: formW, H: formH}

	f.fieldRects = f.fieldRects[:0]
	for _, fld := range f.cfg.Fields {
		endCol := fld.Col + fld.colSpan()
		if endCol > numCols {
			endCol = numCols
		}
		endRow := fld.Row + fld.rowSpan()
		if endRow > numRows {
			endRow = numRows
		}
		rec := Rect{
			X: originX + cellPad + colX[fld.Col],
			Y: originY + cellPad + titleH + rowY[fld.Row],
			W: colX[endCol] - colX[fld.Col],
			H: rowY[endRow] - rowY[fld.Row],
		}
		f.fieldRects = append(f.fieldRects, rec)
	}
}

// naturalWidth is a field's own minimum content width, before grid
// snapping: its label (plus a checkbox glyph's width for FieldCheckbox),
// or MinWidthCells*gridSize if that's larger -- a text box usually needs
// more room than its label alone would suggest.
func (f *Form) naturalWidth(r Renderer, fld FormField) int {
	w := r.TextWidth(fld.Label, f.scale)
	if fld.Type == FieldCheckbox {
		w += r.LineHeight(f.scale)/2 + f.gridSize/2 // glyph + gap
	}
	if fld.MinWidthCells > 0 {
		if mw := fld.MinWidthCells * f.gridSize; mw > w {
			w = mw
		}
	}
	return w
}

// Draw lays out and renders the form. Call it before Update each frame --
// the same convention every other zenui widget uses.
func (f *Form) Draw(r Renderer, screenW, screenH int, theme Theme) {
	f.layout(r, screenW, screenH)

	r.FillRect(f.bounds, theme.Panel)
	drawBorder(r, f.bounds, theme.Border, theme.BorderThickness, theme.DropdownBorderSkipTop)

	lh := r.LineHeight(f.scale)
	cellPad := f.gridSize / 2

	if f.cfg.Title != "" {
		r.DrawText(f.cfg.Title, f.bounds.X+cellPad, f.bounds.Y+cellPad, f.scale, theme.Text)
	}

	for i, fld := range f.cfg.Fields {
		rec := f.fieldRects[i]
		focused := i == f.focus

		col := theme.Text
		if fld.Disabled {
			col = theme.Disabled
		}

		switch fld.Type {
		case FieldLabel:
			r.DrawText(fld.Label, rec.X, rec.Y+(rec.H-lh)/2, f.scale, col)

		case FieldCheckbox:
			size := lh / 2
			if size < 4 {
				size = 4
			}
			box := Rect{X: rec.X, Y: rec.Y + (rec.H-size)/2, W: size, H: size}
			checked := fld.Checked != nil && *fld.Checked
			if checked {
				r.FillRect(box, col)
			} else {
				r.StrokeRect(box, col, 1)
			}
			labelX := rec.X + size + f.gridSize/2
			r.DrawText(fld.Label, labelX, rec.Y+(rec.H-lh)/2, f.scale, col)
			if focused {
				drawBorder(r, rec, theme.SelFill, 1, false)
			}

		case FieldText:
			r.FillRect(rec, theme.Field)
			drawBorder(r, rec, theme.Border, 1, false)
			text := ""
			if fld.Text != nil {
				text = *fld.Text
			}
			r.Clip(rec)
			r.DrawText(text, rec.X+cellPad/2, rec.Y+(rec.H-lh)/2, f.scale, col)
			r.ClipEnd()
			if focused {
				drawBorder(r, rec, theme.SelFill, 1, false)
			}
		}
	}
}

// Update advances the form's state from an input snapshot, hit-testing
// against the fieldRects cached by the last Draw. Tab moves focus forward
// through focusable fields, wrapping; typing and Backspace edit the
// focused FieldText's *Text; clicking a FieldCheckbox toggles *Checked
// and focuses it; clicking a FieldText focuses it. Form has no Accept/
// Cancel of its own -- it always returns/stays Active; a hosting widget
// (Modal) decides when the form as a whole is done.
func (f *Form) Update(in Input) Status {
	if in.pressed(KeyTab) {
		f.focusNext()
	}

	if in.MousePressed {
		for i, rec := range f.fieldRects {
			if !rec.Contains(in.MouseX, in.MouseY) {
				continue
			}
			fld := f.cfg.Fields[i]
			if !fld.focusable() {
				continue
			}
			f.focus = i
			if fld.Type == FieldCheckbox && fld.Checked != nil {
				*fld.Checked = !*fld.Checked
			}
		}
	}

	if f.focus >= 0 && f.focus < len(f.cfg.Fields) {
		fld := f.cfg.Fields[f.focus]
		if fld.Type == FieldText && fld.Text != nil {
			if in.pressed(KeyBackspace) && *fld.Text != "" {
				*fld.Text = (*fld.Text)[:len(*fld.Text)-1]
			}
			for _, r := range in.Chars {
				if r >= 0x20 && r != 0x7f {
					*fld.Text += string(r)
				}
			}
		}
	}

	return f.status
}

// focusNext moves focus to the next focusable field after the current
// one, wrapping around, matching Dialog's own moveSel wrapping
// convention. A no-op if no field is focusable.
func (f *Form) focusNext() {
	n := len(f.cfg.Fields)
	if n == 0 {
		return
	}
	start := f.focus
	next := start
	for i := 0; i < n; i++ {
		next = (next + 1) % n
		if f.cfg.Fields[next].focusable() {
			f.focus = next
			return
		}
	}
	// No focusable field found at all -- leave focus as it was
	// (likely -1 already, from focusFirst finding nothing).
}

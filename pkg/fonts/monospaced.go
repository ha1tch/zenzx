package fonts

// Monospaced reports whether a bundled face is a monospaced one: every character in it has the
// same width, so columns of text line up. All the faces are drawn on a fixed cell, but only a face
// that is monospaced in its design looks right for a hex dump or other columns. TomThumb is the
// one that is not: its BDF declares SPACING "P" (proportional), whatever cell it is drawn in. An
// unknown name is not.
func Monospaced(name Name) bool {
	switch name {
	case NameSinclair, NameSinclairNarrow, NameCozette, NameSpleen, NameCreep, NameHaxorMedium:
		return true
	}
	return false
}

// NearestMonospaced is the face to use where the text needs to be monospaced: the face itself when
// it is, otherwise the monospaced face whose cell is the closest in size, by height first, then by
// width, and the one that comes first in All among equals. The cell is what a face's size is: the
// text is laid out on it.
func NearestMonospaced(name Name) Name {
	if Monospaced(name) {
		return name
	}
	ref, err := Load(name)
	if err != nil {
		return NameSpleen
	}
	best, bestH, bestW := NameSpleen, 1<<30, 1<<30
	for _, n := range All {
		if !Monospaced(n) {
			continue
		}
		f, err := Load(n)
		if err != nil {
			continue
		}
		dh, dw := abs(f.CellHeight-ref.CellHeight), abs(f.CellWidth-ref.CellWidth)
		if dh < bestH || dh == bestH && dw < bestW {
			best, bestH, bestW = n, dh, dw
		}
	}
	return best
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

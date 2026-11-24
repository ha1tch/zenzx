//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"image/color"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/raycanvas"
	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
)

// The mount drawer's look follows the dock (dockround_gui.go): a rounded panel
// that floats off the window edge and lets the machine behind it show through,
// with rounded tiles inside. The corners are raycanvas's soft ones, drawn into
// the display's own frame, the same way the dock draws.

// Slot-tile layout, in device pixels.
const (
	mountDrawerPad  = 14
	mountSlotW      = 200
	mountSlotH      = 96
	mountSlotGap    = 14
	mountDrawerRows = 1

	mountDrawerFloat   = 8  // px between the panel and the window's top edge
	mountDrawerRadius  = 16 // px, the panel's corners
	mountSlotRadius    = 10 // px, a tile's corners
	mountDrawerSidePad = 14 // px, panel inset from the slot block on each side

	// The panel lets the machine behind it show through, as the dock's does:
	// here it is most transparent at the bottom edge (nearest the machine) and
	// solid at the top, so the drawer reads as hanging from the top edge.
	mountPanelFarAlpha  = 0.5
	mountPanelEdgeAlpha = 0.7
)

// canvas is the raycanvas context the drawer draws its rounded shapes with, the
// same kind the dock uses. It draws inside the display's own frame, so its
// BeginFrame/EndFrame are never called.
func (m *mountDrawer) canvas() *raycanvas.Context {
	if m.round == nil {
		m.roundCache = raycanvas.NewSharedCache()
		m.round = raycanvas.NewContext(int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), m.roundCache)
		m.round.SetAntialias(true)
	}
	return m.round
}

// Unload frees the drawer's raycanvas resources.
func (m *mountDrawer) Unload() {
	if m == nil {
		return
	}
	if m.roundCache != nil {
		m.roundCache.Unload()
		m.roundCache = nil
	}
	m.round = nil
}

// drawerHeight is the panel's full height for the current slots.
func (m *mountDrawer) drawerHeight(lineH int) int {
	return mountDrawerPad*2 + mountSlotH + lineH + 6
}

// slotRect is slot i's on-screen rectangle at full extension (anim == 1).
func (m *mountDrawer) slotRect(i, screenW, panelTop int) rl.Rectangle {
	n := len(m.slots)
	totalW := n*mountSlotW + (n-1)*mountSlotGap
	x := (screenW - totalW) / 2
	return rl.Rectangle{
		X:      float32(x + i*(mountSlotW+mountSlotGap)),
		Y:      float32(panelTop + mountDrawerPad),
		Width:  mountSlotW,
		Height: mountSlotH,
	}
}

// slotAt returns the slot under (x,y), or nil. Only meaningful while the drawer
// is fully open.
func (m *mountDrawer) slotAt(x, y int32) *mountSlot {
	if m.anim < 1 {
		return nil
	}
	screenW := int(rl.GetScreenWidth())
	panelTop := mountDrawerFloat // the panel floats just below the top edge when open
	for i := range m.slots {
		r := m.slotRect(i, screenW, panelTop)
		if float32(x) >= r.X && float32(x) <= r.X+r.Width &&
			float32(y) >= r.Y && float32(y) <= r.Y+r.Height {
			return &m.slots[i]
		}
	}
	return nil
}

// rgba converts a theme colour to the color.RGBA raycanvas wants.
func rgba(col zenui.Colour) color.RGBA { return color.RGBA{R: col.R, G: col.G, B: col.B, A: col.A} }

// drawerFaded is col at the share a of its own opacity (the dock's helper, kept
// local so the drawer does not reach into the deck).
func drawerFaded(col zenui.Colour, a float32) color.RGBA {
	c := rgba(col)
	c.A = uint8(float32(c.A)*a + 0.5)
	return c
}

// Draw renders the drawer the dock's way: a rounded panel easing down from the
// top edge, floating off it, with a rounded tile per storage slot. text is the
// shared BDF renderer the host supplies.
func (m *mountDrawer) Draw(r zenuiraylib.Renderer, th zenui.Theme, screenW, screenH int) {
	if !m.Visible() {
		return
	}
	anim := m.anim
	lineH := r.LineHeight(1)
	fullH := m.drawerHeight(lineH)

	// Slide down from above: the panel's bottom eases from the top edge down to
	// its floating rest position, mountDrawerFloat px below the edge.
	panelH := fullH
	panelBottom := float32(panelH+mountDrawerFloat) * anim
	panelTop := int(panelBottom) - panelH

	// The panel floats as a rounded card: span the slot block plus side padding,
	// and let the machine show through (solid at top, half at the bottom edge).
	n := len(m.slots)
	totalW := n*mountSlotW + (n-1)*mountSlotGap
	px := float32((screenW-totalW)/2 - mountDrawerSidePad)
	pw := float32(totalW + 2*mountDrawerSidePad)
	c := m.canvas()
	c.FillRoundRectGradient(px, float32(panelTop), pw, float32(panelH), mountDrawerRadius,
		rgba(th.Panel), drawerFaded(th.Panel, mountPanelFarAlpha))
	c.SetLineWidth(1)
	c.StrokeRoundRectGradient(px, float32(panelTop), pw, float32(panelH), mountDrawerRadius,
		rgba(th.Border), drawerFaded(th.Border, mountPanelEdgeAlpha))

	hoverX, hoverY := rl.GetMouseX(), rl.GetMouseY()
	for i := range m.slots {
		s := &m.slots[i]
		rect := m.slotRect(i, screenW, panelTop)
		over := float32(hoverX) >= rect.X && float32(hoverX) <= rect.X+rect.Width &&
			float32(hoverY) >= rect.Y && float32(hoverY) <= rect.Y+rect.Height

		// Rounded tile fill: a usable target brightens on hover; an inert slot
		// stays dim. The fill and the frame are raycanvas's rounded shapes.
		fill := th.Button
		if !s.target {
			fill = th.Disabled
		} else if over {
			fill = th.ButtonHot
		}
		c.SetFillColor(rgba(fill))
		c.FillRoundRect(rect.X, rect.Y, rect.Width, rect.Height, mountSlotRadius)

		// A hovered drop target gets an accent frame; otherwise the theme border.
		edge := th.Border
		if s.target && over {
			edge = th.DirText
		}
		c.SetStrokeColor(rgba(edge))
		c.SetLineWidth(1)
		c.StrokeRoundRect(rect.X, rect.Y, rect.Width, rect.Height, mountSlotRadius)

		ink := th.Text
		if !s.target {
			ink = th.DimText
		}
		// The label is drawn at scale 2, so the lines under it advance past its
		// own (scale-2) height first, then by the scale-1 line height each.
		labelH := r.LineHeight(2)
		tx, ty := int(rect.X)+10, int(rect.Y)+8
		r.DrawText(s.label, tx, ty, 2, ink)
		cy := ty + labelH + 2
		r.DrawText(s.contents, tx, cy, 1, th.DimText)
		sub := s.free
		if !s.target && s.reason != "" {
			sub = s.reason
		}
		if sub != "" {
			r.DrawText(sub, tx, cy+lineH, 1, th.DimText)
		}
	}
}

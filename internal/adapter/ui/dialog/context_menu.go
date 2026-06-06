package dialog

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/theme"
)

// ContextMenu is a hand-rolled popup. Gio ships no popup primitive, so the
// widget keeps its own open/anchor state and is rendered after the rest of
// the screen by its caller so it paints on top of the underlying content.
//
// Items are keyed by index in the slice — Layout returns the index of the
// clicked item or -1. A click on the scrim (off-menu area) closes the menu
// without selecting anything.
type ContextMenu struct {
	Items  []ContextMenuItem
	open   bool
	anchor image.Point
	scrim  widget.Clickable
}

// ContextMenuItem is a single row in the popup.
type ContextMenuItem struct {
	Label string
	click widget.Clickable
}

// NewContextMenu returns a ContextMenu seeded with the given item labels.
func NewContextMenu(labels ...string) *ContextMenu {
	m := &ContextMenu{Items: make([]ContextMenuItem, len(labels))}
	for i, l := range labels {
		m.Items[i].Label = l
	}
	return m
}

// Open marks the menu visible at the given window-local pixel position.
func (m *ContextMenu) Open(at image.Point) { m.open = true; m.anchor = at }

// Close hides the menu without selecting anything.
func (m *ContextMenu) Close() { m.open = false }

// IsOpen reports whether the menu is currently visible.
func (m *ContextMenu) IsOpen() bool { return m.open }

// Anchor returns the position at which the menu was last opened.
func (m *ContextMenu) Anchor() image.Point { return m.anchor }

// Layout draws the menu (when open) and returns the index of any item that
// was clicked this frame, or -1. Selecting an item closes the menu. Clicks
// outside the menu close it without selecting anything.
//
// Click drains happen BEFORE the corresponding Layout calls because
// widget.Clickable.Layout drains the gesture queue and discards results
// (see gioui.org/widget/button.go layout()). Calling Clicked() after Layout
// always returns false.
func (m *ContextMenu) Layout(gtx layout.Context, th *theme.Theme) int {
	if !m.open {
		return -1
	}

	// Drain scrim clicks BEFORE rendering — events were queued against the
	// prior frame's registration and Layout would discard them.
	for m.scrim.Clicked(gtx) {
		m.open = false
	}
	// Per-item click drain, also before any Layout call.
	selected := -1
	for i := range m.Items {
		item := &m.Items[i]
		clicked := false
		for item.click.Clicked(gtx) {
			clicked = true
		}
		if clicked {
			selected = i
			m.open = false
		}
	}

	// If something dismissed the menu above, don't bother registering the
	// scrim and items for this frame — they'd capture stray clicks the
	// caller's own handlers can act on.
	if !m.open {
		return selected
	}

	scrimClip := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	m.scrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	scrimClip.Pop()

	width := gtx.Dp(unit.Dp(180))
	itemH := gtx.Dp(unit.Dp(32))
	height := itemH * len(m.Items)
	if height == 0 {
		return selected
	}

	panel := op.Affine(f32.Affine2D{}.Offset(f32.Pt(float32(m.anchor.X), float32(m.anchor.Y)))).Push(gtx.Ops)
	defer panel.Pop()

	bgClip := clip.UniformRRect(image.Rect(0, 0, width, height), gtx.Dp(unit.Dp(4))).Push(gtx.Ops)
	paint.ColorOp{Color: th.SurfaceContainer}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bgClip.Pop()

	stroke := gtx.Dp(unit.Dp(1))
	fillBorder(gtx, image.Rect(0, 0, width, height), stroke, th.Outline)

	pad := gtx.Dp(unit.Dp(8))
	for i := range m.Items {
		item := &m.Items[i]
		offsetY := i * itemH
		itemArea := op.Affine(f32.Affine2D{}.Offset(f32.Pt(0, float32(offsetY)))).Push(gtx.Ops)
		local := gtx
		local.Constraints = layout.Exact(image.Pt(width, itemH))
		item.click.Layout(local, func(gtx layout.Context) layout.Dimensions {
			// Render the label at natural size, left-aligned with an 8dp
			// inset, vertically centred. Return the full row size so the
			// Clickable's clip covers the entire item, not just the text.
			macro := op.Record(gtx.Ops)
			sub := gtx
			sub.Constraints.Min = image.Point{}
			sub.Constraints.Max.X = gtx.Constraints.Max.X - pad
			lbl := material.Body2(th.Material, item.Label)
			lbl.Color = th.OnSurface
			lbl.Alignment = text.Start
			dims := lbl.Layout(sub)
			call := macro.Stop()

			dy := (gtx.Constraints.Max.Y - dims.Size.Y) / 2
			if dy < 0 {
				dy = 0
			}
			off := op.Affine(f32.Affine2D{}.Offset(f32.Pt(float32(pad), float32(dy)))).Push(gtx.Ops)
			call.Add(gtx.Ops)
			off.Pop()
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		itemArea.Pop()
	}
	return selected
}

func fillBorder(gtx layout.Context, r image.Rectangle, stroke int, col color.NRGBA) {
	fill := func(rr image.Rectangle) {
		defer clip.Rect{Min: rr.Min, Max: rr.Max}.Push(gtx.Ops).Pop()
		paint.ColorOp{Color: col}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	}
	fill(image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+stroke))
	fill(image.Rect(r.Min.X, r.Max.Y-stroke, r.Max.X, r.Max.Y))
	fill(image.Rect(r.Min.X, r.Min.Y, r.Min.X+stroke, r.Max.Y))
	fill(image.Rect(r.Max.X-stroke, r.Min.Y, r.Max.X, r.Max.Y))
}

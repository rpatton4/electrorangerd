package canvas

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
	"gioui.org/widget/material"
)

// ShearAngle is the cabinet-projection shear ratio along the X axis. A value
// of 0.5 corresponds to a 30-degree cabinet projection — the classic
// axonometric look used by Figma / Notion / Affinity Designer for component
// thumbnails. Adjust via Canvas.Shear.
const ShearAngle = 0.5

// Canvas renders a 2.5D ERD canvas. It owns the shear matrix used for the
// "3D look" and exposes helpers that keep entity titles upright while the
// surrounding box chrome is sheared.
type Canvas struct {
	Shear float32
}

// New constructs a Canvas with the default cabinet-projection shear.
func New() *Canvas {
	return &Canvas{Shear: ShearAngle}
}

// shearAffine returns the f32.Affine2D producing the cabinet projection.
// Pure shear along X — no rotation, no perspective. The inverse is the
// negation of the shear factor; mouse hit-testing must invert before
// calling diagram.Hit (this Plan C iteration does not yet implement
// hit-testing).
func (c *Canvas) shearAffine() f32.Affine2D {
	return f32.Affine2D{}.Shear(f32.Point{}, c.Shear, 0)
}

// SampleEntity renders one sheared entity box with an un-sheared title on
// top. This is the minimal demonstration that the 2.5D pattern works
// inside Gio's 2D pipeline. Full per-entity walk over a domain.Project
// arrives with the real canvas implementation.
func (c *Canvas) SampleEntity(gtx layout.Context, th *material.Theme, title string) layout.Dimensions {
	const boxW, boxH = 220, 96
	bg := color.NRGBA{R: 0x22, G: 0x44, B: 0x88, A: 0xFF}
	shadow := color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x40}
	boxSize := image.Pt(boxW, boxH)

	// Drop shadow: offset translation, no shear — flat under the box.
	shadowStack := op.Affine(f32.Affine2D{}.Offset(f32.Pt(6, 8))).Push(gtx.Ops)
	shadowRect := clip.Rect{Max: boxSize}.Push(gtx.Ops)
	paint.ColorOp{Color: shadow}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	shadowRect.Pop()
	shadowStack.Pop()

	// Sheared box chrome.
	box := op.Affine(c.shearAffine()).Push(gtx.Ops)
	bgRect := clip.Rect{Max: boxSize}.Push(gtx.Ops)
	paint.ColorOp{Color: bg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bgRect.Pop()
	box.Pop()

	// Un-sheared title text. Translate to the box's logical centre and draw
	// upright — Gio rasterizes glyphs upright; sheared text looks broken.
	titleStack := op.Affine(f32.Affine2D{}.Offset(f32.Pt(boxW/2, boxH/2))).Push(gtx.Ops)
	lbl := material.Label(th, unit.Sp(14), title)
	lbl.Color = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	lbl.Alignment = text.Middle
	lbl.Layout(gtx)
	titleStack.Pop()

	return layout.Dimensions{Size: image.Pt(boxW+12, boxH+12)}
}

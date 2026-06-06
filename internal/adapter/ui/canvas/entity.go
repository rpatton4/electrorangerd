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

	"github.com/InfiniteSkye/electrorangerd/internal/domain"
)

// EntityPalette is the colour set Canvas.Entity paints with. The caller
// builds it from its own theme.Theme — keeping the canvas package free of
// any dependency on internal/adapter/ui/theme.
type EntityPalette struct {
	Surface       color.NRGBA
	HeaderSurface color.NRGBA
	Stroke        color.NRGBA
	OnSurface     color.NRGBA
	Shadow        color.NRGBA
}

// Entity renders a single ERD entity at the current transform origin as a
// flat rectangular table: header strip on top, then one row per attribute
// with two columns (key marker + attribute name). The caller positions the
// entity by pushing an op.Affine offset before calling and popping after.
func (c *Canvas) Entity(gtx layout.Context, th *material.Theme, p EntityPalette, e domain.Entity) layout.Dimensions {
	headerH := gtx.Dp(unit.Dp(28))
	rowH := gtx.Dp(unit.Dp(24))
	boxW := gtx.Dp(unit.Dp(220))
	keyW := gtx.Dp(unit.Dp(28))
	stroke := gtx.Dp(unit.Dp(1))
	rows := len(e.Attributes)
	boxH := headerH + rows*rowH
	size := image.Pt(boxW, boxH)

	// Drop shadow for depth.
	shadowOffset := op.Affine(f32.Affine2D{}.Offset(f32.Pt(6, 8))).Push(gtx.Ops)
	shadowClip := clip.Rect{Max: size}.Push(gtx.Ops)
	paint.ColorOp{Color: p.Shadow}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	shadowClip.Pop()
	shadowOffset.Pop()

	// Flat rectangular chrome — body fill, header strip, dividers, outline.
	fillRect(gtx, image.Rect(0, 0, boxW, boxH), p.Surface)
	fillRect(gtx, image.Rect(0, 0, boxW, headerH), p.HeaderSurface)
	fillRect(gtx, image.Rect(0, headerH-stroke, boxW, headerH), p.Stroke)
	for i := 1; i < rows; i++ {
		y := headerH + i*rowH
		fillRect(gtx, image.Rect(0, y-stroke, boxW, y), p.Stroke)
	}
	if rows > 0 {
		fillRect(gtx, image.Rect(keyW, headerH, keyW+stroke, boxH), p.Stroke)
	}
	strokeOutline(gtx, size, stroke, p.Stroke)

	// Text: header label, then per-row key marker + attribute name.
	textInArea(gtx, image.Rect(0, 0, boxW, headerH), layout.Center, func(gtx layout.Context) layout.Dimensions {
		lbl := material.Label(th, unit.Sp(14), e.Name)
		lbl.Color = p.OnSurface
		lbl.Alignment = text.Middle
		return lbl.Layout(gtx)
	})
	for i, attr := range e.Attributes {
		rowY := headerH + i*rowH
		textInArea(gtx, image.Rect(0, rowY, keyW, rowY+rowH), layout.Center, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, keyMarker(attr))
			lbl.Color = p.OnSurface
			lbl.Alignment = text.Middle
			return lbl.Layout(gtx)
		})
		pad := gtx.Dp(unit.Dp(8))
		textInArea(gtx, image.Rect(keyW+pad, rowY, boxW-pad, rowY+rowH), layout.W, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, attr.Name)
			lbl.Color = p.OnSurface
			return lbl.Layout(gtx)
		})
	}

	return layout.Dimensions{Size: image.Pt(boxW+12, boxH+12)}
}

func fillRect(gtx layout.Context, r image.Rectangle, col color.NRGBA) {
	defer clip.Rect{Min: r.Min, Max: r.Max}.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

func strokeOutline(gtx layout.Context, size image.Point, stroke int, col color.NRGBA) {
	fillRect(gtx, image.Rect(0, 0, size.X, stroke), col)
	fillRect(gtx, image.Rect(0, size.Y-stroke, size.X, size.Y), col)
	fillRect(gtx, image.Rect(0, 0, stroke, size.Y), col)
	fillRect(gtx, image.Rect(size.X-stroke, 0, size.X, size.Y), col)
}

func textInArea(gtx layout.Context, r image.Rectangle, dir layout.Direction, body func(gtx layout.Context) layout.Dimensions) {
	offset := op.Affine(f32.Affine2D{}.Offset(f32.Pt(float32(r.Min.X), float32(r.Min.Y)))).Push(gtx.Ops)
	defer offset.Pop()
	local := gtx
	local.Constraints = layout.Exact(image.Pt(r.Dx(), r.Dy()))
	dir.Layout(local, body)
}

func keyMarker(a domain.Attribute) string {
	if a.Primary {
		return "PK"
	}
	return ""
}

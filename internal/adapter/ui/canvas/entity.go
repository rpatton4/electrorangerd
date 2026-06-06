package canvas

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/rpatton4/electrorangerd/internal/domain"
)

// EntityPalette is the colour set Canvas.Entity paints with. The caller
// builds it from its own theme.Theme — keeping the canvas package free of
// any dependency on internal/adapter/ui/theme. Selection is the accent
// used by Canvas.EntitySelection to draw the bold outline and attachment
// handles around a selected entity.
type EntityPalette struct {
	Surface       color.NRGBA
	HeaderSurface color.NRGBA
	Stroke        color.NRGBA
	OnSurface     color.NRGBA
	Shadow        color.NRGBA
	Selection     color.NRGBA
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

	// Text: header label, then per-row key marker + attribute name. All three
	// fields are left-aligned with a consistent 8dp inset from the left edge
	// of their container box, matching the right column's padding. Horizontal
	// alignment is text.Start; vertical centring is computed manually in
	// textInArea because widget.Label clamps its reported dims to
	// gtx.Constraints, which defeats layout.Direction.Layout's offset pass.
	pad := gtx.Dp(unit.Dp(8))
	textInArea(gtx, image.Rect(pad, 0, boxW-pad, headerH), func(gtx layout.Context) layout.Dimensions {
		lbl := material.Label(th, unit.Sp(14), e.Name)
		lbl.Color = p.OnSurface
		lbl.Alignment = text.Start
		return lbl.Layout(gtx)
	})
	for i, attr := range e.Attributes {
		rowY := headerH + i*rowH
		textInArea(gtx, image.Rect(pad, rowY, keyW, rowY+rowH), func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, keyMarker(attr))
			lbl.Color = p.OnSurface
			lbl.Alignment = text.Start
			return lbl.Layout(gtx)
		})
		textInArea(gtx, image.Rect(keyW+pad, rowY, boxW-pad, rowY+rowH), func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, attr.Name)
			lbl.Color = p.OnSurface
			lbl.Alignment = text.Start
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

// textInArea draws body at its natural size inside r, vertically centred.
// Horizontal alignment is delegated to text.Alignment on the body's label
// because widget.Label clamps its reported height to the constraint Min,
// which makes layout.Direction.Layout's offset math collapse to zero.
func textInArea(gtx layout.Context, r image.Rectangle, body func(gtx layout.Context) layout.Dimensions) {
	macro := op.Record(gtx.Ops)
	local := gtx
	local.Constraints = layout.Constraints{Max: image.Pt(r.Dx(), r.Dy())}
	dims := body(local)
	call := macro.Stop()

	dy := (r.Dy() - dims.Size.Y) / 2
	if dy < 0 {
		dy = 0
	}
	offset := op.Affine(f32.Affine2D{}.Offset(f32.Pt(float32(r.Min.X), float32(r.Min.Y+dy)))).Push(gtx.Ops)
	defer offset.Pop()
	call.Add(gtx.Ops)
}

func keyMarker(a domain.Attribute) string {
	return a.KeyKind.String()
}

// HandleCount is the number of attachment handles drawn around each entity
// when selected. Indices 0..HandleCount-1 wrap the box edges in clockwise
// order starting from the top-left handle.
const HandleCount = 8

// HandleOutward returns the unit vector pointing away from the entity at
// the given handle index. Callers that route relationship lines
// orthogonally use this so the line leaves each handle perpendicular to
// the entity edge, which keeps the crow's-foot marker readable.
func HandleOutward(handle int) f32.Point {
	switch handle {
	case 0, 1:
		return f32.Pt(0, -1)
	case 2, 3:
		return f32.Pt(1, 0)
	case 4, 5:
		return f32.Pt(0, 1)
	case 6, 7:
		return f32.Pt(-1, 0)
	}
	return f32.Point{}
}

// HandlePosition returns the offset (in gtx pixels) of the handle at index
// from the entity's top-left corner. Both EntitySelection and any caller
// that needs to attach a line to a handle should use this so the visual
// circles and the hit / line geometry stay in lockstep.
func HandlePosition(gtx layout.Context, e domain.Entity, handle int) (x, y float32) {
	headerH := float32(gtx.Dp(unit.Dp(28)))
	rowH := float32(gtx.Dp(unit.Dp(24)))
	boxW := float32(gtx.Dp(unit.Dp(220)))
	rows := float32(len(e.Attributes))
	boxH := headerH + rows*rowH
	switch handle {
	case 0:
		return boxW / 3, 0
	case 1:
		return 2 * boxW / 3, 0
	case 2:
		return boxW, boxH / 3
	case 3:
		return boxW, 2 * boxH / 3
	case 4:
		return 2 * boxW / 3, boxH
	case 5:
		return boxW / 3, boxH
	case 6:
		return 0, 2 * boxH / 3
	case 7:
		return 0, boxH / 3
	}
	return 0, 0
}

// EntitySelection paints the selection chrome over an entity already drawn
// at the current transform origin: a thicker accent outline and eight small
// attachment-point circles, two per side at the 1/3 and 2/3 marks. The
// circles are the anchor points relationship lines hook onto.
func (c *Canvas) EntitySelection(gtx layout.Context, p EntityPalette, e domain.Entity) {
	headerH := gtx.Dp(unit.Dp(28))
	rowH := gtx.Dp(unit.Dp(24))
	boxW := gtx.Dp(unit.Dp(220))
	rows := len(e.Attributes)
	boxH := headerH + rows*rowH
	size := image.Pt(boxW, boxH)

	boldStroke := gtx.Dp(unit.Dp(2))
	strokeOutline(gtx, size, boldStroke, p.Selection)

	radius := gtx.Dp(unit.Dp(4))
	for h := 0; h < HandleCount; h++ {
		x, y := HandlePosition(gtx, e, h)
		drawHandle(gtx, image.Pt(int(x), int(y)), radius, p.Selection)
	}
}

// RelationshipLine draws a polyline through pts in the selection accent
// colour. With two points it is a straight line; with more it forms a
// connected path with right-angle bends. bold thickens the stroke so the
// diagram view can mark the currently-selected relationship.
func (c *Canvas) RelationshipLine(gtx layout.Context, p EntityPalette, pts []f32.Point, bold bool) {
	if len(pts) < 2 {
		return
	}
	width := float32(gtx.Dp(unit.Dp(2)))
	if bold {
		width = float32(gtx.Dp(unit.Dp(4)))
	}
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(pts[0])
	for _, pt := range pts[1:] {
		path.LineTo(pt)
	}
	spec := path.End()
	paint.FillShape(gtx.Ops, p.Selection, clip.Stroke{Path: spec, Width: width}.Op())
}

// RelationshipMarker draws the crow's-foot cardinality glyph at one end of
// a relationship line. at is the line endpoint (on the entity edge);
// towardOther is the line's other endpoint, used to orient the marker so
// it sits along the line just inside the endpoint. CrowsFootUnspecified
// renders nothing.
func (c *Canvas) RelationshipMarker(gtx layout.Context, p EntityPalette, at, towardOther f32.Point, marker domain.CrowsFoot) {
	if marker == domain.CrowsFootUnspecified {
		return
	}
	dx := towardOther.X - at.X
	dy := towardOther.Y - at.Y
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if length < 1 {
		return
	}
	// unit direction from at toward other endpoint
	ux := dx / length
	uy := dy / length
	// perpendicular (90° CCW)
	px := -uy
	py := ux

	lineW := float32(gtx.Dp(unit.Dp(2)))
	d1 := float32(gtx.Dp(unit.Dp(14)))
	d2 := float32(gtx.Dp(unit.Dp(26)))
	perpHalf := float32(gtx.Dp(unit.Dp(7)))
	crowsDepth := float32(gtx.Dp(unit.Dp(10)))
	crowsWide := float32(gtx.Dp(unit.Dp(7)))
	circleR := float32(gtx.Dp(unit.Dp(4)))
	twinGap := float32(gtx.Dp(unit.Dp(4)))

	perpLineAt := func(d float32) {
		cx := at.X + ux*d
		cy := at.Y + uy*d
		drawLine(gtx, p.Selection,
			f32.Pt(cx+px*perpHalf, cy+py*perpHalf),
			f32.Pt(cx-px*perpHalf, cy-py*perpHalf),
			lineW,
		)
	}
	crowsFootAt := func(d float32) {
		// Vertex sits at distance d from at; three prongs extend back
		// toward at, with the outer two prongs angled outward.
		vx := at.X + ux*d
		vy := at.Y + uy*d
		v := f32.Pt(vx, vy)
		cend := f32.Pt(vx-ux*crowsDepth, vy-uy*crowsDepth)
		uend := f32.Pt(cend.X+px*crowsWide, cend.Y+py*crowsWide)
		lend := f32.Pt(cend.X-px*crowsWide, cend.Y-py*crowsWide)
		drawLine(gtx, p.Selection, v, uend, lineW)
		drawLine(gtx, p.Selection, v, cend, lineW)
		drawLine(gtx, p.Selection, v, lend, lineW)
	}
	circleAt := func(d float32) {
		cx := at.X + ux*d
		cy := at.Y + uy*d
		drawCircleOutline(gtx, p.Selection, f32.Pt(cx, cy), circleR, lineW)
	}

	switch marker {
	case domain.CrowsFootZeroOrOne:
		perpLineAt(d1)
		circleAt(d2)
	case domain.CrowsFootOne:
		perpLineAt(d1)
	case domain.CrowsFootZeroOrMany:
		crowsFootAt(d1)
		circleAt(d2)
	case domain.CrowsFootMany:
		crowsFootAt(d1)
	case domain.CrowsFootOneAndOnlyOne:
		perpLineAt(d1)
		perpLineAt(d1 + twinGap)
	case domain.CrowsFootOneOrMany:
		crowsFootAt(d1)
		perpLineAt(d2)
	}
}

func drawLine(gtx layout.Context, col color.NRGBA, from, to f32.Point, width float32) {
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(from)
	path.LineTo(to)
	spec := path.End()
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: spec, Width: width}.Op())
}

func drawCircleOutline(gtx layout.Context, col color.NRGBA, centre f32.Point, radius, width float32) {
	const kappaC = 0.5522847498307933
	kappa := radius * kappaC
	cx, cy := centre.X, centre.Y
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(cx+radius, cy))
	path.CubeTo(f32.Pt(cx+radius, cy-kappa), f32.Pt(cx+kappa, cy-radius), f32.Pt(cx, cy-radius))
	path.CubeTo(f32.Pt(cx-kappa, cy-radius), f32.Pt(cx-radius, cy-kappa), f32.Pt(cx-radius, cy))
	path.CubeTo(f32.Pt(cx-radius, cy+kappa), f32.Pt(cx-kappa, cy+radius), f32.Pt(cx, cy+radius))
	path.CubeTo(f32.Pt(cx+kappa, cy+radius), f32.Pt(cx+radius, cy+kappa), f32.Pt(cx+radius, cy))
	path.Close()
	spec := path.End()
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: spec, Width: width}.Op())
}

func drawHandle(gtx layout.Context, centre image.Point, radius int, col color.NRGBA) {
	bb := image.Rect(centre.X-radius, centre.Y-radius, centre.X+radius, centre.Y+radius)
	defer clip.Ellipse{Min: bb.Min, Max: bb.Max}.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

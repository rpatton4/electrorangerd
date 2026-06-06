// Package infomenu is the reusable primary navigation element: a small
// circular nav button that opens a larger info menu wheel overlay
// (chrome dial) above a dimming scrim. Any screen that wants the
// navigation simply embeds *InfoMenu in its host struct and calls
// LayoutNavButton where the button should sit + LayoutOverlay after
// its main content so the overlay paints above it.
package infomenu

import (
	"image"
	"image/color"
	"log/slog"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// openDuration is how long the open animation takes: over this span
// the wheel scales from 1/8 to full size and rotates 360°, and the
// scrim alpha fades in alongside.
const openDuration = 500 * time.Millisecond

// InfoMenu owns the state, click handlers, and embedded chrome
// images needed by the navigation element. A single InfoMenu instance
// is meant to live for the lifetime of an App and be reused across
// every screen that surfaces the nav button.
type InfoMenu struct {
	log *slog.Logger

	open     bool
	openedAt time.Time

	navClick   widget.Clickable
	menuClick  widget.Clickable
	scrimClick widget.Clickable

	navImage  paint.ImageOp
	menuImage paint.ImageOp
}

// New builds an InfoMenu with the embedded chrome assets decoded
// ready for paint.
func New(log *slog.Logger) *InfoMenu {
	return &InfoMenu{
		log:       log,
		navImage:  decodeNavButton(log),
		menuImage: decodeMenuButton(log),
	}
}

// LayoutInfoArea draws the bottom info area: a full-width white
// horizon line that bumps smoothly up and over the centred 56dp nav
// button. Below the line is intentionally empty space where hosts
// can later place buttons and information to the left and right of
// the bump. While the menu wheel is open the nav button hides and
// stops registering its click area (so the scrim above intercepts
// taps there) but the area's height stays the same so the layout
// doesn't shift.
//
// Hosts call this in the bottom Rigid slot of their main layout —
// it sizes itself to the full available width and a fixed height,
// so the caller doesn't need to manage positioning.
func (m *InfoMenu) LayoutInfoArea(gtx layout.Context) layout.Dimensions {
	if m.open {
		// Drain any nav clicks captured before the slot stopped
		// registering, so they don't pile up.
		for m.navClick.Clicked(gtx) {
		}
	} else if m.navClick.Clicked(gtx) {
		m.open = true
		m.openedAt = gtx.Now
	}

	areaW := gtx.Constraints.Max.X
	areaH := gtx.Dp(unit.Dp(100))
	btnSz := gtx.Dp(unit.Dp(56))
	cx := areaW / 2
	baselineY := gtx.Dp(unit.Dp(30))
	bumpApexY := gtx.Dp(unit.Dp(0))
	bumpHalfW := gtx.Dp(unit.Dp(75))
	strokeW := float32(gtx.Dp(unit.Dp(2)))
	btnY := gtx.Dp(unit.Dp(7))

	// Build the horizon path: flat segment, smooth cubic bump up to
	// apex, mirror bump back down, flat segment. Each half of the
	// bump uses control points that produce horizontal tangents at
	// both ends so the join with the flat segments is seamless.
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(0, float32(baselineY)))
	p.LineTo(f32.Pt(float32(cx-bumpHalfW), float32(baselineY)))
	p.CubeTo(
		f32.Pt(float32(cx-bumpHalfW/2), float32(baselineY)),
		f32.Pt(float32(cx-bumpHalfW/2), float32(bumpApexY)),
		f32.Pt(float32(cx), float32(bumpApexY)),
	)
	p.CubeTo(
		f32.Pt(float32(cx+bumpHalfW/2), float32(bumpApexY)),
		f32.Pt(float32(cx+bumpHalfW/2), float32(baselineY)),
		f32.Pt(float32(cx+bumpHalfW), float32(baselineY)),
	)
	p.LineTo(f32.Pt(float32(areaW), float32(baselineY)))
	spec := p.End()

	paint.FillShape(gtx.Ops,
		color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		clip.Stroke{Path: spec, Width: strokeW}.Op(),
	)

	// Position the nav button centred horizontally at a fixed Y so
	// it stays put when the line raises/lowers; the gap between the
	// line baseline and the button top is the visual breathing room
	// the design wants.
	btnX := cx - btnSz/2
	offset := op.Offset(image.Pt(btnX, btnY)).Push(gtx.Ops)
	if !m.open {
		m.navClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(btnSz, btnSz)
			gtx.Constraints = layout.Exact(size)
			defer clip.Ellipse{Max: size}.Push(gtx.Ops).Pop()
			return widget.Image{
				Src:      m.navImage,
				Fit:      widget.Cover,
				Position: layout.Center,
			}.Layout(gtx)
		})
	}
	offset.Pop()

	return layout.Dimensions{Size: image.Pt(areaW, areaH)}
}

// LayoutWheel draws the mode wheel image at the requested dp size
// with the same scale+rotation open-animation the overlay uses,
// starting from animStart. Pass a zero time.Time (or any time more
// than openDuration in the past) to render the wheel statically at
// full size. The wheel is the chrome dial with the four mode labels
// — reusable so any screen (e.g. the welcome chooser) can present
// the wheel as a focal element without the overlay's scrim. Clicks
// on the four label sectors land in a future iteration.
func (m *InfoMenu) LayoutWheel(gtx layout.Context, sizeDp unit.Dp, animStart time.Time) layout.Dimensions {
	sz := gtx.Dp(sizeDp)
	progress := m.animationProgress(gtx, animStart)
	return m.renderWheel(gtx, sz, progress)
}

// animationProgress returns the 0..1 progress of an open animation
// that began at startedAt. Schedules an invalidate while the
// animation is still running so the frame loop keeps re-firing.
func (m *InfoMenu) animationProgress(gtx layout.Context, startedAt time.Time) float32 {
	elapsed := gtx.Now.Sub(startedAt)
	progress := float32(elapsed) / float32(openDuration)
	if progress >= 1 {
		progress = 1
	} else if progress < 0 {
		progress = 0
	}
	if progress < 1 {
		gtx.Execute(op.InvalidateCmd{})
	}
	return progress
}

// renderWheel draws the wheel image at the given pixel size with
// the supplied animation progress applied as a scale+rotate affine
// around the wheel's centre. Caller controls outer positioning
// (e.g. via layout.Center) and any input-clip wrapping.
func (m *InfoMenu) renderWheel(gtx layout.Context, sz int, progress float32) layout.Dimensions {
	size := image.Pt(sz, sz)
	gtx.Constraints = layout.Exact(size)

	scale := 0.125 + 0.875*progress
	rotation := progress * 2 * float32(math.Pi)
	centre := f32.Pt(float32(sz)/2, float32(sz)/2)
	affine := f32.Affine2D{}.
		Rotate(centre, rotation).
		Scale(centre, f32.Pt(scale, scale))
	defer op.Affine(affine).Push(gtx.Ops).Pop()

	return widget.Image{
		Src:      m.menuImage,
		Fit:      widget.Cover,
		Position: layout.Center,
	}.Layout(gtx)
}

// LayoutOverlay paints the dim scrim and the animated info menu
// wheel when the menu is open, returning zero dimensions when
// closed. Call this AFTER the host's main content so the overlay
// composes on top of it. Pass the outer (full-window) gtx so the
// scrim covers the full viewport and the wheel sits at the window
// centre.
//
// On open the wheel scales from 1/8 → full size while rotating 360°
// over openDuration; the scrim's alpha fades in over the same span.
// Clicks anywhere outside the wheel hit the scrim and dismiss the
// menu; clicks on the wheel itself currently do nothing.
func (m *InfoMenu) LayoutOverlay(gtx layout.Context) layout.Dimensions {
	if !m.open {
		return layout.Dimensions{}
	}

	_ = m.menuClick.Clicked(gtx) // consume; wheel does nothing yet
	if m.scrimClick.Clicked(gtx) {
		// Close now AND skip painting the overlay this frame —
		// otherwise the current frame still commits scrim + wheel
		// ops and the dismiss looks like it took two clicks.
		m.open = false
		gtx.Execute(op.InvalidateCmd{})
		return layout.Dimensions{}
	}

	progress := m.animationProgress(gtx, m.openedAt)

	// Scrim — semi-opaque near-black across the whole window so the
	// host shell behind reads as muted. Alpha fades in with the
	// animation rather than snapping on.
	m.scrimClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		scrim := color.NRGBA{A: uint8(float32(0xD8) * progress)}
		scrimRect := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
		paint.ColorOp{Color: scrim}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		scrimRect.Pop()
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		sz := gtx.Dp(unit.Dp(320))
		size := image.Pt(sz, sz)

		// Restrict the menu's click area to the inscribed circle so
		// taps in the transparent corners of the bounding square
		// fall through to the scrim below.
		defer clip.Ellipse{Max: size}.Push(gtx.Ops).Pop()

		return m.menuClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return m.renderWheel(gtx, sz, progress)
		})
	})
}

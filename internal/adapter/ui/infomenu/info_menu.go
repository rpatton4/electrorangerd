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
	"gioui.org/io/event"
	"gioui.org/io/pointer"
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

// sectorRingInner is the inner edge of the annulus the four ring
// sectors occupy, expressed as a fraction of the wheel radius. The
// outer edge is the inscribed-circle radius (1.0). Centred labels on
// the menu image fall inside this band, so 0.7 is a generous "outer
// ring" hit area.
const sectorRingInner = 0.7

// Sector identifies which of the four ring sectors of the mode wheel
// the user tapped. The labels are positioned at the cardinal points:
// Diagram at top, Forward at left, Dictionary at right, Reverse at
// bottom. Each sector spans 90° centred on its label.
type Sector int

const (
	SectorDiagram Sector = iota
	SectorForward
	SectorDictionary
	SectorReverse
)

// String returns the spoken label for the sector.
func (s Sector) String() string {
	switch s {
	case SectorDiagram:
		return "Diagram"
	case SectorForward:
		return "Forward"
	case SectorDictionary:
		return "Dictionary"
	case SectorReverse:
		return "Reverse"
	default:
		return "Unknown"
	}
}

// InfoMenu owns the state, click handlers, and embedded chrome
// images needed by the navigation element. A single InfoMenu instance
// is meant to live for the lifetime of an App and be reused across
// every screen that surfaces the nav button. The onSelect callback
// fires whenever the user taps one of the four ring sectors; the
// overlay context auto-dismisses after a sector tap.
type InfoMenu struct {
	log      *slog.Logger
	onSelect func(Sector)

	open     bool
	openedAt time.Time

	navClick   widget.Clickable
	menuClick  widget.Clickable
	scrimClick widget.Clickable

	// Hover state for the wheel: hoveredSector is meaningful only when
	// hovering is true. The renderer swaps menuImage for the matching
	// entry in highlightImages while the hover is active.
	wheelHoverTag struct{}
	hoveredSector Sector
	hovering      bool

	navImage        paint.ImageOp
	menuImage       paint.ImageOp
	highlightImages map[Sector]paint.ImageOp
}

// New builds an InfoMenu with the embedded chrome assets decoded
// ready for paint. onSelect is invoked when the user taps one of the
// four ring sectors on the menu wheel; pass nil to ignore sector taps.
func New(log *slog.Logger, onSelect func(Sector)) *InfoMenu {
	return &InfoMenu{
		log:             log,
		onSelect:        onSelect,
		navImage:        decodeNavButton(log),
		menuImage:       decodeMenuButton(log),
		highlightImages: decodeHighlightImages(log),
	}
}

// resolveLastPressSector inspects the latest press on the menu
// wheel and returns the sector under it. Returns false when the
// press landed inside the centre (not on the outer ring) or no
// press has been recorded.
func (m *InfoMenu) resolveLastPressSector(sz int) (Sector, bool) {
	presses := m.menuClick.History()
	if len(presses) == 0 {
		return 0, false
	}
	last := presses[len(presses)-1]
	return resolveSector(last.Position.X, last.Position.Y, sz)
}

// resolveSector returns the ring sector under a press at (px, py) in
// the local coordinates of a wheel sized sz × sz. The wheel centre is
// at (sz/2, sz/2); the outer ring annulus extends from
// sz/2 * sectorRingInner to sz/2 in radius. Each sector spans 90°
// centred on its label's cardinal direction (Diagram=top,
// Forward=left, Dictionary=right, Reverse=bottom).
func resolveSector(px, py, sz int) (Sector, bool) {
	cx := float64(sz) / 2
	cy := float64(sz) / 2
	dx := float64(px) - cx
	dy := float64(py) - cy
	radius := math.Sqrt(dx*dx + dy*dy)
	maxRadius := float64(sz) / 2

	if radius < maxRadius*sectorRingInner || radius > maxRadius {
		return 0, false
	}

	angle := math.Atan2(dy, dx)
	switch {
	case angle >= -math.Pi/4 && angle < math.Pi/4:
		return SectorDictionary, true
	case angle >= math.Pi/4 && angle < 3*math.Pi/4:
		return SectorReverse, true
	case angle >= 3*math.Pi/4 || angle < -3*math.Pi/4:
		return SectorForward, true
	default:
		return SectorDiagram, true
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
	baselineY := gtx.Dp(unit.Dp(62))
	// The nav button has 28dp radius and a centre at (cx, 67dp). The
	// bump dimensions keep ~10dp clearance from the button: the apex
	// sits 38dp above the centre (29dp Y) and the half-width is wide
	// enough that the cubic's inward bulge stays outside a 38dp circle
	// around the button centre.
	bumpApexY := gtx.Dp(unit.Dp(29))
	bumpHalfW := gtx.Dp(unit.Dp(65))
	strokeW := float32(gtx.Dp(unit.Dp(2)))
	btnY := gtx.Dp(unit.Dp(39))

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
// the wheel as a focal element without the overlay's scrim. Taps on
// the four label sectors fire onSelect.
func (m *InfoMenu) LayoutWheel(gtx layout.Context, sizeDp unit.Dp, animStart time.Time) layout.Dimensions {
	sz := gtx.Dp(sizeDp)
	if m.menuClick.Clicked(gtx) {
		if sector, ok := m.resolveLastPressSector(sz); ok && m.onSelect != nil {
			// onSelect mutates host state (e.g. switching screens);
			// gio doesn't auto-fire a follow-up frame after a state
			// mutation, so without this invalidate the user has to
			// click a second time before the new screen renders.
			m.onSelect(sector)
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	progress := m.animationProgress(gtx, animStart)

	size := image.Pt(sz, sz)
	defer clip.Ellipse{Max: size}.Push(gtx.Ops).Pop()
	return m.menuClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return m.renderWheel(gtx, sz, progress)
	})
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
//
// Hover handling lives here too: the wheel registers a pointer
// hover area in the static (un-animated) wheel coords, resolves the
// pointer to a sector via the same maths the click handler uses,
// and swaps the painted source to the matching per-sector highlight
// image once the open animation has settled.
func (m *InfoMenu) renderWheel(gtx layout.Context, sz int, progress float32) layout.Dimensions {
	size := image.Pt(sz, sz)
	gtx.Constraints = layout.Exact(size)

	event.Op(gtx.Ops, &m.wheelHoverTag)
	m.handleWheelPointer(gtx, sz)

	src := m.menuImage
	if m.hovering && progress >= 1 {
		if hi, ok := m.highlightImages[m.hoveredSector]; ok && hi != (paint.ImageOp{}) {
			src = hi
		}
	}

	scale := 0.125 + 0.875*progress
	rotation := progress * 2 * float32(math.Pi)
	centre := f32.Pt(float32(sz)/2, float32(sz)/2)
	affine := f32.Affine2D{}.
		Rotate(centre, rotation).
		Scale(centre, f32.Pt(scale, scale))
	aff := op.Affine(affine).Push(gtx.Ops)
	dims := widget.Image{
		Src:      src,
		Fit:      widget.Cover,
		Position: layout.Center,
	}.Layout(gtx)
	aff.Pop()

	return dims
}

// handleWheelPointer drains pointer Enter/Leave/Move events on the
// wheel's static hit area and updates hovering / hoveredSector
// accordingly. Press / Release events are NOT consumed here — the
// widget.Clickable that wraps the wheel keeps owning those so sector
// taps still fire onSelect.
func (m *InfoMenu) handleWheelPointer(gtx layout.Context, sz int) {
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &m.wheelHoverTag,
			Kinds:  pointer.Enter | pointer.Leave | pointer.Move,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Enter, pointer.Move:
			if sector, ok := resolveSector(int(pe.Position.X), int(pe.Position.Y), sz); ok {
				m.hoveredSector = sector
				m.hovering = true
			} else {
				m.hovering = false
			}
		case pointer.Leave:
			m.hovering = false
		}
	}
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

	const overlayWheelDp = 320
	sz := gtx.Dp(unit.Dp(overlayWheelDp))

	if m.menuClick.Clicked(gtx) {
		// Sector tap: pick the mode and dismiss the overlay so the
		// user sees the new mode shell behind. Centre taps land
		// inside the wheel but outside the ring and are ignored.
		if sector, ok := m.resolveLastPressSector(sz); ok {
			if m.onSelect != nil {
				m.onSelect(sector)
			}
			m.open = false
			gtx.Execute(op.InvalidateCmd{})
			return layout.Dimensions{}
		}
	}
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

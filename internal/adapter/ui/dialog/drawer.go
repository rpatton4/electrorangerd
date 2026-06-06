package dialog

import (
	"image"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/rpatton4/electrorangerd/internal/adapter/ui/theme"
)

// drawerDuration is how long the open/close animation takes end to end.
const drawerDuration = 220 * time.Millisecond

// Drawer is a button whose body slides down on click to reveal extra content,
// then snaps back on second click. The animation is a clipped translate-Y —
// pure 2D, matching the 2.5D-in-Gio rendering approach.
type Drawer struct {
	Title   string
	Body    string
	Click   widget.Clickable
	open    bool
	since   time.Time
	revOpen bool
}

// Layout draws the drawer.
func (d *Drawer) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	if d.Click.Clicked(gtx) {
		d.toggle()
	}

	progress := d.progress()
	if progress > 0 && progress < 1 {
		gtx.Execute(op.InvalidateCmd{})
	}

	return material.Clickable(gtx, &d.Click, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(12)).Layout(gtx, material.Body1(th.Material, d.Title).Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return d.drawerBody(gtx, th, progress)
			}),
		)
	})
}

// drawerBody clips the body to a translate-Y animation. At progress 0 the
// body height is zero; at progress 1 the body is fully revealed.
func (d *Drawer) drawerBody(gtx layout.Context, th *theme.Theme, progress float32) layout.Dimensions {
	const fullBodyHeightDp = 96
	fullBody := gtx.Dp(unit.Dp(fullBodyHeightDp))
	visible := int(float32(fullBody) * progress)
	if visible <= 0 {
		return layout.Dimensions{}
	}

	clipArea := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, visible)}.Push(gtx.Ops)
	defer clipArea.Pop()

	// Translate the body up by (fullBody - visible) so it "slides down" into
	// view as visible grows.
	offset := op.Affine(f32.Affine2D{}.Offset(f32.Pt(0, float32(visible-fullBody)))).Push(gtx.Ops)
	defer offset.Pop()

	bgRect := clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, fullBody)}.Push(gtx.Ops)
	paint.ColorOp{Color: th.SurfaceContainer}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bgRect.Pop()

	inset := layout.UniformInset(unit.Dp(12))
	inset.Layout(gtx, material.Body2(th.Material, d.Body).Layout)

	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, visible)}
}

// toggle flips the open state and records the transition start so progress
// can ease from 0 → 1 (or 1 → 0).
func (d *Drawer) toggle() {
	d.revOpen = d.open
	d.open = !d.open
	d.since = time.Now()
}

// progress returns the eased open/close progress from 0 (fully closed) to 1
// (fully open). Linear ease — simple, predictable, no spring overshoot.
func (d *Drawer) progress() float32 {
	if d.since.IsZero() {
		if d.open {
			return 1
		}
		return 0
	}
	elapsed := time.Since(d.since)
	if elapsed >= drawerDuration {
		if d.open {
			return 1
		}
		return 0
	}
	frac := float32(elapsed) / float32(drawerDuration)
	if d.open {
		return frac
	}
	return 1 - frac
}

package dialog

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/rpatton4/electrorangerd/internal/adapter/ui/theme"
)

// Notice is a centred message dialog that dismisses on any click —
// scrim or panel both count. It carries no buttons and no title; the
// only state is "open with this message" / "closed". Use it to stub
// out a feature that does not exist yet, or to surface a one-shot
// informational message the user only needs to acknowledge.
//
// An optional Image paints below the message, scaled by ImageScale of
// its intrinsic pixel size (treated as dp so the visual size stays
// consistent with the rest of the UI). Leave Image at its zero value
// for a text-only dialog.
type Notice struct {
	Message    string
	Image      paint.ImageOp
	ImageScale float32
	open       bool
	scrim      widget.Clickable
	panel      widget.Clickable
}

// NewNotice returns an empty, closed Notice.
func NewNotice() *Notice { return &Notice{} }

// Open shows the notice with the given message.
func (n *Notice) Open(msg string) {
	n.Message = msg
	n.open = true
}

// Close hides the notice without firing dismissed.
func (n *Notice) Close() { n.open = false }

// IsOpen reports whether the notice is currently visible.
func (n *Notice) IsOpen() bool { return n.open }

// Layout draws the notice when open and returns dismissed=true on the
// frame the user clicks anywhere (scrim or panel). Click drains run
// BEFORE any nested Layout call because widget.Clickable.Layout
// consumes click gestures internally — same dance as modal.go and
// context_menu.go.
func (n *Notice) Layout(gtx layout.Context, th *theme.Theme) (dismissed bool) {
	if !n.open {
		return false
	}

	scrimHit, panelHit := false, false
	for n.scrim.Clicked(gtx) {
		scrimHit = true
	}
	for n.panel.Clicked(gtx) {
		panelHit = true
	}
	if scrimHit || panelHit {
		n.open = false
		return true
	}

	scrimSize := gtx.Constraints.Max
	scrimClip := clip.Rect{Max: scrimSize}.Push(gtx.Ops)
	n.scrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		paint.ColorOp{Color: color.NRGBA{A: 0x80}}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		return layout.Dimensions{Size: scrimSize}
	})
	scrimClip.Pop()

	panelW := gtx.Dp(unit.Dp(480))
	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = panelW
		gtx.Constraints.Max.X = panelW
		return n.panel.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			content := layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						body := material.Body1(th.Material, n.Message)
						body.Alignment = text.Middle
						return body.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return n.layoutImage(gtx)
					}),
				)
			})
			call := macro.Stop()

			bgClip := clip.UniformRRect(image.Rect(0, 0, content.Size.X, content.Size.Y), gtx.Dp(unit.Dp(8))).Push(gtx.Ops)
			paint.ColorOp{Color: th.SurfaceContainer}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			bgClip.Pop()
			stroke := gtx.Dp(unit.Dp(1))
			fillBorder(gtx, image.Rect(0, 0, content.Size.X, content.Size.Y), stroke, th.Outline)

			call.Add(gtx.Ops)
			return content
		})
	})

	return false
}

// layoutImage renders Notice.Image at ImageScale of its intrinsic
// pixel size, treating the intrinsic dimensions as dp so the visual
// size scales consistently with the rest of the UI. Returns a 16dp
// top spacer above the image so the figure does not crowd the
// message text. Renders nothing when Image is zero or ImageScale is
// non-positive.
func (n *Notice) layoutImage(gtx layout.Context) layout.Dimensions {
	if n.ImageScale <= 0 || n.Image == (paint.ImageOp{}) {
		return layout.Dimensions{}
	}
	src := n.Image.Size()
	if src.X <= 0 || src.Y <= 0 {
		return layout.Dimensions{}
	}
	target := image.Pt(
		gtx.Dp(unit.Dp(float32(src.X)*n.ImageScale)),
		gtx.Dp(unit.Dp(float32(src.Y)*n.ImageScale)),
	)
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(target)
			return widget.Image{
				Src:      n.Image,
				Fit:      widget.Contain,
				Position: layout.Center,
			}.Layout(gtx)
		}),
	)
}

package dialog

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/rpatton4/electrorangerd/internal/adapter/ui/theme"
)

// Modal is a centred dialog with a title row, a caller-supplied body, and
// a Save / Cancel button row. The host renders it on top of everything
// else by calling Layout last; clicks on the scrim count as Cancel.
//
// The two outcomes are surfaced through Layout's return values so the
// caller drives the actual action — the widget itself just opens, paints,
// and tracks the buttons.
type Modal struct {
	Title  string
	open   bool
	save   widget.Clickable
	cancel widget.Clickable
	scrim  widget.Clickable
	panel  widget.Clickable
}

// NewModal returns a Modal with the given title.
func NewModal(title string) *Modal {
	return &Modal{Title: title}
}

// Open makes the modal visible.
func (m *Modal) Open() { m.open = true }

// Close hides the modal without firing Save or Cancel.
func (m *Modal) Close() { m.open = false }

// IsOpen reports whether the modal is currently visible.
func (m *Modal) IsOpen() bool { return m.open }

// Layout draws the modal (when open) and returns (saved, cancelled).
// body lays out the content between the title and the button row.
// Either flag closes the modal as a side effect.
//
// Click drains run BEFORE any Layout call because widget.Clickable.Layout
// consumes click gestures internally; see dialog/context_menu.go for the
// same dance and the linked issue in widget/button.go.
func (m *Modal) Layout(gtx layout.Context, th *theme.Theme, body func(gtx layout.Context) layout.Dimensions) (saved, cancelled bool) {
	if !m.open {
		return false, false
	}

	saveHit, cancelHit, scrimHit, panelHit := false, false, false, false
	for m.save.Clicked(gtx) {
		saveHit = true
	}
	for m.cancel.Clicked(gtx) {
		cancelHit = true
	}
	for m.scrim.Clicked(gtx) {
		scrimHit = true
	}
	for m.panel.Clicked(gtx) {
		panelHit = true
	}
	switch {
	case saveHit:
		saved = true
		m.open = false
	case cancelHit, scrimHit && !panelHit:
		cancelled = true
		m.open = false
	}
	if !m.open {
		return saved, cancelled
	}

	// Scrim: full-area translucent darken; clicks on it dismiss as Cancel
	// unless a panel click absorbed the same frame's press.
	scrimSize := gtx.Constraints.Max
	scrimClip := clip.Rect{Max: scrimSize}.Push(gtx.Ops)
	m.scrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		paint.ColorOp{Color: color.NRGBA{A: 0x80}}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		return layout.Dimensions{Size: scrimSize}
	})
	scrimClip.Pop()

	// Panel: fixed width, vertical-centred. Record content first, paint
	// background + outline behind it using the recorded dims, then
	// replay the content on top.
	panelW := gtx.Dp(unit.Dp(480))
	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = panelW
		gtx.Constraints.Max.X = panelW
		return m.panel.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			content := layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.H6(th.Material, m.Title).Layout),
					layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
					layout.Rigid(body),
					layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceEnd}.Layout(gtx,
							layout.Rigid(material.Button(th.Material, &m.save, "Save").Layout),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(material.Button(th.Material, &m.cancel, "Cancel").Layout),
						)
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

	return saved, cancelled
}

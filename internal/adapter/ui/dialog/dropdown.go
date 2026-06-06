package dialog

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/rpatton4/electrorangerd/internal/adapter/ui/theme"
)

// Dropdown is a simple selection control: a bar showing the current option
// (or a placeholder when nothing is selected), and a list of options that
// appears below when the bar is clicked. Selecting an item closes the list
// and updates the current selection.
type Dropdown struct {
	Options     []string
	Placeholder string
	selected    int
	open        bool
	bar         widget.Clickable
	items       []widget.Clickable
}

// NewDropdown returns a Dropdown over the given option labels. Initial
// selection is -1 (none); the bar shows Placeholder until something is
// chosen.
func NewDropdown(placeholder string, options ...string) *Dropdown {
	return &Dropdown{
		Options:     options,
		Placeholder: placeholder,
		selected:    -1,
		items:       make([]widget.Clickable, len(options)),
	}
}

// Selected returns the index of the currently chosen option, or -1.
func (d *Dropdown) Selected() int { return d.selected }

// SetSelected updates the current selection; pass -1 to clear it.
func (d *Dropdown) SetSelected(i int) {
	if i < 0 || i >= len(d.Options) {
		d.selected = -1
		return
	}
	d.selected = i
}

// Close hides the items list.
func (d *Dropdown) Close() { d.open = false }

// IsOpen reports whether the items list is currently shown.
func (d *Dropdown) IsOpen() bool { return d.open }

// Layout draws the bar and (when open) the items list below it. Click
// drains run BEFORE any Layout call because widget.Clickable.Layout
// consumes click gestures internally.
func (d *Dropdown) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	for d.bar.Clicked(gtx) {
		d.open = !d.open
	}
	for i := range d.items {
		for d.items[i].Clicked(gtx) {
			d.selected = i
			d.open = false
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.layoutBar(gtx, th)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !d.open {
				return layout.Dimensions{}
			}
			return d.layoutItems(gtx, th)
		}),
	)
}

func (d *Dropdown) layoutBar(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	label := d.Placeholder
	if d.selected >= 0 && d.selected < len(d.Options) {
		label = d.Options[d.selected]
	}
	return d.bar.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		macro := op.Record(gtx.Ops)
		dims := layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, material.Body1(th.Material, label).Layout),
				layout.Rigid(material.Body1(th.Material, "▾").Layout),
			)
		})
		call := macro.Stop()

		bgClip := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
		paint.ColorOp{Color: th.SurfaceVariant}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		bgClip.Pop()
		stroke := gtx.Dp(unit.Dp(1))
		fillBorder(gtx, image.Rect(0, 0, dims.Size.X, dims.Size.Y), stroke, th.Outline)

		call.Add(gtx.Ops)
		return dims
	})
}

func (d *Dropdown) layoutItems(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	children := make([]layout.FlexChild, len(d.Options))
	for i := range d.Options {
		i := i
		children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.items[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, material.Body1(th.Material, d.Options[i]).Layout)
			})
		})
	}
	macro := op.Record(gtx.Ops)
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	call := macro.Stop()

	bgClip := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	paint.ColorOp{Color: th.SurfaceContainer}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bgClip.Pop()
	stroke := gtx.Dp(unit.Dp(1))
	fillBorder(gtx, image.Rect(0, 0, dims.Size.X, dims.Size.Y), stroke, th.Outline)

	call.Add(gtx.Ops)
	return dims
}

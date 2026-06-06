package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/theme"
)

// welcomeView is the initial post-unlock chooser. It renders one button per
// mode, vertically stacked and centered, sized uniformly to the widest label.
// Clicking a button reports the selection up via the onSelect callback
// supplied at construction.
type welcomeView struct {
	onSelect func(Mode)
	clicks   [len(allModes)]widget.Clickable
}

func newWelcomeView(onSelect func(Mode)) *welcomeView {
	return &welcomeView{onSelect: onSelect}
}

func (v *welcomeView) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	for i, m := range allModes {
		if v.clicks[i].Clicked(gtx) {
			v.onSelect(m)
		}
	}

	// Pass 1 — measure each button at its natural size. The recorded ops
	// (including pointer-input registrations) are discarded with the macro,
	// so the real registration only happens in pass 2.
	var maxSize image.Point
	for i, m := range allModes {
		loose := gtx
		loose.Constraints.Min = image.Point{}
		macro := op.Record(gtx.Ops)
		dims := material.Button(th.Material, &v.clicks[i], m.Label()).Layout(loose)
		macro.Stop()
		if dims.Size.X > maxSize.X {
			maxSize.X = dims.Size.X
		}
		if dims.Size.Y > maxSize.Y {
			maxSize.Y = dims.Size.Y
		}
	}

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(allModes)*2-1)
		for i, m := range allModes {
			i, m := i, m
			if i > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout))
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = maxSize
				gtx.Constraints.Max.X = maxSize.X
				return material.Button(th.Material, &v.clicks[i], m.Label()).Layout(gtx)
			}))
		}
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

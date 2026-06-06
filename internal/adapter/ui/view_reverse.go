package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// reverseView is the per-mode view for ModeReverseEngineering. Like the
// forward view it owns no History stack — reverse-engineering produces a
// Project as a one-shot output the user accepts or discards.
type reverseView struct {
	reverse port.ReverseEngineer
}

func newReverseView(rev port.ReverseEngineer) *reverseView {
	return &reverseView{reverse: rev}
}

func (v *reverseView) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.H4(th, "Reverse Engineering").Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(material.Body2(th, "Build an ER diagram from a live PostgreSQL data source. Inferred relationships will appear with Provenance metadata for review.").Layout),
		)
	})
}

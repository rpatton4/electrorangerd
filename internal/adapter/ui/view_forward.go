package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/dialog"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// forwardView is the per-mode view for ModeForward (Forward Mode). It owns
// no History stack — forward engineering is a one-shot operation that
// reviews a MigrationPlan rather than an edit session.
type forwardView struct {
	forward port.ForwardEngineer
	drawer  *dialog.Drawer
}

func newForwardView(fwd port.ForwardEngineer) *forwardView {
	return &forwardView{
		forward: fwd,
		drawer: &dialog.Drawer{
			Title: "Migration plan (click to expand)",
			Body:  "The forward engineer will surface a MigrationPlan here — V_ versioned files plus optional R_ repeatable files. Flyway Community has no undo, so reversals are forward 'compensation' migrations.",
		},
	}
}

func (v *forwardView) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.H4(th, "Forward Engineering").Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(material.Body2(th, "Review the migration plan before pushing to a target database.").Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(24)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.drawer.Layout(gtx, th)
			}),
		)
	})
}

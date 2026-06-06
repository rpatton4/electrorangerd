package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/canvas"
	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/theme"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// diagramView is the per-mode view for ModeDiagram (Diagram Mode). It holds
// the 2.5D canvas and the diagram-local History stack. Real per-entity walks
// and editing arrive in a future iteration.
type diagramView struct {
	history port.History
	canvas  *canvas.Canvas
}

func newDiagramView(history port.History) *diagramView {
	return &diagramView{history: history, canvas: canvas.New()}
}

func (v *diagramView) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.H4(th.Material, "Diagram Edit").Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(material.Body2(th.Material, "2.5D canvas — entity boxes carry axonometric shear; titles stay upright.").Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(32)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.canvas.SampleEntity(gtx, th.Material, "Customer")
			}),
		)
	})
}

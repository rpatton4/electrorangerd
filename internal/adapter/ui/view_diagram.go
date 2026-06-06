package ui

import (
	"gioui.org/layout"

	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/theme"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

// diagramView is the per-mode view for ModeDiagram (Diagram Mode). It holds
// the diagram-local History stack. Real per-entity walks and editing arrive
// in a future iteration.
type diagramView struct {
	history port.History
}

func newDiagramView(history port.History) *diagramView {
	return &diagramView{history: history}
}

func (v *diagramView) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	return layout.Dimensions{Size: gtx.Constraints.Max}
}
